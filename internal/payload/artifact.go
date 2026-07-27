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
		CertOption:          CertSelfSigned,
		ImportManipulation:  false,
		BenignImports:       []string{},
		EmbedResources:      false,
		ShellcodeEncode:     EncodeNone,
	}
}

func BuiltinTemplates() []ArtifactTemplate {
	return []ArtifactTemplate{
		{
			Name:                "default_exe",
			Description:         "Standard EXE with default PE sections and direct entry point",
			OutputType:          "exe",
			PESections:          DefaultPESections(),
			EntryPointTechnique: EPDirect,
			Timestamp:           TSRandom,
			CertOption:          CertSelfSigned,
			ImportManipulation:  false,
			ShellcodeEncode:     EncodeNone,
		},
		{
			Name:                "default_dll",
			Description:         "Standard DLL with custom export table",
			OutputType:          "dll",
			PESections:          DefaultPESections(),
			EntryPointTechnique: EPCallback,
			Timestamp:           TSRandom,
			CertOption:          CertSelfSigned,
			ImportManipulation:  true,
			BenignImports:       []string{"kernel32.dll", "advapi32.dll", "ws2_32.dll"},
			ShellcodeEncode:     EncodeXOR,
		},
		{
			Name:                "service_exe",
			Description:         "Service EXE with ServiceMain entry point",
			OutputType:          "service_exe",
			PESections:          DefaultPESections(),
			EntryPointTechnique: EPCallback,
			Timestamp:           TSKeep,
			CertOption:          CertNone,
			ImportManipulation:  true,
			BenignImports:       []string{"advapi32.dll", "kernel32.dll"},
			ShellcodeEncode:     EncodeNone,
		},
		{
			Name:                "xor_loader",
			Description:         "XOR-encoded shellcode loader",
			OutputType:          "shellcode",
			PESections:          PESectionConfig{Text: ".text", Data: ".data", Rdata: ".rdata", Reloc: ".reloc"},
			EntryPointTechnique: EPDirect,
			Timestamp:           TSRandom,
			CertOption:          CertSelfSigned,
			ImportManipulation:  true,
			BenignImports:       []string{"kernel32.dll", "user32.dll"},
			ShellcodeEncode:     EncodeXOR,
		},
		{
			Name:                "aes_loader",
			Description:         "AES-encrypted shellcode loader",
			OutputType:          "shellcode",
			PESections:          PESectionConfig{Text: ".text", Data: ".data", Rdata: ".rdata", Reloc: ".reloc"},
			EntryPointTechnique: EPDirect,
			Timestamp:           TSRandom,
			CertOption:          CertSelfSigned,
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

func ApplyPESectionNames(data []byte, sections PESectionConfig) {
	if len(data) < 0x100 || data[0] != 'M' || data[1] != 'Z' {
		return
	}
	peOffset := int(data[0x3C]) | int(data[0x3D])<<8
	if peOffset+4 >= len(data) {
		return
	}
	sectionOffset := peOffset + 0xF8
	if sectionOffset+40*4 > len(data) {
		return
	}
	names := []string{sections.Text, sections.Data, sections.Rdata, sections.Reloc}
	for i, name := range names {
		if name == "" {
			continue
		}
		off := sectionOffset + i*40
		if off+8 > len(data) {
			continue
		}
		b := make([]byte, 8)
		copy(b, name)
		for j := len(name); j < 8; j++ {
			b[j] = 0
		}
		copy(data[off:off+8], b)
	}
}

func AddBenignImports(data []byte, dlls []string) {
	_ = data
	_ = dlls
}
