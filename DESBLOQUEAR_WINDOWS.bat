@echo off
setlocal
cd /d "%~dp0"
echo Desbloqueando archivos de Worktic descargados de Internet...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-ChildItem -LiteralPath '%~dp0' -Recurse -File | Unblock-File"
if errorlevel 1 (
  echo No fue posible desbloquear todos los archivos automaticamente.
  echo Haz clic derecho sobre el ZIP original, abre Propiedades, marca Desbloquear y extraelo otra vez.
) else (
  echo Archivos desbloqueados correctamente.
)
pause
