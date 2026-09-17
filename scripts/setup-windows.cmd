@echo off
REM FlowSage - Windows environment bootstrap.
REM Creates the Python venv, installs tool dependencies, and prepares config.yaml.
REM See docs/dev/01-environment.md for details and troubleshooting.
setlocal
cd /d "%~dp0.."
set "ROOT=%CD%"

echo.
echo ==========================================
echo   FlowSage environment setup
echo   Root: %ROOT%
echo ==========================================
echo.

REM ---------- 1. Go ----------
where go >nul 2>nul
if errorlevel 1 (
  echo [X] Go not found in PATH. Install Go 1.25+ from https://go.dev/dl/
  exit /b 1
)
for /f "tokens=3" %%v in ('go version') do set "GOVER=%%v"
echo [OK] Go %GOVER%

REM ---------- 2. Python ----------
set "PY="
where python >nul 2>nul && set "PY=python"
if "%PY%"=="" (
  echo [X] python not found in PATH. Install Python 3.10+ from https://www.python.org/downloads/
  exit /b 1
)
for /f "tokens=2" %%v in ('%PY% --version') do set "PYVER=%%v"
echo [OK] Python %PYVER%

REM ---------- 3. C compiler - required by CGO for go-sqlite3 ----------
where gcc >nul 2>nul
if errorlevel 1 (
  echo [!] gcc not found. The Go build needs CGO for go-sqlite3.
  echo     Install MSYS2 or MinGW-w64 and add its bin directory to PATH.
  echo     Docs: docs/dev/01-environment.md
) else (
  echo [OK] gcc found
)

REM ---------- 4. virtualenv ----------
if not exist "%ROOT%\venv\Scripts\python.exe" (
  echo [*] Creating Python venv...
  %PY% -m venv "%ROOT%\venv"
  if errorlevel 1 (
    echo [X] venv creation failed
    exit /b 1
  )
) else (
  echo [OK] venv already exists
)

REM ---------- 5. Python dependencies ----------
echo [*] Installing Python dependencies via Tsinghua mirror...
"%ROOT%\venv\Scripts\python.exe" -m pip install --upgrade pip -q --index-url https://pypi.tuna.tsinghua.edu.cn/simple
"%ROOT%\venv\Scripts\python.exe" -m pip install -r "%ROOT%\requirements.txt" --index-url https://pypi.tuna.tsinghua.edu.cn/simple
if errorlevel 1 (
  echo [!] Some packages failed to install. Re-run this script - already-installed packages are skipped.
)

REM ---------- 6. python3 shim ----------
REM Windows python3 points at a broken Microsoft Store alias; a copy inside the
REM venv resolves to the venv interpreter because it reads pyvenv.cfg by location.
if not exist "%ROOT%\venv\Scripts\python3.exe" (
  copy /y "%ROOT%\venv\Scripts\python.exe" "%ROOT%\venv\Scripts\python3.exe" >nul
  echo [OK] python3.exe created in venv\Scripts
) else (
  echo [OK] python3.exe already present
)

REM ---------- 7. config ----------
if not exist "%ROOT%\config.yaml" (
  copy /y "%ROOT%\config.example.yaml" "%ROOT%\config.yaml" >nul
  echo [OK] config.yaml created - EDIT IT and fill in ai.channels.*.api_key
) else (
  echo [OK] config.yaml already exists
)

echo.
echo ------------------------------------------------------------
echo  Setup finished. Next steps:
echo    1. Edit config.yaml and set your AI base_url / api_key / model
echo    2. Run:  run-windows.cmd --http
echo    3. Open http://127.0.0.1:8080/
echo       The admin password is printed in the console on first start.
echo ------------------------------------------------------------
echo.
endlocal