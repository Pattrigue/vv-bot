@echo off
setlocal

set "SERVICE=VVBot"
set "BOT_DIR=%~dp0VVBot"
set "LOG=%~dp0VVBot\vv-bot.log"

if /i "%~1"=="run" goto :run

net session >nul 2>&1
if errorlevel 1 (
    powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

if not exist "%BOT_DIR%" mkdir "%BOT_DIR%"

if not exist "%BOT_DIR%\.env" (
    (
        echo BOT_TOKEN=
        echo TRELLO_API_KEY=
        echo TRELLO_TOKEN=
        echo TRELLO_IN_PROGRESS_LIST_ID=
    ) > "%BOT_DIR%\.env"
    echo Fill in .env, save it and close Notepad to continue.
    notepad "%BOT_DIR%\.env"
)

sc query "%SERVICE%" >nul 2>&1
if errorlevel 1 nssm install "%SERVICE%" "%~f0" run

nssm set "%SERVICE%" Application "%~f0"
nssm set "%SERVICE%" AppParameters run
nssm set "%SERVICE%" AppDirectory "%BOT_DIR%"
nssm set "%SERVICE%" AppStdout "%LOG%"
nssm set "%SERVICE%" AppStderr "%LOG%"

echo Restarting %SERVICE%...
nssm stop "%SERVICE%" >nul 2>&1
nssm start "%SERVICE%"
timeout /t 10 /nobreak >nul
nssm status "%SERVICE%"
if exist "%LOG%" powershell -NoProfile -Command "$m = Select-String -Path '%LOG%' -Pattern 'Fetching latest build' | Select-Object -Last 1; if ($m) { (Get-Content '%LOG%' | Select-Object -Skip ($m.LineNumber - 1)) -replace '^Terminate batch job \(Y/N\)\? ', '' }"
pause
exit /b

:run
cd /d "%BOT_DIR%"

echo Fetching latest build...
set "ASSET_URL="
set "DOWNLOADED="
for /f "usebackq delims=" %%u in (`powershell -NoProfile -Command "[Net.ServicePointManager]::SecurityProtocol='Tls12'; (Invoke-RestMethod 'https://api.github.com/repos/Pattrigue/vv-bot/releases/latest').assets[0].url"`) do set "ASSET_URL=%%u"
if defined ASSET_URL curl -fsSL --retry 3 --retry-delay 5 -H "Accept: application/octet-stream" -o vv-bot.new.exe "%ASSET_URL%" && set "DOWNLOADED=1"
if not defined DOWNLOADED (
    echo Download failed. Keeping the current build.
    del /q vv-bot.new.exe 2>nul
    goto :launch
)

move /y vv-bot.new.exe vv-bot.exe >nul
if errorlevel 1 echo Could not replace vv-bot.exe.

:launch
vv-bot.exe
exit /b %errorlevel%
