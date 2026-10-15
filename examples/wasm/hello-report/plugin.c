// ForgeC2 WASM plugin example — ABI v1 (see docs/PLUGIN_TIERS.md).
//
// A `report` plugin that reads `params.label` out of the input JSON the host
// wrote and answers with a markdown report echoing that label. It is
// freestanding C: no libc, no WASI, no imports except the optional host log.
//
// Build it with ./build.sh (any wasm32-capable clang; the module does not link
// against libc, so no sysroot is needed).

typedef unsigned int u32;
typedef unsigned char u8;

// ── the whole libc this module needs ────────────────────────────────────────
// clang lowers copy loops and struct assignments to memcpy/memset even under
// -nostdlib; wasm-ld then fails with "undefined symbol: memcpy" unless the
// module defines them. They run downwards so the compiler cannot turn them
// back into calls to themselves.
__attribute__((used)) void *memcpy(void *dst, const void *src, unsigned long n) {
    u8 *d = (u8 *)dst;
    const u8 *s = (const u8 *)src;
    unsigned long i = n;
    while (i > 0) {
        i--;
        d[i] = s[i];
    }
    return dst;
}

__attribute__((used)) void *memset(void *dst, int c, unsigned long n) {
    u8 *d = (u8 *)dst;
    unsigned long i = n;
    while (i > 0) {
        i--;
        d[i] = (u8)c;
    }
    return dst;
}

// ── optional host import ────────────────────────────────────────────────────
// Diagnostics land in the plugin's stderr, capped by the host at 256 KiB.
__attribute__((import_module("forgec2"), import_name("log")))
extern void host_log(const char *ptr, u32 len);

// ── module-private state ────────────────────────────────────────────────────
// This is the part the host must never scribble on: everything here lives in
// the data segment its linker placed (at 1024 for wasm-ld's default layout).
// The host therefore writes the input *past* the module's initial memory.
#define REPLY_MAX 4096
#define SCRATCH_MAX 256
#define VALUE_MAX 128

static u8 reply[REPLY_MAX];
static u8 scratch[SCRATCH_MAX];
static u32 reply_len;

static void push(const u8 *s, u32 n) {
    for (u32 i = 0; i < n && reply_len < REPLY_MAX; i++) reply[reply_len++] = s[i];
}

static void push_str(const char *s) {
    u32 n = 0;
    while (s[n]) n++;
    push((const u8 *)s, n);
}

// push_json_string writes s as the *inside* of a JSON string literal.
static void push_json_string(const u8 *s, u32 n) {
    for (u32 i = 0; i < n && reply_len < REPLY_MAX; i++) {
        u8 c = s[i];
        if (c == '"' || c == '\\') {
            push((const u8 *)"\\", 1);
            push(&c, 1);
        } else if (c < 0x20) {
            push((const u8 *)"_", 1);
        } else {
            push(&c, 1);
        }
    }
}

static const char b64[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

static void push_base64(const u8 *src, u32 n) {
    for (u32 i = 0; i < n; i += 3) {
        u32 rem = n - i;
        u32 v = (u32)src[i] << 16;
        if (rem > 1) v |= (u32)src[i + 1] << 8;
        if (rem > 2) v |= (u32)src[i + 2];
        u8 out[4];
        out[0] = (u8)b64[(v >> 18) & 63];
        out[1] = (u8)b64[(v >> 12) & 63];
        out[2] = rem > 1 ? (u8)b64[(v >> 6) & 63] : (u8)'=';
        out[3] = rem > 2 ? (u8)b64[v & 63] : (u8)'=';
        push(out, 4);
    }
}

// find_string locates `"key"` (a JSON string value, no escapes) in the input
// and returns its length, or -1. *start receives the value offset. A real
// plugin would use a JSON parser; the example stays dependency-free.
static int find_string(const u8 *in, u32 len, const char *key, u32 *start) {
    u32 klen = 0;
    while (key[klen]) klen++;
    for (u32 i = 0; i + klen + 4 <= len; i++) {
        if (in[i] != '"') continue;
        u32 j = 0;
        while (j < klen && in[i + 1 + j] == (u8)key[j]) j++;
        if (j != klen || in[i + 1 + j] != '"') continue;
        u32 p = i + klen + 2;
        while (p < len && (in[p] == ' ' || in[p] == '\t')) p++;
        if (p >= len || in[p] != ':') continue;
        p++;
        while (p < len && (in[p] == ' ' || in[p] == '\t')) p++;
        if (p >= len || in[p] != '"') continue;
        p++;
        u32 s = p;
        while (p < len && in[p] != '"') p++;
        if (p >= len) return -1;
        *start = s;
        u32 n = p - s;
        if (n > VALUE_MAX) n = VALUE_MAX;
        return (int)n;
    }
    return -1;
}

// ── ABI v1 entry point ──────────────────────────────────────────────────────
// One i64 result: low 32 bits are the reply pointer, high 32 bits its length.
// Multi-value *results* are deliberately avoided — clang lowers a two-field
// struct return to an indirect (sret) parameter, so a `(result i32 i32)` export
// would not be reachable from ordinary C/Rust code.
typedef unsigned long long u64;

__attribute__((export_name("run")))
u64 run(const u8 *in, u32 len) {
    u32 val_start = 0;
    int val_len = find_string(in, len, "label", &val_start);
    if (val_len < 0) {
        val_start = 0;
        val_len = 0;
    }
    const u8 *val = in + val_start;

    // Markdown body: the report content (JSON carries it base64-encoded
    // because Report.Content is raw bytes).
    u32 body_len = 0;
    const char *head = "# wasm-hello\n\nBuilt by `examples/wasm/hello-report` (ABI v1, freestanding C).\n\nlabel: ";
    for (u32 i = 0; head[i] && body_len < SCRATCH_MAX; i++) scratch[body_len++] = (u8)head[i];
    for (u32 i = 0; i < (u32)val_len && body_len < SCRATCH_MAX; i++) scratch[body_len++] = val[i];
    if (body_len < SCRATCH_MAX) scratch[body_len++] = '\n';

    reply_len = 0;
    push_str("{\"title\":\"wasm-hello: ");
    push_json_string(val, (u32)val_len);
    push_str("\",\"format\":\"markdown\",\"content\":\"");
    push_base64(scratch, body_len);
    push_str("\"}");

    if (val_len > 0) host_log((const char *)val, (u32)val_len);

    return ((u64)reply_len << 32) | (u64)(unsigned long)reply;
}
