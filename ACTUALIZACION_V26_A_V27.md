# Actualización WorkticAI V26 → V27

Esta actualización parte de la instalación V26 ya migrada a PostgreSQL y Redis.

## Antes de subir

En tu PC, dentro del proyecto V27:

```powershell
go version
go test ./...
```

Debe usar Go 1.26.x y terminar sin errores.

Opcionalmente valida build Linux:

```powershell
$env:CGO_ENABLED="0"
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -trimpath -ldflags="-s -w" -o worktic-ai-linux .
go build -trimpath -ldflags="-s -w" -o worktic-migrate-linux ./cmd/migrate
Remove-Item worktic-ai-linux,worktic-migrate-linux
Remove-Item Env:CGO_ENABLED,Env:GOOS,Env:GOARCH
```

## Despliegue

No vuelvas a ejecutar la migración SQLite → PostgreSQL de V26. Esa migración ya terminó.

Sube la V27 normalmente:

```powershell
git add -A
git commit -m "WorkticAI V27 Growth Command Center"
git push origin main
```

Render construirá con:

```dockerfile
FROM golang:1.26-bookworm AS builder
COPY go.mod go.sum ./
```

Al arrancar, V27 crea automáticamente sus tres tablas de WhatsApp Marketing en PostgreSQL.

## Variables

No se requieren variables nuevas. Confirma especialmente:

```env
BASE_URL=https://workticai.com
OPENAI_API_KEY=<ya existente>
BACKGROUND_WORKERS_ENABLED=true
DATABASE_DRIVER=postgres
DATABASE_URL=<Internal Database URL>
REDIS_REQUIRED=true
REDIS_URL=<Internal Redis URL>
```

No cambies `CHANNEL_ENCRYPTION_KEY`.

## Smoke test V27

1. Login normal.
2. Resumen debe mostrar Growth Command Center.
3. Abrir Copilot flotante y preguntar: `Revisa mi configuración`.
4. Abrir WhatsApp Marketing.
5. Confirmar que aparece al menos una conexión Cloud ya configurada.
6. Crear campaña en borrador con plantilla aprobada.
7. Previsualizar audiencia: solo debe contar leads con consentimiento.
8. Iniciar una campaña de prueba hacia una audiencia reducida.
9. Confirmar estados `sent/delivered/read` cuando Meta los reporte.
10. Responder desde el teléfono y confirmar `replied` + conversación en Inbox.
11. Enviar `SALIR` desde un número de prueba y confirmar que queda excluido de campañas futuras.
12. Revisar `/healthz` y `/readyz`.

## Rollback

V27 no modifica ni borra las tablas V26. Si debes volver al commit V26, las tablas V27 pueden permanecer sin afectar la versión anterior.
