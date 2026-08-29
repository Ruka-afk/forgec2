package payload

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
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
		max := big.NewInt(4102444800)
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return 0, err
		}
		return uint32(n.Int64()), nil
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
	peOffset := int(data[0x3C]) | int(data[0x3D])<<8
	tsOffset := peOffset + 8
	if tsOffset+4 <= len(data) {
		binary.LittleEndian.PutUint32(data[tsOffset:], ts)
	}
}

// ApplyPESectionNames rewrites the section header names of a PE image.
// The section table starts after the optional header, whose size must be read
// from IMAGE_FILE_HEADER (a hardcoded offset silently fails on PE32+ images).
func ApplyPESectionNames(data []byte, sections PESectionConfig) {
	if len(data) < 0x100 || data[0] != 'M' || data[1] != 'Z' {
		return
	}
	peOffset := int(data[0x3C]) | int(data[0x3D])<<8
	if peOffset+24 >= len(data) {
		return
	}
	if data[peOffset] != 'P' || data[peOffset+1] != 'E' {
		return
	}
	numSections := int(data[peOffset+6]) | int(data[peOffset+7])<<8
	if numSections <= 0 || numSections > 96 {
		return
	}
	sizeOptHeader := int(data[peOffset+20]) | int(data[peOffset+21])<<8
	if sizeOptHeader == 0 {
		return
	}
	sectionOffset := peOffset + 4 + 20 + sizeOptHeader
	if sectionOffset+40*numSections > len(data) {
		return
	}
	names := []string{sections.Text, sections.Data, sections.Rdata, sections.Reloc}
	max := numSections
	if len(names) < max {
		max = len(names)
	}
	for i := 0; i < max; i++ {
		name := names[i]
		if name == "" {
			continue
		}
		off := sectionOffset + i*40
		b := make([]byte, 8)
		copy(b, name)
		copy(data[off:off+8], b)
	}
}

// (AddBenignImports lives in artifact_imports.go)
