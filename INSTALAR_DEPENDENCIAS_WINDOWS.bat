@echo off
setlocal
cd /d "%~dp0"
title Instalar Worktic AI V13.0.4

set "APP_EXE=%~dp0worktic-ai-v13.exe"

echo [1/4] Desbloqueando archivos descargados...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-ChildItem -LiteralPath '%~dp0' -Recurse -File | Unblock-File" >nul 2>nul

echo [2/4] Verificando Go...
where go >nul 2>nul
if errorlevel 1 (
  echo ERROR: Go no esta instalado o no esta disponible en PATH.
  echo Instala Go 1.24 o superior y vuelve a ejecutar este archivo.
  pause
  exit /b 1
)
go version

echo [3/4] Descargando dependencias...
go env -w GOPROXY=https://proxy.golang.org,direct
go mod tidy
if errorlevel 1 (
  echo ERROR descargando dependencias.
  pause
  exit /b 1
)

echo [4/4] Compilando...
if exist "%APP_EXE%" del /f /q "%APP_EXE%" >nul 2>nul
go build -mod=mod -o "%APP_EXE%" .
if errorlevel 1 (
  echo ERROR compilando.
  pause
  exit /b 1
)

echo.
echo Instalacion completada correctamente.
echo Ejecuta INICIAR_WINDOWS.bat para abrir Worktic.
pause
