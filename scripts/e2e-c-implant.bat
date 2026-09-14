@echo off
REM ForgeC2 C-Implant E2E verification batch script
REM Usage: e2e-c-implant.bat [HOST] [PORT] [USER] [PASS] [AGENT_ID] [TASK_TYPE] [COMMAND_JSON]

setlocal enabledelayedexpansion

set HOST=%1
if "%HOST%"=="" set HOST=127.0.0.1

set PORT=%2
if "%PORT%"=="" set PORT=8000

set USER=%3
if "%USER%"=="" set USER=labtest

set PASS=%4
if "%PASS%"=="" set PASS=labtest

set AGENT_ID=%5
if "%AGENT_ID%"=="" (
    echo ERROR: AGENT_ID required
    exit /b 1
)

set TASK_TYPE=%6
if "%TASK_TYPE%"=="" set TASK_TYPE=shell

set CMD=%~7
if "%CMD%"=="" set CMD=%E2E_CMD%
if "%CMD%"=="" set CMD=echo CBEACON-E2E-VERIFIED

set BASE_URL=http://%HOST%:%PORT%
set COOKIE_JAR=%TEMP%\forgec2-e2e-cookies.txt

echo [E2E] Target: %BASE_URL%
echo [E2E] Agent: %AGENT_ID%
echo [E2E] Task Type: %TASK_TYPE%
echo [E2E] Command: %CMD%

REM Clean previous cookie jar
if exist "%COOKIE_JAR%" del /f /q "%COOKIE_JAR%"

REM 1. Login
echo [E2E] Logging in as %USER%...
curl.exe -s -c "%COOKIE_JAR%" -d "username=%USER%&password=%PASS%" "%BASE_URL%/api/login" >nul
if %ERRORLEVEL% neq 0 (
    echo [E2E] ERROR: Login failed
    exit /b 1
)
echo [E2E] OK: Login succeeded

REM 2. GET authenticated endpoint to trigger CSRF cookie rotation
echo [E2E] Triggering CSRF cookie rotation...
curl.exe -s -b "%COOKIE_JAR%" -c "%COOKIE_JAR%" -H "Accept: application/json" "%BASE_URL%/api/v1/tasks" >nul
if %ERRORLEVEL% neq 0 (
    echo [E2E] ERROR: GET /api/v1/tasks failed
    exit /b 1
)
echo [E2E] OK: CSRF cookie rotated

REM 3. Extract forgec2_csrf cookie from jar
set CSRF_TOKEN=
for /f "tokens=7 delims=	" %%A in ('type "%COOKIE_JAR%" ^| find "forgec2_csrf"') do (
    set CSRF_TOKEN=%%A
)
if "%CSRF_TOKEN%"=="" (
    echo [E2E] ERROR: forgec2_csrf cookie not found
    exit /b 1
)
echo [E2E] OK: CSRF token extracted (%CSRF_TOKEN:~0,16%...)

REM 4. Create task
echo [E2E] Creating %TASK_TYPE% task...
echo {"agent_id":"%AGENT_ID%","type":"%TASK_TYPE%","command":"%CMD%"} > "%TEMP%\e2e-task.json"
curl.exe -s -b "%COOKIE_JAR%" -c "%COOKIE_JAR%" -H "Content-Type: application/json" -H "Accept: application/json" -H "X-CSRF-Token: %CSRF_TOKEN%" -X POST --data-binary @"%TEMP%\e2e-task.json" "%BASE_URL%/api/v1/tasks" > "%TEMP%\e2e-task-response.json"
if %ERRORLEVEL% neq 0 (
    echo [E2E] ERROR: POST /api/v1/tasks failed
    type "%TEMP%\e2e-task-response.json"
    exit /b 1
)

REM Extract task ID using PowerShell (reliable JSON parsing)
for /f "delims=" %%A in ('powershell.exe -NoProfile -Command "(Get-Content '%TEMP%\e2e-task-response.json' -Raw | ConvertFrom-Json).data.id"') do set TASK_ID=%%A
if "%TASK_ID%"=="" (
    echo [E2E] ERROR: Failed to extract task ID
    type "%TEMP%\e2e-task-response.json"
    exit /b 1
)
echo [E2E] OK: Task created: ID=%TASK_ID%

REM 5. Poll for completion
echo [E2E] Polling task %TASK_ID% for completion (timeout 60s)...

:POLL_LOOP
curl.exe -s -b "%COOKIE_JAR%" -H "Accept: application/json" "%BASE_URL%/api/v1/tasks/%TASK_ID%" > "%TEMP%\e2e-poll.json"
if %ERRORLEVEL% neq 0 (
    echo [E2E] ERROR: Poll failed
    exit /b 1
)

REM Use PowerShell for reliable JSON parsing
for /f "delims=" %%A in ('powershell.exe -NoProfile -Command "(Get-Content '%TEMP%\e2e-poll.json' -Raw | ConvertFrom-Json).data.status"') do set STATUS=%%A
if "%STATUS%"=="" (
    echo [E2E] ERROR: Failed to parse status
    exit /b 1
)

if "%STATUS%"=="completed" (
    echo [E2E] OK: Task completed!
    REM Print result using PowerShell
    powershell.exe -NoProfile -Command "(Get-Content '%TEMP%\e2e-poll.json' -Raw | ConvertFrom-Json).data.result"
    goto :CLEANUP
)

if "%STATUS%"=="failed" (
    echo [E2E] ERROR: Task failed
    for /f "delims=" %%A in ('powershell.exe -NoProfile -Command "(Get-Content '%TEMP%\e2e-poll.json' -Raw | ConvertFrom-Json).data.error"') do set ERROR=%%A
    echo [E2E] Error: %ERROR%
    goto :CLEANUP
)

echo [E2E] Status: %STATUS% (waiting...)
timeout /t 3 /nobreak >nul
goto :POLL_LOOP

:CLEANUP
if exist "%TEMP%\e2e-task.json" del /f /q "%TEMP%\e2e-task.json"
if exist "%TEMP%\e2e-task-response.json" del /f /q "%TEMP%\e2e-task-response.json"
if exist "%TEMP%\e2e-poll.json" del /f /q "%TEMP%\e2e-poll.json"
if exist "%COOKIE_JAR%" del /f /q "%COOKIE_JAR%"
exit /b 0