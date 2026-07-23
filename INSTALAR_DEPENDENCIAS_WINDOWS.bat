@echo off
cd /d "%~dp0"
echo [1/3] Verificando Go...
go version || (echo Instala Go 1.24 o superior & pause & exit /b 1)
echo [2/3] Descargando dependencias...
go env -w GOPROXY=https://proxy.golang.org,direct
go mod tidy
if errorlevel 1 (echo Error descargando dependencias & pause & exit /b 1)
echo [3/3] Compilando...
go build -mod=mod -o worktic-omnichannel-v3.exe .
if errorlevel 1 (echo Error compilando & pause & exit /b 1)
echo Instalacion completada.
pause
