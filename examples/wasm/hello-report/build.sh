#!/usr/bin/env sh
# Build plugin.wasm for the ForgeC2 WASM tier (ABI v1).
#
# Any clang with a wasm32 backend works, with or without a sysroot: the module
# is freestanding (-nostdlib) and declares its own host import. plugin.c also
# defines the two libc calls (memcpy/memset) clang lowers copy loops to, so no
# sysroot or compiler-rt is needed.
#
#   ./build.sh                              # clang from PATH
#   CC="zig cc" TARGET=wasm32-freestanding sh build.sh
#   CC=/opt/wasi-sdk/bin/clang sh build.sh  # wasi-sdk also works
set -eu

CC="${CC:-clang}"
TARGET="${TARGET:-wasm32-unknown-unknown}"

# CC may be a wrapper such as "zig cc", so it is deliberately unquoted.
# shellcheck disable=SC2086
$CC \
    --target="$TARGET" \
    -Os -nostdlib -ffreestanding \
    -Wl,--no-entry \
    -Wl,--export=run \
    -Wl,--export-memory \
    -Wl,--strip-all \
    -Wl,-z,stack-size=65536 \
    -o plugin.wasm \
    plugin.c

echo "built plugin.wasm:"
ls -l plugin.wasm
