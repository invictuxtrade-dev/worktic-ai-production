@echo off
setlocal
cd /d "%~dp0"
title Worktic AI V13.0.4

if not exist ".env" copy ".env.example" ".env" >nul
if not exist "data" mkdir "data"

set "APP_EXE=%~dp0worktic-ai-v13.exe"

if not exist "%APP_EXE%" (
  echo Compilando Worktic AI V13.0.4...
  where go >nul 2>nul
  if errorlevel 1 (
    echo.
    echo ERROR: Go no esta instalado o no esta disponible en PATH.
    echo Instala Go 1.24 o superior y ejecuta INSTALAR_DEPENDENCIAS_WINDOWS.bat.
    pause
    exit /b 1
  )
  go build -mod=mod -o "%APP_EXE%" .
  if errorlevel 1 (
    echo.
    echo ERROR: No se pudo compilar la aplicacion.
    echo Ejecuta INSTALAR_DEPENDENCIAS_WINDOWS.bat y revisa el mensaje mostrado.
    pause
    exit /b 1
  )
)

echo Iniciando Worktic AI en http://localhost:8080 ...
start "" /b powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Sleep -Seconds 2; Start-Process 'http://localhost:8080'" >nul 2>nul
"%APP_EXE%"
set "EXIT_CODE=%ERRORLEVEL%"

echo.
if not "%EXIT_CODE%"=="0" echo Worktic se cerro con codigo %EXIT_CODE%.
pause
exit /b %EXIT_CODE%
