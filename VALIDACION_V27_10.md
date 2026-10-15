# Validación WorkticAI V27.10 — Commercial Website + Annual Billing

## Fuente
Base usada: `WhatsappCloudGit(4).zip`, verificada como **WorkticAI V27.9.2 — Copilot Functional Hotfix**, rama `main`, commit `8154e68`.

## Alcance
La actualización preserva la aplicación V27.9.2 y cambia únicamente:
- `static/index.html` — web comercial pública completamente actualizada.
- `static/login.js` / `static/login.html` — preservación del plan/ciclo elegido desde la landing.
- `static/app.js` / `static/app.html` — checkout mensual/anual y cache busting.
- `main.go` — checkout y activación mensual/anual.
- `promo_codes.go` — promo codes compatibles con el precio base del ciclo elegido.
- `VERSION.txt`.
- `VALIDACION_V27_10.md`.

## Facturación anual
Descuento comercial anual fijo: **20%** sobre 12 mensualidades.

| Plan | Mensual | Anual | Equiv. mensual | Ahorro anual |
| --- | ---: | ---: | ---: | ---: |
| Free | 0 | 0 | 0 | 0 |
| Personal | 25 USDT | 240 USDT | 20 USDT | 60 USDT |
| Negocio | 75 USDT | 720 USDT | 60 USDT | 180 USDT |
| Empresa | 150 USDT | 1.440 USDT | 120 USDT | 360 USDT |

No se crea ninguna tabla ni columna nueva. El ciclo anual se determina mediante el `original_amount_usdt` ya existente en el pago. Al aprobar un pago anual, la suscripción vence a `now.AddDate(1,0,0)`.

## Seguridad de producción
- NO ejecutar `/app/worktic-migrate`.
- NO cambiar `DATABASE_URL`, `REDIS_URL`, `BASE_URL`, `CHANNEL_ENCRYPTION_KEY` ni credenciales Meta/OpenAI.
- No se cambia Docker/Go ni la infraestructura Render.

## Validaciones
```powershell
Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
go test ./...
node --check .\static\app.js
node --check .\static\login.js
git diff --check
git status
```
