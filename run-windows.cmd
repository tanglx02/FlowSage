@echo off
REM CyberStrikeAI Windows launcher.
REM Adds venv (python3) and Git usr\bin (sh) to PATH so Linux-style tool recipes resolve.
setlocal
cd /d "%~dp0"

set "PATH=%~dp0venv\Scripts;%PATH%"
if exist "%ProgramFiles%\Git\usr\bin" set "PATH=%ProgramFiles%\Git\usr\bin;%PATH%"

if not exist "cyberstrike-ai.exe" (
  echo [*] cyberstrike-ai.exe not found, building...
  go build -o cyberstrike-ai.exe ./cmd/server || exit /b 1
)

echo [*] Starting CyberStrikeAI (config.yaml)...
echo [*] Plain HTTP: run "run-windows.cmd --http"
cyberstrike-ai.exe -config config.yaml %*