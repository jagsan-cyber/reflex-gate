@echo off
setlocal EnableExtensions
cd /d "%~dp0"

if not exist ".venv\Scripts\python.exe" (
    echo [ERROR] Missing venv. Run: python -m venv .venv
    echo         then: .venv\Scripts\pip install -r requirements.txt
    pause
    exit /b 1
)

if "%JEV_LLM_BASE_URL%"=="" set "JEV_LLM_BASE_URL=http://127.0.0.1:8080/v1"
if "%JEV_HOST%"=="" set "JEV_HOST=0.0.0.0"
if "%JEV_PORT%"=="" set "JEV_PORT=8090"

echo JEV API  http://127.0.0.1:%JEV_PORT%/demo
echo LLM      %JEV_LLM_BASE_URL%
echo Start llama-server first with --parallel 2 (slot 0=decision, slot 1=extract).
echo.

".venv\Scripts\python.exe" jev_api.py
exit /b %ERRORLEVEL%
