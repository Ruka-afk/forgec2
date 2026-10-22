package payload

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// benignFuncs maps well-known DLLs to a benign export that exists in every
// shipped version of the DLL. The Windows loader validates every imported
// function name at load time, so only real exports may be used.
var benignFuncs = map[string]string{
	"kernel32.dll": "GetModuleHandleA",
	"advapi32.dll": "RegOpenKeyExA",
	"user32.dll":   "MessageBoxA",
	"ws2_32.dll":   "WSAStartup",
	"bcrypt.dll":   "BCryptOpenAlgorithmProvider",
	"ntdll.dll":    "NtQueryInformationProcess",
	"shell32.dll":  "ShellExecuteA",
	"ole32.dll":    "CoCreateInstance",
	"wininet.dll":  "InternetOpenA",
	"gdi32.dll":    "GetDeviceCaps",
	"comctl32.dll": "InitCommonControlsEx",
	"version.dll":  "GetFileVersionInfoA",
	"crypt32.dll":  "CertOpenStore",
	"iphlpapi.dll": "GetAdaptersInfo",
	"netapi32.dll": "NetApiBufferFree",
	"psapi.dll":    "EnumProcesses",
	"secur32.dll":  "AcquireCredentialsHandleA",
	"setupapi.dll": "SetupDiGetClassDevsA",
	"winmm.dll":    "waveOutOpen",
	"wldap32.dll":  "ldap_init",
}

type peSectionView struct {
	rawPtr  uint32
	rawSize uint32
	va      uint32
	virtSz  uint32
	// hdrOff is the file offset of this section's 40-byte header, enabling
	// in-place SizeOfRawData/VirtualSize growth for import slack.
	hdrOff int
}

type peImportView struct {
	data          []byte
	sections      []peSectionView
	importRVA     uint32
	importSize    uint32
	importOff     int
	descCount     int // number of descriptors before the null terminator
	tableEnd      int // file offset just past the null descriptor
	sectionRawEnd int // file offset of the end of the section holding the table
	is64          bool
	thunkSize     int // 8 for PE32+, 4 for PE32
}

// peImportDescriptor mirrors IMAGE_IMPORT_DESCRIPTOR (20 bytes).
type peImportDescriptor struct {
	OriginalFirstThunk uint32
	TimeDateStamp      uint32
	ForwarderChain     uint32
	Name               uint32
	FirstThunk         uint32
}

func (d *peImportDescriptor) isNull() bool {
	return d.OriginalFirstThunk == 0 && d.TimeDateStamp == 0 &&
		d.ForwarderChain == 0 && d.Name == 0 && d.FirstThunk == 0
}

func parseImportView(data []byte) (*peImportView, error) {
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return nil, fmt.Errorf("not a PE image (missing MZ header)")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOff+24 >= len(data) {
		return nil, fmt.Errorf("not a PE image (truncated header)")
	}
	if data[peOff] != 'P' || data[peOff+1] != 'E' {
		return nil, fmt.Errorf("not a PE image (missing PE signature)")
	}
	numSections := int(binary.LittleEndian.Uint16(data[peOff+6 : peOff+8]))
	if numSections <= 0 || numSections > 96 {
		return nil, fmt.Errorf("invalid section count %d", numSections)
	}
	sizeOpt := int(binary.LittleEndian.Uint16(data[peOff+20 : peOff+22]))
	if sizeOpt == 0 {
		return nil, fmt.Errorf("missing optional header")
	}
	magic := binary.LittleEndian.Uint16(data[peOff+24 : peOff+26])
	var dataDirOff int
	switch magic {
	case 0x10B: // PE32
		dataDirOff = peOff + 24 + 0x60
	case 0x20B: // PE32+
		dataDirOff = peOff + 24 + 0x70
	default:
		return nil, fmt.Errorf("unsupported optional header magic 0x%04X", magic)
	}
	thunkSize := 4
	if magic == 0x20B {
		thunkSize = 8
	}
	secOff := peOff + 4 + 20 + sizeOpt
	if secOff+40*numSections > len(data) {
		return nil, fmt.Errorf("truncated section headers")
	}
	sections := make([]peSectionView, 0, numSections)
	for i := 0; i < numSections; i++ {
		h := secOff + i*40
		sections = append(sections, peSectionView{
			va:      binary.LittleEndian.Uint32(data[h+12:]),
			virtSz:  binary.LittleEndian.Uint32(data[h+8:]),
			rawPtr:  binary.LittleEndian.Uint32(data[h+20:]),
			rawSize: binary.LittleEndian.Uint32(data[h+16:]),
			hdrOff:  h,
		})
	}

	// Data directory entry 1 = IMAGE_DIRECTORY_ENTRY_IMPORT.
	if dataDirOff+16 > len(data) {
		return nil, fmt.Errorf("truncated data directories")
	}
	importRVA := binary.LittleEndian.Uint32(data[dataDirOff+8:])
	importSize := binary.LittleEndian.Uint32(data[dataDirOff+12:])
	if importRVA == 0 {
		return nil, fmt.Errorf("PE has no import directory; imports cannot be added")
	}
	importOff, ok := rvaToOffset(sections, importRVA)
	if !ok {
		return nil, fmt.Errorf("import directory RVA 0x%08X not mapped by any section", importRVA)
	}

	view := &peImportView{
		data:       data,
		sections:   sections,
		importRVA:  importRVA,
		importSize: importSize,
		importOff:  importOff,
		is64:       magic == 0x20B,
		thunkSize:  thunkSize,
	}

	// Bounds of the section that owns the import table. The descriptor array
	// must terminate inside that section: walking contiguous file bytes into
	// the next section would "find" a terminator in unrelated data and make a
	// corrupt table look valid.
	sec, ok := sectionForOffset(sections, view.importOff)
	if !ok {
		return nil, fmt.Errorf("import directory not within any section")
	}
	view.sectionRawEnd = int(sec.rawPtr + sec.rawSize)
	if view.sectionRawEnd > len(data) {
		view.sectionRawEnd = len(data)
	}
	if view.sectionRawEnd <= importOff+20 {
		return nil, fmt.Errorf("import section is too small for a descriptor table")
	}

	maxEntries := int(importSize) / 20
	if maxEntries == 0 {
		maxEntries = 512
	}
	if maxEntries > 4096 {
		maxEntries = 4096
	}
	if sectionEntries := (view.sectionRawEnd - importOff) / 20; maxEntries > sectionEntries {
		maxEntries = sectionEntries
	}
	terminated := false
	for i := 0; i < maxEntries; i++ {
		base := importOff + i*20
		if base+20 > view.sectionRawEnd {
			break
		}
		var d peImportDescriptor
		d.OriginalFirstThunk = binary.LittleEndian.Uint32(data[base:])
		d.TimeDateStamp = binary.LittleEndian.Uint32(data[base+4:])
		d.ForwarderChain = binary.LittleEndian.Uint32(data[base+8:])
		d.Name = binary.LittleEndian.Uint32(data[base+12:])
		d.FirstThunk = binary.LittleEndian.Uint32(data[base+16:])
		if d.isNull() {
			view.descCount = i
			view.tableEnd = base + 20
			terminated = true
			break
		}
	}
	if !terminated {
		return nil, fmt.Errorf("import descriptor table is unterminated within its section")
	}
	return view, nil
}

// peHasCertificateTable reports whether the optional header's security
// directory names a certificate table. The directory's VirtualAddress field is
// a file offset for this entry, not an RVA.
func peHasCertificateTable(data []byte) (bool, error) {
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return false, fmt.Errorf("not a PE image (missing MZ header)")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOff+24 > len(data) {
		return false, fmt.Errorf("truncated PE header")
	}
	if string(data[peOff:peOff+4]) != "PE\x00\x00" {
		return false, fmt.Errorf("missing PE signature")
	}
	optOff := peOff + 24
	sizeOpt := int(binary.LittleEndian.Uint16(data[peOff+20 : peOff+22]))
	if sizeOpt < 0x60 || optOff+sizeOpt > len(data) {
		return false, fmt.Errorf("truncated PE optional header")
	}
	var dataDirOff, numDirsOff int
	switch binary.LittleEndian.Uint16(data[optOff : optOff+2]) {
	case 0x10B:
		dataDirOff, numDirsOff = optOff+0x60, optOff+0x5C
	case 0x20B:
		dataDirOff, numDirsOff = optOff+0x70, optOff+0x6C
	default:
		return false, fmt.Errorf("unsupported PE optional-header magic")
	}
	if dataDirOff+5*8 > len(data) || dataDirOff+5*8 > optOff+sizeOpt {
		return false, nil
	}
	numDirs := uint32(0)
	if numDirsOff+4 <= optOff+sizeOpt {
		numDirs = binary.LittleEndian.Uint32(data[numDirsOff : numDirsOff+4])
	}
	if numDirs < 5 {
		return false, nil
	}
	secOff := binary.LittleEndian.Uint32(data[dataDirOff+4*8:])
	secSize := binary.LittleEndian.Uint32(data[dataDirOff+4*8+4:])
	return secOff != 0 || secSize != 0, nil
}

// mappedString reads a NUL-terminated string at rva, refusing to cross the
// end of the containing section's mapped/raw extent. max bounds the scan so a
// corrupt image cannot make validation walk the whole file.
func (v *peImportView) mappedString(rva uint32, max int) (string, error) {
	for _, s := range v.sections {
		if rva < s.va {
			continue
		}
		delta := rva - s.va
		mapped := s.virtSz
		if s.rawSize > mapped {
			mapped = s.rawSize
		}
		if delta >= mapped || delta >= s.rawSize {
			continue
		}
		limit := uint64(delta) + uint64(mapped)
		if limit > uint64(s.rawSize) {
			limit = uint64(s.rawSize)
		}
		start := uint64(s.rawPtr) + uint64(delta)
		end := uint64(s.rawPtr) + limit
		if end > uint64(len(v.data)) {
			end = uint64(len(v.data))
		}
		if start >= end {
			return "", fmt.Errorf("RVA 0x%x is outside the file", rva)
		}
		for i := start; i < end; i++ {
			if v.data[i] == 0 {
				return string(v.data[start:i]), nil
			}
			if i-start >= uint64(max) {
				break
			}
		}
		return "", fmt.Errorf("string at RVA 0x%x is not NUL-terminated within its section", rva)
	}
	return "", fmt.Errorf("RVA 0x%x is not mapped by any section", rva)
}

// validateThunkArray walks an ILT/IAT until its null terminator. Every named
// entry must resolve to a hint/name pair inside mapped section data; ordinal
// imports have no name to check.
func (v *peImportView) validateThunkArray(rva uint32, desc int) error {
	thunkSize := v.thunkSize
	if thunkSize == 0 {
		thunkSize = 4
		if v.is64 {
			thunkSize = 8
		}
	}
	for j := 0; j < 65536; j++ {
		at64 := uint64(rva) + uint64(j*thunkSize)
		if at64 > uint64(^uint32(0)) {
			return fmt.Errorf("descriptor %d thunk RVA overflows", desc)
		}
		off, ok := v.mappedOffset(uint32(at64), uint32(thunkSize))
		if !ok {
			return fmt.Errorf("descriptor %d thunk entry %d at RVA 0x%x is not mapped", desc, j, at64)
		}
		var entry uint64
		if thunkSize == 8 {
			entry = binary.LittleEndian.Uint64(v.data[off:])
		} else {
			entry = uint64(binary.LittleEndian.Uint32(v.data[off:]))
		}
		if entry == 0 {
			return nil
		}
		// Ordinal imports are encoded with the high bit set and have no
		// hint/name payload to validate.
		if thunkSize == 8 && entry&(uint64(1)<<63) != 0 {
			continue
		}
		if thunkSize == 4 && entry&(uint64(1)<<31) != 0 {
			continue
		}
		if entry > uint64(^uint32(0)) {
			return fmt.Errorf("descriptor %d thunk entry %d has an out-of-range RVA 0x%x", desc, j, entry)
		}
		hintRVA := uint32(entry)
		if _, ok := v.mappedOffset(hintRVA, 2); !ok {
			return fmt.Errorf("descriptor %d hint/name RVA 0x%x is not mapped", desc, hintRVA)
		}
		if _, err := v.mappedString(hintRVA+2, 512); err != nil {
			return fmt.Errorf("descriptor %d import name: %w", desc, err)
		}
	}
	return fmt.Errorf("descriptor %d thunk array has no null terminator", desc)
}

// validatePEImports validates the import table the way the Windows loader
// consumes it. A PE with no import directory is accepted; a present but
// truncated or dangling table is not. This is what turns a broken post-link
// injection into a build failure instead of an image that only fails on the
// target host.
func validatePEImports(data []byte) error {
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return fmt.Errorf("not a PE image (missing MZ header)")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOff+24 > len(data) {
		return fmt.Errorf("truncated PE header")
	}
	if string(data[peOff:peOff+4]) != "PE\x00\x00" {
		return fmt.Errorf("missing PE signature")
	}
	optOff := peOff + 24
	magic := binary.LittleEndian.Uint16(data[optOff : optOff+2])
	var dataDirOff int
	switch magic {
	case 0x10B:
		dataDirOff = optOff + 0x60
	case 0x20B:
		dataDirOff = optOff + 0x70
	default:
		return fmt.Errorf("unsupported PE optional-header magic 0x%04x", magic)
	}
	if dataDirOff+16 > len(data) {
		return fmt.Errorf("truncated PE data directories")
	}
	importRVA := binary.LittleEndian.Uint32(data[dataDirOff+8:])
	importSize := binary.LittleEndian.Uint32(data[dataDirOff+12:])
	if importRVA == 0 && importSize == 0 {
		return nil
	}
	if importRVA == 0 || importSize == 0 {
		return fmt.Errorf("import directory has only one of RVA/size set (RVA=0x%x size=%d)", importRVA, importSize)
	}
	view, err := parseImportView(data)
	if err != nil {
		return err
	}
	if view.descCount == 0 {
		return fmt.Errorf("import directory has no descriptors")
	}
	if uint64(importSize) < uint64(20*(view.descCount+1)) {
		return fmt.Errorf("import directory size %d is smaller than %d descriptors plus the null terminator",
			importSize, view.descCount)
	}
	for i := 0; i < view.descCount; i++ {
		base := view.importOff + i*20
		if base+20 > view.sectionRawEnd {
			return fmt.Errorf("descriptor %d crosses the import section boundary", i)
		}
		var d peImportDescriptor
		d.OriginalFirstThunk = binary.LittleEndian.Uint32(data[base:])
		d.TimeDateStamp = binary.LittleEndian.Uint32(data[base+4:])
		d.ForwarderChain = binary.LittleEndian.Uint32(data[base+8:])
		d.Name = binary.LittleEndian.Uint32(data[base+12:])
		d.FirstThunk = binary.LittleEndian.Uint32(data[base+16:])
		if d.Name == 0 || d.FirstThunk == 0 {
			return fmt.Errorf("descriptor %d is missing a DLL name or IAT", i)
		}
		dll, err := view.mappedString(d.Name, 512)
		if err != nil {
			return fmt.Errorf("descriptor %d DLL name: %w", i, err)
		}
		if dll == "" {
			return fmt.Errorf("descriptor %d has an empty DLL name", i)
		}
		for _, c := range dll {
			if c < 0x20 || c > 0x7e {
				return fmt.Errorf("descriptor %d DLL name %q contains a non-printable byte", i, dll)
			}
		}
		thunkRVA := d.OriginalFirstThunk
		if thunkRVA == 0 {
			thunkRVA = d.FirstThunk
		}
		if err := view.validateThunkArray(thunkRVA, i); err != nil {
			return err
		}
	}
	return nil
}

func rvaToOffset(sections []peSectionView, rva uint32) (int, bool) {
	for _, s := range sections {
		sz := s.virtSz
		if s.rawSize > sz {
			sz = s.rawSize
		}
		if rva >= s.va && rva < s.va+sz && rva < s.va+s.rawSize {
			return int(s.rawPtr + (rva - s.va)), true
		}
	}
	return 0, false
}

func sectionForOffset(sections []peSectionView, off int) (peSectionView, bool) {
	for _, s := range sections {
		if uint32(off) >= s.rawPtr && uint32(off) < s.rawPtr+s.rawSize {
			return s, true
		}
	}
	return peSectionView{}, false
}

func (v *peImportView) offsetToRVA(off int) (uint32, bool) {
	for _, s := range v.sections {
		if uint32(off) >= s.rawPtr && uint32(off) < s.rawPtr+s.rawSize {
			return s.va + (uint32(off) - s.rawPtr), true
		}
	}
	return 0, false
}

// mappedOffset resolves an RVA to a file offset and requires size bytes to be
// inside the section's file-backed extent (the larger of VirtualSize and
// SizeOfRawData). It is the loader's view of an import table; rvaToOffset is
// deliberately laxer because the injection paths may grow VirtualSize after
// writing.
func (v *peImportView) mappedOffset(rva uint32, size uint32) (int, bool) {
	for _, s := range v.sections {
		if rva < s.va {
			continue
		}
		delta := rva - s.va
		// The loader maps the section's file-backed raw data as well as its
		// virtual size. Go's .idata intentionally places the ILT's terminating
		// null thunk at exactly VirtualSize (still inside SizeOfRawData), so
		// accepting only delta < VirtualSize would reject valid toolchain
		// output.
		mapped := s.virtSz
		if s.rawSize > mapped {
			mapped = s.rawSize
		}
		if delta >= mapped || uint64(delta)+uint64(size) > uint64(mapped) {
			continue
		}
		off := uint64(s.rawPtr) + uint64(delta)
		if off+uint64(size) > uint64(len(v.data)) {
			continue
		}
		return int(off), true
	}
	return 0, false
}

// importedDLLs returns the DLL names already present in the import table.
func (v *peImportView) importedDLLs() []string {
	var names []string
	for i := 0; i < v.descCount; i++ {
		base := v.importOff + i*20
		nameRVA := binary.LittleEndian.Uint32(v.data[base+12:])
		off, ok := rvaToOffset(v.sections, nameRVA)
		if !ok || off < 0 || off >= len(v.data) {
			continue
		}
		end := off
		for end < len(v.data) && end-off < 260 && v.data[end] != 0 {
			end++
		}
		if n := string(v.data[off:end]); n != "" {
			names = append(names, strings.ToLower(n))
		}
	}
	return names
}

// AddBenignImports ensures the requested DLLs appear in the import table of a
// PE image. Already-imported DLLs are left untouched. Missing DLLs are added
// using one of two strategies:
//
//  1. In-place: when the descriptor table is followed by contiguous zero
//     slack (typical of hand-packed binaries), new descriptors are appended
//     right after the existing table.
//  2. Relocation: otherwise the entire descriptor array is rewritten into the
//     trailing zero region of the section that holds the import table (Go
//     toolchain binaries always leave zero padding at the end of .idata),
//     and the import data directory is repointed at the new array. Only the
//     descriptor array is moved; the ILT/IAT/name data of existing imports
//     stays where it is (the copied descriptors reference it by its original
//     RVA), so no base relocations need to be rewritten.
//
// If neither strategy has enough zero space, a fresh PE section is appended
// and the import directory is repointed at it. Appending never shifts existing
// bytes, so the COFF symbol-table pointer, section raw offsets and trailing
// metadata stay valid — the old in-place growth path shifted all of them
// without updating PointerToSymbolTable.
// missingDLL pairs a DLL to add with a benign export that provably exists in
// every shipped version of that DLL.
type missingDLL struct {
	name   string
	export string
}

// AddBenignImports injects benign DLL imports into a PE. It first tries to use
// the existing import section's slack, then appends a dedicated section when
// the table cannot be extended safely. Returns the possibly reallocated image
// bytes.
func AddBenignImports(data []byte, dlls []string) ([]byte, error) {
	if len(dlls) == 0 {
		return data, nil
	}
	view, err := parseImportView(data)
	if err != nil {
		return data, fmt.Errorf("import manipulation: %w", err)
	}
	// Adding an import changes bytes covered by an Authenticode digest. Refuse
	// up front instead of producing a loaded-but-invalid signature (or letting
	// a later signing step fail with a less obvious "already signed" error).
	if signed, err := peHasCertificateTable(data); err != nil {
		return data, fmt.Errorf("import manipulation: %w", err)
	} else if signed {
		return data, fmt.Errorf("import manipulation: cannot modify a signed PE (certificate table present)")
	}

	existing := view.importedDLLs()
	has := func(name string) bool {
		for _, e := range existing {
			if e == name {
				return true
			}
		}
		return false
	}

	var missing []missingDLL
	for _, raw := range dlls {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if !strings.HasSuffix(name, ".dll") {
			name += ".dll"
		}
		if has(name) {
			continue
		}
		export, ok := benignFuncs[name]
		if !ok {
			return data, artifactValidationError("import manipulation: no known benign export for %s", name)
		}
		missing = append(missing, missingDLL{name: name, export: export})
	}
	if len(missing) == 0 {
		return data, nil
	}

	tryInject := func(d []byte) ([]byte, bool, error, error) {
		// Each strategy mutates its input before it can discover that the
		// result would not fit. Work on a copy per strategy so a failed
		// attempt cannot corrupt the image the next strategy (or the
		// new-section fallback) builds on.
		inPlace := append([]byte(nil), d...)
		v, perr := parseImportView(inPlace)
		if perr != nil {
			return d, false, perr, nil
		}
		iErr := injectImportsInPlace(v, missing)
		if iErr == nil {
			if vErr := validatePEImports(inPlace); vErr == nil {
				return inPlace, true, nil, nil
			} else {
				// An in-place write can overwrite a zero byte that is a live
				// null terminator (for example the old ILT's). Treat that as
				// a failed strategy and let relocation/new-section try.
				iErr = vErr
			}
		}

		relocated := append([]byte(nil), d...)
		rv, perr := parseImportView(relocated)
		if perr != nil {
			return d, false, iErr, perr
		}
		rErr := injectImportsRelocated(rv, missing)
		if rErr == nil {
			if vErr := validatePEImports(relocated); vErr == nil {
				return relocated, true, nil, nil
			} else {
				rErr = vErr
			}
		}
		return d, false, iErr, rErr
	}

	out, ok, inErr, relErr := tryInject(data)
	if ok {
		return out, nil
	}
	appended, aErr := appendImportSection(view, missing)
	if aErr == nil {
		return appended, nil
	}
	return data, artifactValidationError(
		"import manipulation: no safe space for %d new import(s): in-place=%v; relocate=%v; new-section=%v",
		len(missing), inErr, relErr, aErr)
}

const (
	importSectionName = ".idata2"
	// IMAGE_SCN_CNT_INITIALIZED_DATA | IMAGE_SCN_MEM_READ | IMAGE_SCN_MEM_WRITE.
	// The loader writes resolved addresses into the IAT in this section.
	importSectionCharacteristics = 0xC0000040
)

func alignUp64(v, align uint64) uint64 {
	if align <= 1 {
		return v
	}
	return (v + align - 1) / align * align
}

// appendImportSection appends a fresh PE section holding a complete import
// descriptor array (existing descriptors copied verbatim plus missingDLL
// entries) and repoints the import data directory at it.
//
// Appending is intentional: the previous fallback spliced bytes into the
// import section and shifted every later section and the COFF string table,
// but left PointerToSymbolTable at its old offset. debug/pe still reads that
// string table even when NumberOfSymbols is zero, so the generated PE failed
// to parse. It also grew VirtualSize without checking the next section's VA,
// which could make the new table overlap .tls. A new section changes neither
// existing raw offsets nor any virtual range.
func appendImportSection(view *peImportView, missing []missingDLL) ([]byte, error) {
	data := view.data
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return nil, fmt.Errorf("not a PE image")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOff < 0 || peOff+24 > len(data) {
		return nil, fmt.Errorf("truncated PE header")
	}
	if string(data[peOff:peOff+4]) != "PE\x00\x00" {
		return nil, fmt.Errorf("missing PE signature")
	}
	numSections := int(binary.LittleEndian.Uint16(data[peOff+6 : peOff+8]))
	if numSections <= 0 || numSections >= 96 {
		return nil, fmt.Errorf("cannot add a section to a PE with %d sections", numSections)
	}
	sizeOpt := int(binary.LittleEndian.Uint16(data[peOff+20 : peOff+22]))
	optOff := peOff + 24
	if sizeOpt < 0x60 || optOff+sizeOpt > len(data) {
		return nil, fmt.Errorf("truncated PE optional header")
	}
	magic := binary.LittleEndian.Uint16(data[optOff : optOff+2])
	var dataDirOff int
	switch magic {
	case 0x10B:
		dataDirOff = optOff + 0x60
	case 0x20B:
		dataDirOff = optOff + 0x70
	default:
		return nil, fmt.Errorf("unsupported PE optional-header magic 0x%04x", magic)
	}
	if dataDirOff+5*8 > len(data) || dataDirOff+5*8 > optOff+sizeOpt {
		return nil, fmt.Errorf("truncated PE data directories")
	}
	if signed, err := peHasCertificateTable(data); err != nil {
		return nil, err
	} else if signed {
		return nil, fmt.Errorf("cannot add imports to a signed PE (certificate table present)")
	}

	sectionAlign := binary.LittleEndian.Uint32(data[optOff+32:])
	fileAlign := binary.LittleEndian.Uint32(data[optOff+36:])
	if sectionAlign == 0 {
		sectionAlign = 0x1000
	}
	if fileAlign == 0 {
		fileAlign = 0x200
	}
	sizeOfImage := binary.LittleEndian.Uint32(data[optOff+56:])
	sizeOfHeaders := binary.LittleEndian.Uint32(data[optOff+60:])
	sizeOfInit := binary.LittleEndian.Uint32(data[optOff+8:])
	oldChecksum := binary.LittleEndian.Uint32(data[optOff+64:])

	secOff := optOff + sizeOpt
	hdrEnd := secOff + numSections*40
	if hdrEnd+40 > len(data) {
		return nil, fmt.Errorf("no room for another section header")
	}
	// The header table must stay inside SizeOfHeaders and must not overwrite
	// the first section's raw data. Some minimal fixtures omit SizeOfHeaders,
	// so also check the first raw pointer directly.
	if sizeOfHeaders != 0 && uint32(hdrEnd+40) > sizeOfHeaders {
		return nil, fmt.Errorf("no room for another section header inside SizeOfHeaders")
	}
	minRaw := uint32(0)
	for _, s := range view.sections {
		if s.rawSize == 0 {
			continue
		}
		if minRaw == 0 || s.rawPtr < minRaw {
			minRaw = s.rawPtr
		}
	}
	if minRaw != 0 && uint32(hdrEnd+40) > minRaw {
		return nil, fmt.Errorf("section header table would overlap section data")
	}

	// Lay out the descriptor array at offset zero, then the per-DLL thunk,
	// hint/name and DLL-name data. Existing descriptors are copied verbatim;
	// their ILT/IAT/name RVAs remain valid because the new section does not
	// move any existing bytes.
	descEntries := view.descCount + len(missing) + 1
	descBytes64 := uint64(20) * uint64(descEntries)
	if descBytes64 > uint64(^uint32(0)) {
		return nil, fmt.Errorf("import descriptor table is too large")
	}
	descBytes := uint32(descBytes64)
	type newImportLayout struct {
		ilt, iat, hint, name int
	}
	layout := make([]newImportLayout, len(missing))
	align8 := func(pos int) int { return (pos + 7) &^ 7 }
	pos := int(descBytes64)
	for i, m := range missing {
		pos = align8(pos)
		layout[i].ilt = pos
		pos += 16
		pos = align8(pos)
		layout[i].iat = pos
		pos += 16
		pos = align8(pos)
		layout[i].hint = pos
		pos += 2 + len(m.export) + 1
		pos = align8(pos)
		layout[i].name = pos
		pos += len(m.name) + 1
	}
	used := pos
	rawSize64 := alignUp64(uint64(used), uint64(fileAlign))
	if rawSize64 == 0 || rawSize64 > uint64(^uint32(0)) {
		return nil, fmt.Errorf("import section size is invalid")
	}
	rawSize := uint32(rawSize64)

	maxEnd := uint64(sizeOfImage)
	for _, s := range view.sections {
		extent := uint64(s.virtSz)
		if uint64(s.rawSize) > extent {
			extent = uint64(s.rawSize)
		}
		end := uint64(s.va) + extent
		if end > maxEnd {
			maxEnd = end
		}
	}
	newVA64 := alignUp64(maxEnd, uint64(sectionAlign))
	sizeOfImage64 := alignUp64(newVA64+uint64(rawSize), uint64(sectionAlign))
	newRaw64 := alignUp64(uint64(len(data)), uint64(fileAlign))
	if newVA64 > uint64(^uint32(0)) || sizeOfImage64 > uint64(^uint32(0)) ||
		newRaw64 > uint64(^uint32(0)) || newRaw64+uint64(rawSize) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("PE image is too large to extend")
	}
	newVA := uint32(newVA64)
	newRaw := uint32(newRaw64)
	newSizeOfImage := uint32(sizeOfImage64)
	outLen64 := newRaw64 + uint64(rawSize)
	maxInt := uint64(^uint(0) >> 1)
	if outLen64 > maxInt {
		return nil, fmt.Errorf("PE image is too large for this host")
	}
	out := make([]byte, int(outLen64))
	copy(out, data)
	payload := out[int(newRaw):int(newRaw64+uint64(rawSize))]

	copy(payload[:20*view.descCount], data[view.importOff:view.importOff+20*view.descCount])
	thunkSize := view.thunkSize
	if thunkSize == 0 {
		thunkSize = 4
		if view.is64 {
			thunkSize = 8
		}
	}
	for i, m := range missing {
		l := layout[i]
		if l.ilt+16 > len(payload) || l.iat+16 > len(payload) ||
			l.hint+2+len(m.export)+1 > len(payload) ||
			l.name+len(m.name)+1 > len(payload) {
			return nil, fmt.Errorf("import section layout overflow")
		}
		hintNameRVA := newVA + uint32(l.hint)
		nameRVA := newVA + uint32(l.name)
		iltRVA := newVA + uint32(l.ilt)
		iatRVA := newVA + uint32(l.iat)
		payload[l.hint], payload[l.hint+1] = 0, 0
		copy(payload[l.hint+2:], m.export)
		payload[l.hint+2+len(m.export)] = 0
		copy(payload[l.name:], m.name)
		payload[l.name+len(m.name)] = 0
		if thunkSize == 8 {
			binary.LittleEndian.PutUint64(payload[l.ilt:], uint64(hintNameRVA))
			binary.LittleEndian.PutUint64(payload[l.ilt+8:], 0)
			binary.LittleEndian.PutUint64(payload[l.iat:], uint64(hintNameRVA))
			binary.LittleEndian.PutUint64(payload[l.iat+8:], 0)
		} else {
			binary.LittleEndian.PutUint32(payload[l.ilt:], hintNameRVA)
			binary.LittleEndian.PutUint32(payload[l.ilt+4:], 0)
			binary.LittleEndian.PutUint32(payload[l.iat:], hintNameRVA)
			binary.LittleEndian.PutUint32(payload[l.iat+4:], 0)
		}
		dpos := 20 * (view.descCount + i)
		binary.LittleEndian.PutUint32(payload[dpos:], iltRVA)
		binary.LittleEndian.PutUint32(payload[dpos+4:], 0)
		binary.LittleEndian.PutUint32(payload[dpos+8:], 0)
		binary.LittleEndian.PutUint32(payload[dpos+12:], nameRVA)
		binary.LittleEndian.PutUint32(payload[dpos+16:], iatRVA)
	}
	// The null descriptor after the last new entry is already zero.

	hdr := out[hdrEnd : hdrEnd+40]
	for i := range hdr {
		hdr[i] = 0
	}
	copy(hdr, importSectionName)
	binary.LittleEndian.PutUint32(hdr[8:], rawSize)  // VirtualSize
	binary.LittleEndian.PutUint32(hdr[12:], newVA)   // VirtualAddress
	binary.LittleEndian.PutUint32(hdr[16:], rawSize) // SizeOfRawData
	binary.LittleEndian.PutUint32(hdr[20:], newRaw)  // PointerToRawData
	binary.LittleEndian.PutUint32(hdr[36:], importSectionCharacteristics)

	binary.LittleEndian.PutUint16(out[peOff+6:], uint16(numSections+1))
	binary.LittleEndian.PutUint32(out[optOff+56:], newSizeOfImage)
	if uint64(sizeOfInit)+uint64(rawSize) <= uint64(^uint32(0)) {
		binary.LittleEndian.PutUint32(out[optOff+8:], sizeOfInit+rawSize)
	}
	binary.LittleEndian.PutUint32(out[dataDirOff+8:], newVA)
	binary.LittleEndian.PutUint32(out[dataDirOff+12:], descBytes)

	// Preserve a checksum that was already present; a zero checksum is valid
	// for Windows user-mode images and is left untouched.
	if oldChecksum != 0 {
		binary.LittleEndian.PutUint32(out[optOff+64:], computePEChecksum(out, optOff+64))
	}
	return out, nil
}

// layoutMissingImportData returns the file offset just past the ILT/IAT/
// hint-name/DLL-name data for missing, starting at start. Using the real
// export and DLL names (rather than a fixed maximum) keeps the relocation
// bound exact; otherwise a long name can overrun the section by a few bytes.
func layoutMissingImportData(start int, missing []missingDLL) int {
	align8 := func(pos int) int { return (pos + 7) &^ 7 }
	pos := start
	for _, m := range missing {
		pos = align8(pos)
		pos += 16 // ILT
		pos = align8(pos)
		pos += 16 // IAT
		pos += 2 + len(m.export) + 1
		pos += len(m.name) + 1
	}
	return pos
}

// injectImportsInPlace appends new import descriptors immediately after the
// existing table when the section has contiguous zero slack there.
func injectImportsInPlace(view *peImportView, missing []missingDLL) error {
	align8 := func(pos int) int { return (pos + 7) &^ 7 }
	p := view.tableEnd + 20*len(missing)
	for _, m := range missing {
		p = align8(p)
		p += 16 // ILT
		p = align8(p)
		p += 16 // IAT
		p += 2 + len(m.export) + 1
		p += len(m.name) + 1
	}
	if p > view.sectionRawEnd {
		return fmt.Errorf("no slack space after the import table")
	}
	// The old null terminator is already zero; everything through the end of
	// the new thunk area must be untouched zeros so the PE keeps loading.
	for i := view.tableEnd - 20; i < p; i++ {
		if view.data[i] != 0 {
			return fmt.Errorf("no contiguous zero slack after the import table")
		}
	}

	// Write the new descriptors over the old null terminator, followed by a
	// fresh null terminator, then the thunk/name data.
	descPos := view.importOff + view.descCount*20
	writeU32 := func(pos int, v uint32) {
		binary.LittleEndian.PutUint32(view.data[pos:], v)
	}
	rvaAt := func(pos int, what string) (uint32, error) {
		rva, ok := view.offsetToRVA(pos)
		if !ok {
			return 0, fmt.Errorf("%s at file offset 0x%x is outside section raw data", what, pos)
		}
		return rva, nil
	}
	pos := view.tableEnd + 20*len(missing)
	for k, m := range missing {
		pos = align8(pos)
		iltPos := pos
		iltRVA, err := rvaAt(pos, "ILT")
		if err != nil {
			return err
		}
		pos += 16
		pos = align8(pos)
		iatPos := pos
		iatRVA, err := rvaAt(pos, "IAT")
		if err != nil {
			return err
		}
		pos += 16
		hintNameRVA, err := rvaAt(pos, "hint/name")
		if err != nil {
			return err
		}
		if pos+2+len(m.export)+1 > len(view.data) {
			return fmt.Errorf("hint/name data exceeds the image")
		}
		view.data[pos] = 0 // 2-byte hint
		view.data[pos+1] = 0
		copy(view.data[pos+2:], m.export)
		view.data[pos+2+len(m.export)] = 0
		pos += 2 + len(m.export) + 1
		nameRVA, err := rvaAt(pos, "DLL name")
		if err != nil {
			return err
		}
		if pos+len(m.name)+1 > len(view.data) {
			return fmt.Errorf("DLL name data exceeds the image")
		}
		copy(view.data[pos:], m.name)
		view.data[pos+len(m.name)] = 0
		pos += len(m.name) + 1

		dpos := descPos + k*20
		writeU32(dpos, iltRVA)        // OriginalFirstThunk -> ILT
		writeU32(dpos+4, 0)           // TimeDateStamp
		writeU32(dpos+8, 0)           // ForwarderChain
		writeU32(dpos+12, nameRVA)    // Name
		writeU32(dpos+16, iatRVA)     // FirstThunk -> IAT
		writeU32(iltPos, hintNameRVA) // first thunk -> hint/name
		writeU32(iltPos+8, 0)         // null terminator thunk
		writeU32(iatPos, hintNameRVA)
		writeU32(iatPos+8, 0)
	}
	// Null terminator after the new descriptors.
	nullPos := descPos + len(missing)*20
	if nullPos+20 > len(view.data) {
		return fmt.Errorf("import descriptor null terminator exceeds the image")
	}
	for i := 0; i < 20; i++ {
		view.data[nullPos+i] = 0
	}
	// The loader walks the table from the data directory's declared size, so
	// it must cover the new entries (plus the null terminator). The section
	// VirtualSize is raised to the new table extent when the old size is
	// page-rounded below it.
	view.setImportDirectory(view.importRVA, uint32(20*(view.descCount+len(missing)+1)))
	if err := view.expandImportSectionVirtualSize(uint32(p - view.rawPtrForImport())); err != nil {
		return err
	}
	return nil
}

// injectImportsRelocated rewrites the import descriptor array into the
// trailing zero region of the import section and repoints the import data
// directory at it. Existing descriptors are copied verbatim (their ILT/IAT
// references stay at their original RVAs), so existing imports keep binding
// through the same thunks the code already points at.
func injectImportsRelocated(view *peImportView, missing []missingDLL) error {
	align8 := func(pos int) int { return (pos + 7) &^ 7 }

	// Find the trailing zero region of the section that owns the import
	// table: from the last non-zero byte to the end of the section's raw
	// data. Go toolchain binaries always leave this padding in .idata.
	trailStart := view.sectionRawEnd
	for trailStart > view.importOff && view.data[trailStart-1] == 0 {
		trailStart--
	}
	if trailStart >= view.sectionRawEnd {
		return fmt.Errorf("no trailing zero space in import section")
	}

	// The last zero bytes before the padding are often a live null terminator:
	// Go places the ILT's terminating thunk exactly at VirtualSize. Relocating
	// the descriptor array to that offset would overwrite the terminator (the
	// original descriptor still points at the ILT), so start after the whole
	// old mapped region plus one full thunk-entry guard. This keeps every
	// existing import structure intact; the section's VirtualSize is extended
	// below to map the new table.
	host, ok := view.importSection()
	if !ok {
		return fmt.Errorf("import section not found")
	}
	guardStart := int(host.rawPtr) + int(host.virtSz) + view.thunkSize
	guardStart = (guardStart + 3) &^ 3
	if guardStart > trailStart {
		trailStart = guardStart
	}
	if trailStart >= view.sectionRawEnd {
		return fmt.Errorf("no trailing zero space after the mapped import data")
	}

	// Needed: relocated descriptor array (existing + new + null) plus the new
	// ILT/IAT/name data. Compute the data extent from the actual names so a
	// long export or DLL name cannot overrun the section.
	dataStart := trailStart + 20*(view.descCount+len(missing)+1)
	needed := layoutMissingImportData(dataStart, missing) - trailStart
	if needed <= 0 || trailStart+needed > view.sectionRawEnd {
		return fmt.Errorf("trailing zero space too small")
	}
	// The section's VirtualSize must cover the rewritten array. The raw data
	// exists (it is part of the file), so this only requires the mapped
	// region not to collide with the next section.
	requiredVsz := (trailStart - view.rawPtrForImport()) + needed
	if err := view.expandImportSectionVirtualSize(uint32(requiredVsz)); err != nil {
		return err
	}

	// Copy the existing descriptors verbatim, then append the new ones and a
	// fresh null terminator. Thunk/name data is laid out AFTER the whole
	// descriptor array (descriptors + new entries + null), so it never
	// overlaps the array itself.
	firstNewDesc := trailStart + 20*view.descCount
	pos := dataStart
	copy(view.data[trailStart:firstNewDesc], view.data[view.importOff:view.importOff+20*view.descCount])
	writeU32 := func(p int, v uint32) {
		binary.LittleEndian.PutUint32(view.data[p:], v)
	}
	rvaAt := func(p int, what string) (uint32, error) {
		rva, ok := view.offsetToRVA(p)
		if !ok {
			return 0, fmt.Errorf("%s at file offset 0x%x is outside section raw data", what, p)
		}
		return rva, nil
	}
	for k, m := range missing {
		pos = align8(pos)
		iltPos := pos
		iltRVA, err := rvaAt(pos, "ILT")
		if err != nil {
			return err
		}
		pos += 16
		pos = align8(pos)
		iatPos := pos
		iatRVA, err := rvaAt(pos, "IAT")
		if err != nil {
			return err
		}
		pos += 16
		hintNameRVA, err := rvaAt(pos, "hint/name")
		if err != nil {
			return err
		}
		if pos+2+len(m.export)+1 > len(view.data) {
			return fmt.Errorf("hint/name data exceeds the image")
		}
		view.data[pos] = 0
		view.data[pos+1] = 0
		copy(view.data[pos+2:], m.export)
		view.data[pos+2+len(m.export)] = 0
		pos += 2 + len(m.export) + 1
		nameRVA, err := rvaAt(pos, "DLL name")
		if err != nil {
			return err
		}
		if pos+len(m.name)+1 > len(view.data) {
			return fmt.Errorf("DLL name data exceeds the image")
		}
		copy(view.data[pos:], m.name)
		view.data[pos+len(m.name)] = 0
		pos += len(m.name) + 1

		dpos := firstNewDesc + k*20
		writeU32(dpos, iltRVA)
		writeU32(dpos+4, 0)
		writeU32(dpos+8, 0)
		writeU32(dpos+12, nameRVA)
		writeU32(dpos+16, iatRVA)
		writeU32(iltPos, hintNameRVA)
		writeU32(iltPos+8, 0)
		writeU32(iatPos, hintNameRVA)
		writeU32(iatPos+8, 0)
	}
	nullPos := dataStart - 20
	if nullPos < trailStart || nullPos+20 > len(view.data) {
		return fmt.Errorf("import descriptor null terminator is outside the section")
	}
	for i := 0; i < 20; i++ {
		view.data[nullPos+i] = 0
	}

	// Repoint the import data directory at the relocated array.
	newRVA, ok := view.offsetToRVA(trailStart)
	if !ok {
		return fmt.Errorf("relocated import table is outside section raw data")
	}
	view.setImportDirectory(newRVA, uint32(20*(view.descCount+len(missing)+1)))
	return nil
}

// importSection returns the section that owns the import descriptor table.
func (v *peImportView) importSection() (peSectionView, bool) {
	for _, s := range v.sections {
		if uint32(v.importOff) >= s.rawPtr && uint32(v.importOff) < s.rawPtr+s.rawSize {
			return s, true
		}
	}
	return peSectionView{}, false
}

// rawPtrForImport returns the PointerToRawData of the section that owns the
// import table.
func (v *peImportView) rawPtrForImport() int {
	if s, ok := v.importSection(); ok {
		return int(s.rawPtr)
	}
	return 0
}

// expandImportSectionVirtualSize raises the VirtualSize of the section that
// owns the import table to cover the rewritten data, failing when the mapped
// region would collide with the next section.
func (v *peImportView) expandImportSectionVirtualSize(required uint32) error {
	data := v.data
	if len(data) < 0x40 {
		return fmt.Errorf("truncated PE header")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOff+24 > len(data) {
		return fmt.Errorf("truncated PE header")
	}
	numSections := int(binary.LittleEndian.Uint16(data[peOff+6 : peOff+8]))
	sizeOpt := int(binary.LittleEndian.Uint16(data[peOff+20 : peOff+22]))
	optOff := peOff + 24
	secOff := optOff + sizeOpt
	if secOff+numSections*40 > len(data) || optOff+56+4 > len(data) {
		return fmt.Errorf("truncated PE section table")
	}
	idx := -1
	importSectionRaw := v.rawPtrForImport()
	for i := 0; i < numSections; i++ {
		h := secOff + i*40
		rawPtr := binary.LittleEndian.Uint32(data[h+20:])
		if rawPtr == uint32(importSectionRaw) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("import section header not found")
	}
	curVsz := binary.LittleEndian.Uint32(data[secOff+idx*40+8:])
	if required <= curVsz {
		return nil
	}
	// Next section VA limits the mapped extent.
	nextVA := uint32(0)
	baseVA := binary.LittleEndian.Uint32(data[secOff+idx*40+12:])
	for i := 0; i < numSections; i++ {
		h := secOff + i*40
		va := binary.LittleEndian.Uint32(data[h+12:])
		if va > baseVA && (nextVA == 0 || va < nextVA) {
			nextVA = va
		}
	}
	if nextVA != 0 && uint64(baseVA)+uint64(required) > uint64(nextVA) {
		return fmt.Errorf("import section cannot be extended without overlapping the next section")
	}
	binary.LittleEndian.PutUint32(data[secOff+idx*40+8:], required)

	// Grow SizeOfImage to cover the new mapped extent. Without this, extending
	// the last section would pass the local bounds check here but fail
	// validation (or be truncated by the loader).
	sectionAlign := binary.LittleEndian.Uint32(data[optOff+32:])
	if sectionAlign == 0 {
		sectionAlign = 0x1000
	}
	maxEnd := uint64(0)
	for i := 0; i < numSections; i++ {
		h := secOff + i*40
		va := binary.LittleEndian.Uint32(data[h+12:])
		vsz := binary.LittleEndian.Uint32(data[h+8:])
		raw := binary.LittleEndian.Uint32(data[h+16:])
		if raw > vsz {
			vsz = raw
		}
		if end := uint64(va) + uint64(vsz); end > maxEnd {
			maxEnd = end
		}
	}
	if sizeOfImage := alignUp64(maxEnd, uint64(sectionAlign)); sizeOfImage <= uint64(^uint32(0)) {
		binary.LittleEndian.PutUint32(data[optOff+56:], uint32(sizeOfImage))
	}
	return nil
}

// setImportDirectory rewrites the IMAGE_DIRECTORY_ENTRY_IMPORT entry (RVA and
// size) in the optional header's data directory.
func (v *peImportView) setImportDirectory(rva, size uint32) {
	data := v.data
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	magic := binary.LittleEndian.Uint16(data[peOff+24 : peOff+26])
	dirBase := peOff + 24 + 0x60
	if magic == 0x20B {
		dirBase = peOff + 24 + 0x70
	}
	binary.LittleEndian.PutUint32(data[dirBase+8:], rva)
	binary.LittleEndian.PutUint32(data[dirBase+12:], size)
}
