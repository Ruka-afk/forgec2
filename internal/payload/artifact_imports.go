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
	"kernel32.dll":  "GetModuleHandleA",
	"advapi32.dll":  "RegOpenKeyExA",
	"user32.dll":    "MessageBoxA",
	"ws2_32.dll":    "WSAStartup",
	"bcrypt.dll":    "BCryptOpenAlgorithmProvider",
	"ntdll.dll":     "NtQueryInformationProcess",
	"shell32.dll":   "ShellExecuteA",
	"ole32.dll":     "CoCreateInstance",
	"wininet.dll":   "InternetOpenA",
	"gdi32.dll":     "GetDeviceCaps",
	"comctl32.dll":  "InitCommonControlsEx",
	"version.dll":   "GetFileVersionInfoA",
	"crypt32.dll":   "CertOpenStore",
	"iphlpapi.dll":  "GetAdaptersInfo",
	"netapi32.dll":  "NetApiBufferFree",
	"psapi.dll":     "EnumProcesses",
	"secur32.dll":   "AcquireCredentialsHandleA",
	"setupapi.dll":  "SetupDiGetClassDevsA",
	"winmm.dll":     "waveOutOpen",
	"wldap32.dll":   "ldap_init",
}

type peSectionView struct {
	rawPtr  uint32
	rawSize uint32
	va      uint32
	virtSz  uint32
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
// PE image. Already-imported DLLs are left untouched; missing DLLs are appended
// after the existing import descriptor table when the owning section has
// contiguous zero slack, otherwise an explicit error is returned instead of
// silently doing nothing.
func AddBenignImports(data []byte, dlls []string) error {
	if len(dlls) == 0 {
		return nil
	}
	view, err := parseImportView(data)
	if err != nil {
		return fmt.Errorf("import manipulation: %w", err)
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

	type missingDLL struct {
		name   string
		export string
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
			return fmt.Errorf("import manipulation: no known benign export for %s", name)
		}
		missing = append(missing, missingDLL{name: name, export: export})
	}
	if len(missing) == 0 {
		return nil
	}

	// Space needed: new descriptors plus a final null terminator, then
	// per-DLL ILT (16B), IAT (16B), hint/name and DLL name strings.
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
		return fmt.Errorf("import manipulation: no slack space after the import table (%d bytes needed, %d available); binary cannot be extended in place", p-view.tableEnd+20, view.sectionRawEnd-(view.tableEnd-20))
	}
	// The old null terminator is already zero; everything through the end of
	// the new thunk area must be untouched zeros so the PE keeps loading.
	for i := view.tableEnd - 20; i < p; i++ {
		if view.data[i] != 0 {
			return fmt.Errorf("import manipulation: no contiguous zero slack after the import table")
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
		writeU32(dpos, iltRVA)          // OriginalFirstThunk -> ILT
		writeU32(dpos+4, 0)             // TimeDateStamp
		writeU32(dpos+8, 0)             // ForwarderChain
		writeU32(dpos+12, nameRVA)      // Name
		writeU32(dpos+16, iatRVA)       // FirstThunk -> IAT
		writeU32(iltPos, hintNameRVA)   // first thunk -> hint/name
		writeU32(iltPos+8, 0)           // null terminator thunk
		writeU32(iatPos, hintNameRVA)
		writeU32(iatPos+8, 0)
	}
	// Null terminator after the new descriptors.
	nullPos := descPos + len(missing)*20
	for i := 0; i < 20; i++ {
		view.data[nullPos+i] = 0
	}
	return nil
}
