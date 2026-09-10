@echo off
setlocal
cd /d "%~dp0"

where go >nul 2>&1
if errorlevel 1 (
    echo Go 1.22 or newer is required: https://go.dev/dl/
    pause
    exit /b 1
)

echo Starting LandWalker at http://localhost:3000
go run .
if errorlevel 1 pause
