@echo off
setlocal
if exist "%~dp0run.exe" (
    start "" "%~dp0run.exe"
    exit /b 0
)
echo run.exe not found.
echo Build it first or ask me to generate it.
pause
