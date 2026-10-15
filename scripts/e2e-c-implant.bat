@echo off
REM ForgeC2 C-Implant E2E verification batch script
REM Usage: e2e-c-implant.bat [HOST] [PORT] [USER] [PASS] [AGENT_ID] [TASK_TYPE] [COMMAND_JSON] [EXTRA_JSON_FIELDS]
REM
REM EXTRA_JSON_FIELDS (argv 8 or the E2E_EXTRA_JSON env var) is a raw JSON
REM fragment appended inside the task body, e.g. for a `find` that searches
REM one root:
REM   set E2E_EXTRA_JSON="path":"C:\\Windows\\Temp"
REM Values are JSON, so every backslash must be doubled: a single "\U" is not
REM a valid JSON escape and the server answers "invalid request". Prefer the
REM env var over argv 8, since cmd strips quotes from arguments.
REM
REM The task body is written with `echo`, so a COMMAND containing cmd
REM metacharacters (^ & | < > % " ) must be escaped for cmd or sent through
REM the REST API instead.

setlocal enabledelayedexpansion

set HOST=%1
if "%HOST%"=="" set HOST=127.0.0.1

set PORT=%2
if "%PORT%"=="" set PORT=8000

set USER=%3
if "%USER%"=="" set USER=labtest

REM No default password: PASS must come from argv %4 or E2E_PASS env.
REM A hardcoded default would bake a weak credential into CI logs and history.
set PASS=%4
if "%PASS%"=="" set PASS=%E2E_PASS%
if "%PASS%"=="" (
    echo ERROR: password required ^(argv %%4 or E2E_PASS env^)
    exit /b 1
)

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

set EXTRA=%~8
if "%EXTRA%"=="" set EXTRA=%E2E_EXTRA_JSON%
if not "%EXTRA%"=="" set EXTRA=,%EXTRA%

set BASE_URL=http://%HOST%:%PORT%
set COOKIE_JAR=%TEMP%\forgec2-e2e-cookies.txt

echo [E2E] Target: %BASE_URL%
echo [E2E] Agent: %AGENT_ID%
echo [E2E] Task Type: %TASK_TYPE%
echo [E2E] Command: %CMD%
if not "%EXTRA%"=="" echo [E2E] Extra fields: %EXTRA%

REM Clean previous cookie jar
if exist "%COOKIE_JAR%" del /f /q "%COOKIE_JAR%"

REM 1. Login. Inspect the status code as well as curl's exit code: curl exits 0
REM on HTTP 401 (and 429), which used to surface only as a confusing
REM "forgec2_csrf cookie not found" several steps later.
echo [E2E] Logging in as %USER%...
for /f "delims=" %%A in ('curl.exe -s -o nul -w "%%{http_code}" -c "%COOKIE_JAR%" -d "username=%USER%&password=%PASS%" "%BASE_URL%/api/login"') do set LOGIN_CODE=%%A
if not "%LOGIN_CODE%"=="302" if not "%LOGIN_CODE%"=="200" (
    echo [E2E] ERROR: Login failed with HTTP %LOGIN_CODE% - check the credentials
    if exist "%COOKIE_JAR%" del /f /q "%COOKIE_JAR%"
    exit /b 1
)
echo [E2E] OK: Login succeeded

REM 2. GET authenticated endpoint to trigger CSRF cookie rotation
echo [E2E] Triggering CSRF cookie rotation...
curl.exe -s -b "%COOKIE_JAR%" -c "%COOKIE_JAR%" -H "Accept: application/json" "%BASE_URL%/api/v1/tasks" >nul
if %ERRORLEVEL% neq 0 (
    echo [E2E] ERROR: GET /api/v1/tasks failed
    if exist "%COOKIE_JAR%" del /f /q "%COOKIE_JAR%"
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
echo {"agent_id":"%AGENT_ID%","type":"%TASK_TYPE%","command":"%CMD%"%EXTRA%} > "%TEMP%\e2e-task.json"
curl.exe -s -b "%COOKIE_JAR%" -c "%COOKIE_JAR%" -H "Content-Type: application/json" -H "Accept: application/json" -H "X-CSRF-Token: %CSRF_TOKEN%" -X POST --data-binary @"%TEMP%\e2e-task.json" "%BASE_URL%/api/v1/tasks" > "%TEMP%\e2e-task-response.json"
if %ERRORLEVEL% neq 0 (
    echo [E2E] ERROR: POST /api/v1/tasks failed
    type "%TEMP%\e2e-task-response.json"
    exit /b 1
)

REM Extract task ID using PowerShell (reliable JSON parsing)
for /f "delims=" %%A in ('powershell.exe -NoProfile -Command "(Get-Content '%TEMP%\e2e-task-response.json' -Raw -Encoding UTF8 | ConvertFrom-Json).data.id"') do set TASK_ID=%%A
if "%TASK_ID%"=="" (
    echo [E2E] ERROR: Failed to extract task ID
    type "%TEMP%\e2e-task-response.json"
    exit /b 1
)
echo [E2E] OK: Task created: ID=%TASK_ID%

REM 5. Poll for completion
echo [E2E] Polling task %TASK_ID% for completion (timeout 60s)...
set POLL_COUNT=0
set MAX_POLL=20

:POLL_LOOP
set /a POLL_COUNT+=1
REM No parentheses in this echo: an unquoted ")" inside an if-block closes it
REM early and the goto below would run on the first iteration.
if !POLL_COUNT! GTR %MAX_POLL% (
    echo [E2E] ERROR: Poll timeout after %MAX_POLL% attempts
    goto :CLEANUP_FAIL
)
curl.exe -s -b "%COOKIE_JAR%" -H "Accept: application/json" "%BASE_URL%/api/v1/tasks/%TASK_ID%" > "%TEMP%\e2e-poll.json"
if %ERRORLEVEL% neq 0 (
    echo [E2E] ERROR: Poll failed
    exit /b 1
)

REM Use PowerShell for reliable JSON parsing (-Encoding UTF8: results may
REM carry non-ASCII text, and ANSI decoding would break ConvertFrom-Json)
for /f "delims=" %%A in ('powershell.exe -NoProfile -Command "(Get-Content '%TEMP%\e2e-poll.json' -Raw -Encoding UTF8 | ConvertFrom-Json).data.status"') do set STATUS=%%A
if "%STATUS%"=="" (
    echo [E2E] ERROR: Failed to parse status
    type "%TEMP%\e2e-poll.json"
    exit /b 1
)

if "%STATUS%"=="completed" (
    echo [E2E] OK: Task completed!
    goto :SHOW_RESULT
)

if "%STATUS%"=="failed" (
    echo [E2E] ERROR: Task failed
    goto :SHOW_ERROR
)

echo [E2E] Status: %STATUS% - waiting...
REM ping, not `timeout`: timeout refuses to run when stdin is redirected, and
REM this script is meant to be driven from other scripts and CI.
ping -n 4 127.0.0.1 >nul 2>&1
goto :POLL_LOOP

:SHOW_RESULT
powershell.exe -NoProfile -Command "(Get-Content '%TEMP%\e2e-poll.json' -Raw -Encoding UTF8 | ConvertFrom-Json).data.result"
goto :CLEANUP

:SHOW_ERROR
for /f "delims=" %%A in ('powershell.exe -NoProfile -Command "(Get-Content '%TEMP%\e2e-poll.json' -Raw -Encoding UTF8 | ConvertFrom-Json).data.error"') do set TASK_ERROR=%%A
echo [E2E] Error: %TASK_ERROR%
goto :CLEANUP_FAIL

:CLEANUP
if exist "%TEMP%\e2e-task.json" del /f /q "%TEMP%\e2e-task.json"
if exist "%TEMP%\e2e-task-response.json" del /f /q "%TEMP%\e2e-task-response.json"
if exist "%TEMP%\e2e-poll.json" del /f /q "%TEMP%\e2e-poll.json"
if exist "%COOKIE_JAR%" del /f /q "%COOKIE_JAR%"
exit /b 0

:CLEANUP_FAIL
if exist "%TEMP%\e2e-task.json" del /f /q "%TEMP%\e2e-task.json"
if exist "%TEMP%\e2e-task-response.json" del /f /q "%TEMP%\e2e-task-response.json"
if exist "%TEMP%\e2e-poll.json" del /f /q "%TEMP%\e2e-poll.json"
if exist "%COOKIE_JAR%" del /f /q "%COOKIE_JAR%"
exit /b 1