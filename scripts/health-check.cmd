@echo off
setlocal

if "%~1"=="" (
  set "BASE_URL=http://127.0.0.1:8317"
) else (
  set "BASE_URL=%~1"
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0health-check.ps1" -BaseUrl "%BASE_URL%"
exit /b %errorlevel%
