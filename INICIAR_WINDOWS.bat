@echo off
cd /d "%~dp0"
if not exist .env copy .env.example .env >nul
if not exist data mkdir data
if exist worktic-omnichannel-v3.exe goto run

echo Compilando Worktic Omnichannel AI V3...
go build -mod=mod -o worktic-omnichannel-v3.exe .
if errorlevel 1 (
 echo.
 echo No se pudo compilar. Ejecuta primero INSTALAR_DEPENDENCIAS_WINDOWS.bat
 pause
 exit /b 1
)
:run
start "" http://localhost:8080
worktic-omnichannel-v3.exe
pause
