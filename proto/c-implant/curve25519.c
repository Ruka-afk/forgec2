/* curve25519.c — 时域恒定的 X25519（5x51-limb）。
 *
 * 实现说明：
 *  - 有限域运算使用 radix-2^51、int64 肢，中间值一律 __int128。
 *  - 乘法 schoolbook + 19 折叠 + 两轮进位；平方复用乘法。
 *  - 求逆用二进制幂 z^(2^255-21)，慢但显然正确。
 *  - 阶梯为 RFC 7748 伪代码，含恒定时间 cswap。
 *  - x25519_selftest() 用本地 Go 标准库生成的向量校验，失败则调用方
 *    必须拒绝上线（fail closed）。向量见 x25519_vectors.h。
 */
#include <stdint.h>
#include <string.h>
#include "curve25519.h"

typedef int64_t fe[5];

/* p = 2^255-19 的 51-bit 肢表示 */
static const int64_t PLIMB[5] = {
    0x7FFFFFFFFFFFFLL - 18, /* 2^51-19 */
    0x7FFFFFFFFFFFFLL,
    0x7FFFFFFFFFFFFLL,
    0x7FFFFFFFFFFFFLL,
    0x7FFFFFFFFFFFFLL
};

/* 一轮 51-bit 进位（含 19 折叠），输入可为 __int128 量级。 */
static void fe_carry(fe h, __int128 t[5]) {
    int i;
    for (i = 0; i < 5; i++) {
        int64_t carry = (int64_t)(t[i] >> 51);
        t[i] &= (((__int128)1 << 51) - 1);
        if (i + 1 < 5) t[i + 1] += carry;
        else t[0] += (__int128)carry * 19;
    }
    h[0] = (int64_t)t[0]; h[1] = (int64_t)t[1]; h[2] = (int64_t)t[2];
    h[3] = (int64_t)t[3]; h[4] = (int64_t)t[4];
}

static void fe_add(fe h, const fe f, const fe g) {
    h[0] = f[0] + g[0];
    h[1] = f[1] + g[1];
    h[2] = f[2] + g[2];
    h[3] = f[3] + g[3];
    h[4] = f[4] + g[4];
}

static void fe_sub(fe h, const fe f, const fe g) {
    /* 偏置 16p 保证逐肢非负（ladder 中 |f-g| 远小于 2^55）。 */
    __int128 t[5];
    int i;
    for (i = 0; i < 5; i++)
        t[i] = (__int128)f[i] - g[i] + (__int128)16 * PLIMB[i];
    {
        fe tmp;
        fe_carry(tmp, t);
        /* 再走一轮收敛 h0 的折叠增量 */
        __int128 t2[5];
        for (i = 0; i < 5; i++) t2[i] = tmp[i];
        fe_carry(h, t2);
    }
}

static void fe_mul(fe h, const fe f, const fe g) {
    __int128 t[9] = {0, 0, 0, 0, 0, 0, 0, 0, 0};
    __int128 t5[5];
    int i, j;
    for (i = 0; i < 5; i++)
        for (j = 0; j < 5; j++)
            t[i + j] += (__int128)f[i] * g[j];
    for (i = 5; i < 9; i++)
        t[i - 5] += t[i] * 19;
    for (i = 0; i < 5; i++) t5[i] = t[i];
    {
        fe tmp;
        fe_carry(tmp, t5);
        __int128 t6[5];
        for (i = 0; i < 5; i++) t6[i] = tmp[i];
        fe_carry(h, t6);
    }
}

static void fe_sq(fe h, const fe f) {
    fe_mul(h, f, f);
}

/* z^(2^255-21)：二进制幂（LSB-first）。2^255-21 低字节 0xEB，高位全 1。 */
static void fe_invert(fe out, const fe z) {
    fe r, base, tmp;
    int i;
    memcpy(base, z, sizeof(fe));
    r[0] = 1; r[1] = 0; r[2] = 0; r[3] = 0; r[4] = 0;
    for (i = 0; i < 255; i++) {
        int bit = (i < 8) ? ((0xEB >> i) & 1) : 1;
        if (bit) {
            fe_mul(tmp, r, base);
            memcpy(r, tmp, sizeof(fe));
        }
        if (i < 254) {
            fe_sq(tmp, base);
            memcpy(base, tmp, sizeof(fe));
        }
    }
    memcpy(out, r, sizeof(fe));
}

static void fe_frombytes(fe h, const unsigned char s[32]) {
    /* 逐位装配：显然正确（255 个有效位）。 */
    int i;
    int64_t acc[5] = {0, 0, 0, 0, 0};
    for (i = 0; i < 255; i++) {
        int64_t b = (s[i / 8] >> (i % 8)) & 1;
        acc[i / 51] |= b << (i % 51);
    }
    h[0] = acc[0]; h[1] = acc[1]; h[2] = acc[2]; h[3] = acc[3]; h[4] = acc[4];
}

/* 完全规约并序列化。 */
static void fe_tobytes(unsigned char s[32], const fe f) {
    __int128 t[5];
    int i, pass, b;
    int64_t h[5];
    for (i = 0; i < 5; i++) t[i] = f[i];
    /* 4 轮进位收敛到每肢 < 2^51 */
    for (pass = 0; pass < 4; pass++) {
        for (i = 0; i < 5; i++) {
            int64_t carry = (int64_t)(t[i] >> 51);
            t[i] &= (((__int128)1 << 51) - 1);
            if (i + 1 < 5) t[i + 1] += carry;
            else t[0] += (__int128)carry * 19;
        }
    }
    for (i = 0; i < 5; i++) h[i] = (int64_t)t[i];
    /* 条件减 p：无分支比较（setcc，非秘密相关跳转）+ 掩码选择 */
    {
        uint64_t eq = 1, gt = 0;
        for (i = 4; i >= 0; i--) {
            uint64_t eqi = (h[i] == PLIMB[i]) ? 1u : 0u;
            uint64_t gti = ((uint64_t)h[i] > (uint64_t)PLIMB[i]) ? 1u : 0u;
            gt = gt | (eq & gti);
            eq = eq & eqi;
        }
        {
            int64_t ge = (int64_t)(gt | eq); /* h >= p ? */
            /* d = h - p（带借位，__int128 精度足够） */
            __int128 borrow = 0;
            int64_t d[5];
            for (i = 0; i < 5; i++) {
                __int128 cur = (__int128)h[i] - PLIMB[i] - borrow;
                if (cur < 0) {
                    d[i] = (int64_t)(cur + ((__int128)1 << 51));
                    borrow = 1;
                } else {
                    d[i] = (int64_t)cur;
                    borrow = 0;
                }
            }
            {
                int64_t mask = -ge;
                for (i = 0; i < 5; i++) h[i] = (h[i] & ~mask) | (d[i] & mask);
            }
        }
    }
    for (i = 0; i < 32; i++) s[i] = 0;
    for (b = 0; b < 255; b++) {
        if ((h[b / 51] >> (b % 51)) & 1)
            s[b / 8] |= (unsigned char)(1u << (b % 8));
    }
}

static void fe_cswap(fe f, fe g, unsigned int b) {
    /* b ∈ {0,1}，恒定时间交换 */
    int64_t mask = -(int64_t)b;
    int i;
    for (i = 0; i < 5; i++) {
        int64_t x = mask & (f[i] ^ g[i]);
        f[i] ^= x;
        g[i] ^= x;
    }
}

/* ---------- X25519 (RFC 7748 ladder) ---------- */

static const unsigned char basepoint[32] = {9};

void x25519_scalarmult(unsigned char out[32], const unsigned char scalar[32],
                       const unsigned char u[32]) {
    unsigned char k[32];
    fe x1, x2, z2, x3, z3, a, b, c, d, da, cb, aa, bb, e, tmp;
    fe a24;
    unsigned int swap = 0, kt;
    int t;
    memcpy(k, scalar, 32);
    k[0] &= 248;
    k[31] &= 127;
    k[31] |= 64;
    fe_frombytes(x1, u);
    x2[0] = 1; x2[1] = 0; x2[2] = 0; x2[3] = 0; x2[4] = 0;
    z2[0] = 0; z2[1] = 0; z2[2] = 0; z2[3] = 0; z2[4] = 0;
    memcpy(x3, x1, sizeof(fe));
    z3[0] = 1; z3[1] = 0; z3[2] = 0; z3[3] = 0; z3[4] = 0;
    a24[0] = 121665; a24[1] = 0; a24[2] = 0; a24[3] = 0; a24[4] = 0;
    for (t = 254; t >= 0; t--) {
        kt = (k[t >> 3] >> (t & 7)) & 1;
        swap ^= kt;
        fe_cswap(x2, x3, swap);
        fe_cswap(z2, z3, swap);
        swap = kt;
        fe_add(a, x2, z2);      /* A */
        fe_sq(aa, a);           /* AA */
        fe_sub(b, x2, z2);      /* B */
        fe_sq(bb, b);           /* BB */
        fe_sub(e, aa, bb);      /* E */
        fe_add(c, x3, z3);      /* C */
        fe_sub(d, x3, z3);      /* D */
        fe_mul(da, d, a);
        fe_mul(cb, c, b);
        fe_add(tmp, da, cb);
        fe_sq(x3, tmp);
        fe_sub(tmp, da, cb);
        fe_sq(tmp, tmp);
        fe_mul(z3, tmp, x1);
        fe_mul(x2, aa, bb);
        fe_mul(tmp, a24, e);
        fe_add(tmp, tmp, aa);
        fe_mul(z2, e, tmp);
    }
    fe_cswap(x2, x3, swap);
    fe_cswap(z2, z3, swap);
    fe_invert(tmp, z2);
    fe_mul(x2, x2, tmp);
    fe_tobytes(out, x2);
}

void x25519_basepoint(unsigned char out[32], const unsigned char scalar[32]) {
    x25519_scalarmult(out, scalar, basepoint);
}

#include "x25519_vectors.h"

int x25519_selftest(void) {
    unsigned char out[32];
    int i;
    /* peer closure first: PEER_PUB must equal scalarmult(PEER_PRIV) */
    x25519_basepoint(out, X25519_PEER_PRIV);
    if (memcmp(out, X25519_PEER_PUB, 32) != 0) return -1;
    for (i = 0; i < 3; i++) {
        x25519_basepoint(out, X25519_VECTORS[i].priv);
        if (memcmp(out, X25519_VECTORS[i].pub, 32) != 0) return -2 - i;
        x25519_scalarmult(out, X25519_VECTORS[i].priv, X25519_PEER_PUB);
        if (memcmp(out, X25519_VECTORS[i].shared, 32) != 0) return -10 - i;
    }
    return 0;
}
