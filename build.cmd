@echo off
setlocal EnableExtensions
cd /d "%~dp0" || exit /b 1

rem GitHub Actions sets CI=true/GITHUB_ACTIONS=true. Never pause in automation.
set "IS_CI=0"
if /I "%CI%"=="true" set "IS_CI=1"
if /I "%GITHUB_ACTIONS%"=="true" set "IS_CI=1"

where go >nul 2>nul
if errorlevel 1 (
  echo ERROR: Go 1.23 or newer was not found.
  echo Install it from https://go.dev/dl/ and retry.
  if "%IS_CI%"=="0" pause
  exit /b 1
)
where py >nul 2>nul
if errorlevel 1 (
  echo ERROR: Python 3 launcher was not found.
  echo Install Python 3 from https://www.python.org/downloads/ and enable the py launcher.
  if "%IS_CI%"=="0" pause
  exit /b 1
)

if not exist build mkdir build
set "GOOS=windows"
set "GOARCH=amd64"
set "CGO_ENABLED=0"

echo [1/2] Building SwiftSearch Plus for Windows x64...
go build -trimpath -ldflags="-H=windowsgui -s -w" -o "build\SwiftSearchPlus-unicon.exe" .
if errorlevel 1 goto :fail

echo [2/2] Embedding the project icon and DPI manifest...
py -3 "tools\embed_resources.py" --exe "build\SwiftSearchPlus-unicon.exe" --icon "assets\swiftsearch-plus.ico" --output "build\SwiftSearchPlus.exe"
if errorlevel 1 goto :fail

del "build\SwiftSearchPlus-unicon.exe" >nul 2>nul
if not exist "build\SwiftSearchPlus.exe" goto :fail
for %%F in ("build\SwiftSearchPlus.exe") do if %%~zF LSS 100000 goto :fail

echo.
echo BUILD SUCCESS: "%~dp0build\SwiftSearchPlus.exe"
echo NOTE: This is an experimental build; test it on Windows before relying on it.
if "%IS_CI%"=="1" goto :done
start "" "%~dp0build"
pause
:done
exit /b 0

:fail
echo.
echo BUILD FAILED. Review the error shown above.
if "%IS_CI%"=="0" pause
exit /b 1
