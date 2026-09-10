#pragma once

/* X25519 (RFC 7748)，donna 风格 5x51-limb 实现，恒定时间 ladder。
 * 随 CNG（AES-GCM/HMAC/RNG）配合使用；在 Win7 等无 CNG-X25519 的
 * 主机上同样可用。正确性由 x25519_selftest() + 编译进的 Go 标准库
 * 生成向量保证，失败则调用方必须拒绝上线。 */

#ifdef __cplusplus
extern "C" {
#endif

/* out = scalar * u（clamping 内置）。u 不必约化。 */
void x25519_scalarmult(unsigned char out[32], const unsigned char scalar[32],
                       const unsigned char u[32]);

/* out = scalar * basepoint。 */
void x25519_basepoint(unsigned char out[32], const unsigned char scalar[32]);

/* 内嵌向量自检：0=通过。 */
int x25519_selftest(void);

#ifdef __cplusplus
}
#endif
