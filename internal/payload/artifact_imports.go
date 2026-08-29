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
	importOff     int
	descCount     int // number of descriptors before the null terminator
	tableEnd      int // file offset just past the null descriptor
	sectionRawEnd int // file offset of the end of the section holding the table
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
	peOff := int(data[0x3C]) | int(data[0x3D])<<8
	if peOff+24 >= len(data) {
		return nil, fmt.Errorf("not a PE image (truncated header)")
	}
	if data[peOff] != 'P' || data[peOff+1] != 'E' {
		return nil, fmt.Errorf("not a PE image (missing PE signature)")
	}
	numSections := int(data[peOff+6]) | int(data[peOff+7])<<8
	if numSections <= 0 || numSections > 96 {
		return nil, fmt.Errorf("invalid section count %d", numSections)
	}
	sizeOpt := int(data[peOff+20]) | int(data[peOff+21])<<8
	if sizeOpt == 0 {
		return nil, fmt.Errorf("missing optional header")
	}
	magic := uint16(data[peOff+24]) | uint16(data[peOff+25])<<8
	var dataDirOff int
	switch magic {
	case 0x10B: // PE32
		dataDirOff = peOff + 24 + 0x60
	case 0x20B: // PE32+
		dataDirOff = peOff + 24 + 0x70
	default:
		return nil, fmt.Errorf("unsupported optional header magic 0x%04X", magic)
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
		data:      data,
		sections:  sections,
		importRVA: importRVA,
		importOff: importOff,
	}

	maxEntries := int(importSize) / 20
	if maxEntries == 0 {
		maxEntries = 512
	}
	if maxEntries > 4096 {
		maxEntries = 4096
	}
	terminated := false
	for i := 0; i < maxEntries; i++ {
		base := importOff + i*20
		if base+20 > len(data) {
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
		return nil, fmt.Errorf("import descriptor table is unterminated")
	}

	// Bounds of the section that owns the import table.
	sec, ok := sectionForOffset(sections, view.importOff)
	if !ok {
		return nil, fmt.Errorf("import directory not within any section")
	}
	view.sectionRawEnd = int(sec.rawPtr + sec.rawSize)
	if view.sectionRawEnd > len(data) {
		view.sectionRawEnd = len(data)
	}
	return view, nil
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
// If neither strategy has enough zero space, an explicit error is returned
// instead of silently doing nothing.
// missingDLL pairs a DLL to add with a benign export that provably exists in
// every shipped version of that DLL.
type missingDLL struct {
	name   string
	export string
}

// AddBenignImports injects benign DLL imports into a PE, growing the terminal
// section (raw+virtual, aligned) when the existing layout lacks physical
// slack. Returns the possibly reallocated image bytes.
func AddBenignImports(data []byte, dlls []string) ([]byte, error) {
	if len(dlls) == 0 {
		return data, nil
	}
	view, err := parseImportView(data)
	if err != nil {
		return data, fmt.Errorf("import manipulation: %w", err)
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
		v, perr := parseImportView(d)
		if perr != nil {
			return d, false, perr, nil
		}
		iErr := injectImportsInPlace(v, missing)
		if iErr == nil {
			return d, true, nil, nil
		}
		rErr := injectImportsRelocated(v, missing)
		if rErr == nil {
			return d, true, nil, nil
		}
		return d, false, iErr, rErr
	}

	if out, ok, _, _ := tryInject(data); ok {
		return out, nil
	}

	// Growth fallback: the import table's host section is grown in place —
	// aligned zeros spliced at its raw end, subsequent sections' file offsets
	// shifted, and SizeOfImage recomputed. Go's linker packs VirtSize ==
	// RawSize so there is never pre-existing slack, and any post-link payload
	// growth (e.g. a new embedded feature) starves the injectors.
	hostIdx := -1
	for i, sec := range view.sections {
		end := sec.va + sec.virtSz
		if view.importRVA >= sec.va && view.importRVA < end {
			hostIdx = i
			break
		}
	}
	if hostIdx < 0 {
		return data, artifactValidationError(
			"import manipulation: no space for %d new import(s); import section not found", len(missing))
	}
	sec := view.sections[hostIdx]
	// Size the growth to satisfy injectImportsRelocated's own requirement:
	// relocated descriptor array (existing + new + null) plus ILT/IAT/name
	// data for the new DLLs.
	grow := 20*(view.descCount+len(missing)+1) + spaceForMissing(len(missing))
	if grow < 0x200 {
		grow = 0x200
	}
	grow = ((grow + 0x1ff) / 0x200) * 0x200

	spliceAt := int(sec.rawPtr) + int(sec.rawSize)
	pad := make([]byte, grow)
	out := make([]byte, 0, len(data)+grow)
	out = append(out, data[:spliceAt]...)
	out = append(out, pad...)
	out = append(out, data[spliceAt:]...)

	// Grown section headers: raw and virtual sizes both extend.
	newRaw := uint32(int(sec.rawSize) + grow)
	newVirt := uint32(int(sec.virtSz) + grow)
	binary.LittleEndian.PutUint32(out[sec.hdrOff+8:], newVirt)
	binary.LittleEndian.PutUint32(out[sec.hdrOff+16:], newRaw)

	// Shift every LATER section's file offset by grow (RVAs untouched).
	oldEnd := spliceAt
	for _, other := range view.sections {
		if other.rawPtr >= uint32(oldEnd) {
			np := binary.LittleEndian.Uint32(out[other.hdrOff+20:])
			binary.LittleEndian.PutUint32(out[other.hdrOff+20:], np+uint32(grow))
		}
	}

	// Recompute SizeOfImage from the extended layout (optional header +56),
	// aligned to the section alignment at optional header +32.
	maxEnd := uint32(0)
	for _, s2 := range view.sections {
		v := s2.va + s2.virtSz
		if v > maxEnd {
			maxEnd = v
		}
	}
	if peOff := int(binary.LittleEndian.Uint32(out[0x3C:0x40])); peOff > 0 && peOff+24+56 <= len(out) {
		opt := peOff + 24
		secAlign := binary.LittleEndian.Uint32(out[opt+32:])
		if secAlign == 0 {
			secAlign = 0x1000
		}
		binary.LittleEndian.PutUint32(out[opt+56:], ((maxEnd+secAlign-1)/secAlign)*secAlign)
	}

	out2, ok, iErr2, rErr2 := tryInject(out)
	if ok {
		return out2, nil
	}
	return data, artifactValidationError(
		"import manipulation: no zero slack space for %d new import(s) even after growth: in-place=%v; relocate=%v",
		len(missing), iErr2, rErr2)
}

// spaceForMissing computes the total bytes needed for the new descriptors
// plus per-DLL ILT (16B), IAT (16B), hint/name and DLL name strings.
func spaceForMissing(missing int) int {
	align8 := func(pos int) int { return (pos + 7) &^ 7 }
	p := 20 * missing
	for i := 0; i < missing; i++ {
		p = align8(p)
		p += 16 // ILT
		p = align8(p)
		p += 16         // IAT
		p += 2 + 20 + 1 // longest known benign export name
		p += 20 + 1     // longest known DLL name
	}
	return p
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
	pos := view.tableEnd + 20*len(missing)
	for k, m := range missing {
		pos = align8(pos)
		iltPos := pos
		iltRVA, _ := view.offsetToRVA(pos)
		pos += 16
		pos = align8(pos)
		iatPos := pos
		iatRVA, _ := view.offsetToRVA(pos)
		pos += 16
		hintNameRVA, _ := view.offsetToRVA(pos)
		view.data[pos] = 0 // 2-byte hint
		view.data[pos+1] = 0
		copy(view.data[pos+2:], m.export)
		view.data[pos+2+len(m.export)] = 0
		pos += 2 + len(m.export) + 1
		nameRVA, _ := view.offsetToRVA(pos)
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

	// Needed: relocated descriptor array (existing + new + null) plus the new
	// ILT/IAT/name data.
	needed := 20*(view.descCount+len(missing)+1) + spaceForMissing(len(missing))
	if trailStart+needed > view.sectionRawEnd {
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
	pos := trailStart + 20*(view.descCount+len(missing)+1)
	copy(view.data[trailStart:firstNewDesc], view.data[view.importOff:view.importOff+20*view.descCount])
	writeU32 := func(p int, v uint32) {
		binary.LittleEndian.PutUint32(view.data[p:], v)
	}
	for k, m := range missing {
		pos = align8(pos)
		iltPos := pos
		iltRVA, _ := view.offsetToRVA(pos)
		pos += 16
		pos = align8(pos)
		iatPos := pos
		iatRVA, _ := view.offsetToRVA(pos)
		pos += 16
		hintNameRVA, _ := view.offsetToRVA(pos)
		view.data[pos] = 0
		view.data[pos+1] = 0
		copy(view.data[pos+2:], m.export)
		view.data[pos+2+len(m.export)] = 0
		pos += 2 + len(m.export) + 1
		nameRVA, _ := view.offsetToRVA(pos)
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
	for i := 0; i < 20; i++ {
		view.data[pos+i] = 0
	}

	// Repoint the import data directory at the relocated array.
	newRVA, _ := view.offsetToRVA(trailStart)
	view.setImportDirectory(newRVA, uint32(20*(view.descCount+len(missing)+1)))
	return nil
}

// rawPtrForImport returns the PointerToRawData of the section that owns the
// import table.
func (v *peImportView) rawPtrForImport() int {
	for _, s := range v.sections {
		if uint32(v.importOff) >= s.rawPtr && uint32(v.importOff) < s.rawPtr+s.rawSize {
			return int(s.rawPtr)
		}
	}
	return 0
}

// expandImportSectionVirtualSize raises the VirtualSize of the section that
// owns the import table to cover the rewritten data, failing when the mapped
// region would collide with the next section.
func (v *peImportView) expandImportSectionVirtualSize(required uint32) error {
	data := v.data
	peOff := int(data[0x3C]) | int(data[0x3D])<<8
	numSections := int(data[peOff+6]) | int(data[peOff+7])<<8
	sizeOpt := int(data[peOff+20]) | int(data[peOff+21])<<8
	secOff := peOff + 4 + 20 + sizeOpt
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
	if nextVA != 0 && baseVA+required > nextVA {
		return fmt.Errorf("import section cannot be extended without overlapping the next section")
	}
	binary.LittleEndian.PutUint32(data[secOff+idx*40+8:], required)
	return nil
}

// setImportDirectory rewrites the IMAGE_DIRECTORY_ENTRY_IMPORT entry (RVA and
// size) in the optional header's data directory.
func (v *peImportView) setImportDirectory(rva, size uint32) {
	data := v.data
	peOff := int(data[0x3C]) | int(data[0x3D])<<8
	magic := binary.LittleEndian.Uint16(data[peOff+24:])
	dirBase := peOff + 24 + 0x60
	if magic == 0x20B {
		dirBase = peOff + 24 + 0x70
	}
	binary.LittleEndian.PutUint32(data[dirBase+8:], rva)
	binary.LittleEndian.PutUint32(data[dirBase+12:], size)
}
