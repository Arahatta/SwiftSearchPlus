@echo off
setlocal
cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
  echo Go 1.23 or newer was not found.
  echo Install it from https://go.dev/dl/ and retry.
  pause
  exit /b 1
)
where py >nul 2>nul
if errorlevel 1 (
  echo Python 3 launcher was not found.
  echo Install Python 3 from https://www.python.org/downloads/ and enable the py launcher.
  pause
  exit /b 1
)

if not exist build mkdir build
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

echo [1/2] Building SwiftSearch Plus for Windows x64...
go build -trimpath -ldflags="-H=windowsgui -s -w" -o "build\SwiftSearchPlus-unicon.exe" .
if errorlevel 1 goto :fail

echo [2/2] Embedding the project icon and DPI manifest...
py -3 "tools\embed_resources.py" --exe "build\SwiftSearchPlus-unicon.exe" --icon "assets\swiftsearch-plus.ico" --output "build\SwiftSearchPlus.exe"
if errorlevel 1 goto :fail

del "build\SwiftSearchPlus-unicon.exe" >nul 2>nul
echo.
echo Build completed: "%~dp0build\SwiftSearchPlus.exe"
echo NOTE: This is an experimental build; test it on Windows before relying on it.
start "" "%~dp0build"
pause
exit /b 0

:fail
echo.
echo Build failed. Review the error shown above.
pause
exit /b 1
