@echo off
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0start.ps1"
if errorlevel 1 (
  echo.
  echo 启动失败，请查看上方错误信息。
  pause
)
endlocal
