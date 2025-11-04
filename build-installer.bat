@echo off
echo Docker for Android Installer Builder
echo ====================================
echo.
echo This script builds the installer binaries for ARM64 and x86_64 architectures.
echo Note: This requires Go to be installed and available in PATH.
echo.

REM Check if Go is available
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo ERROR: Go is not installed or not in PATH
    echo Please install Go from https://golang.org/dl/
    exit /b 1
)

echo Go version:
go version
echo.

REM Create release directory
if not exist "release" mkdir release

echo Building ARM64 installer...
cd installer
call CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ..\release\install-docker-arm64 .
if %errorlevel% neq 0 (
    echo ERROR: ARM64 build failed
    exit /b 1
)
cd ..

echo Building x86_64 installer...
cd installer
call CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ..\release\install-docker-x86_64 .
if %errorlevel% neq 0 (
    echo ERROR: x86_64 build failed
    exit /b 1
)
cd ..

echo.
echo Build completed successfully!
echo Binaries created in release/:
dir release\*.exe 2>nul || dir release\install-docker-*
echo.
echo SHA256 checksums:
certutil -hashfile release\install-docker-arm64 SHA256
certutil -hashfile release\install-docker-x86_64 SHA256

pause