#pragma once
#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

/* All functions return 0 on success, nonzero on failure (fail closed). */

/* Cryptographically secure random bytes (CNG RNG). */
int cng_random(BYTE *out, DWORD len);

/* SHA-256 (CNG). out must hold 32 bytes. */
int cng_sha256(const BYTE *data, DWORD len, BYTE out[32]);

/* HMAC-SHA256 (CNG). out must hold 32 bytes. */
int cng_hmac_sha256(const BYTE *key, DWORD keylen,
                    const BYTE *data, DWORD datalen, BYTE out[32]);

/* HKDF-SHA256 extract+expand, L=32 (RFC 5869, matches agent hkdfSHA256). */
int cng_hkdf_sha256(const BYTE *ikm, DWORD ikmlen,
                    const BYTE *salt, DWORD saltlen,
                    const BYTE *info, DWORD infolen, BYTE out[32]);

/* AES-256-GCM encrypt. Output layout: [12B nonce][ciphertext][16B tag].
 * Returns total output length, or -1 on failure. */
int cng_aesgcm_encrypt(const BYTE key[32],
                       const BYTE *pt, DWORD ptlen,
                       const BYTE *aad, DWORD aadlen,
                       BYTE *out, DWORD outcap);

/* AES-256-GCM decrypt of [nonce][ct][tag]. Returns plaintext length, -1 on failure. */
int cng_aesgcm_decrypt(const BYTE key[32],
                       const BYTE *blob, DWORD bloblen,
                       const BYTE *aad, DWORD aadlen,
                       BYTE *out, DWORD outcap);

/* X25519 (software ladder + CNG RNG; works back to Windows 7).
 * Adopt an identity private key (fresh or persisted); runs the compiled-in
 * self-test first and fails closed on mismatch. */
int cng_x25519_use_priv(const BYTE priv[32], BYTE pub[32]);

/* X25519 agreement with a 32-byte peer public key. */
int cng_x25519_agree(const BYTE peer_pub[32], BYTE shared[32]);

/* Ephemeral handshake keypair (single use, wiped after agreement). */
int cng_x25519_ephemeral(BYTE pub[32]);
int cng_x25519_agree_eph(const BYTE peer_pub[32], BYTE shared[32]);

/* Wipe secrets from memory. */
void cng_wipe(void *p, DWORD len);

/* Minimal base64 (no external deps). b64enc returns malloc'd NUL-terminated
 * string (caller frees with free()); b64dec returns malloc'd buffer with
 * *outlen set, or NULL on invalid input. */
char *b64enc(const BYTE *in, DWORD len);
BYTE *b64dec(const char *in, DWORD *outlen);

#ifdef __cplusplus
}
#endif
