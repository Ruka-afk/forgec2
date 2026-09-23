@echo off
REM build-common.cmd - shared cross-toolchain guard for the C implant build scripts.
REM
REM Usage (from within proto/c-implant/):
REM   call build-common.cmd check
REM Exits 1 (via the caller errorlevel) if the mingw-w64 cross compiler is
REM missing, so every build script fails loudly instead of with an obscure
REM link error or a silently broken binary.
REM
REM Deliberately SMALL: it only validates the toolchain. It does NOT run the
REM compile. Each build script keeps its own single physical compile line, and
REM that must stay a single line - a multi-line ^ / %%var continuation has
REM silently produced broken binaries (missing task strings) in some
REM environments (see the history note in build.bat). Do not move the compile
REM into this file.
REM
REM Canonical reference (for updating in one place):
REM   compiler : x86_64-w64-mingw32-gcc
REM   sources  : beacon.c crypto_cng.c curve25519.c sqlite3.c (amalgamation)
REM              evade.c dns_transport.c
REM   libs     : -lwinhttp -lbcrypt -lpsapi -liphlpapi -lgdiplus -lgdi32
REM              -luser32 -lole32 -ldnsapi -lws2_32
REM   (build-e2e.bat instead links the prebuilt -lsqlite3 and omits sqlite3.c)
REM   DNS transport opt-in (build.bat / build-release.bat): C2_TRANSPORT=dns
REM   + C2_DNS_DOMAIN=... expands to -DC2_TRANSPORT_DNS -DC2_DNS_DOMAIN=...
REM
REM NOTE: never put a raw close-paren inside a multi-line if (...) block;
REM it terminates the block early and yields ". was unexpected at this time."

if /i "%~1"=="check" goto do_check
exit /b 0

:do_check
x86_64-w64-mingw32-gcc --version >nul 2>&1
if errorlevel 1 (
  echo ERROR: x86_64-w64-mingw32-gcc not found.
  echo Install the mingw-w64 cross toolchain: GCC for Windows, x86_64 target.
  exit /b 1
)
exit /b 0
