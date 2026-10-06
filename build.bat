@echo off
setlocal
rem Builds both release binaries into dist\. %~dp0 is this script's own
rem folder, so it always builds this repo regardless of the caller's
rem current directory.
cd /d "%~dp0"

if not exist dist mkdir dist

echo Building dist\WasteLess.exe (windows/amd64)...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w" -o dist\WasteLess.exe ./cmd/server
if errorlevel 1 exit /b 1

echo Building dist\wasteless-linux (linux/amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-s -w" -o dist\wasteless-linux ./cmd/server
if errorlevel 1 exit /b 1

echo Done. Binaries are in dist\
endlocal
