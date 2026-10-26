# WorkticAI V27.34.6 — Studio Wallet Premium

Base exacta: WorkticAI V27.34.5 COMPLETO; no sobrescribir ramas Git con cambios independientes.

## Alcance
- Billetera de Studio rediseñada dentro del Studio, con vista contraíble para dejar visible el editor existente.
- Chip en cabecera desktop/móvil que consulta el saldo autenticado por tenant; botón + que abre recargas sin permitir pagos mientras el servicio no esté habilitado.
- Saldos reales del endpoint `/api/commercial/wallet`, cupos editables desde Administración (no se cambian cifras ni límites de contratación).
- Movimiento real de generaciones, reservas y recargas desde `/api/commercial/movements`, siempre consultando por tenant de la sesión. No hay actividad inventada.
- Botón recarga cuando está deshabilitada muestra el estado; cuando esté habilitada usa la solicitud existente de pagos sujetos a verificación administrativa. No se acreditan créditos por presentar hashes.
- Indicadores (créditos del plan/comprados, consumidos, reservados, progreso y equivalencias) y permisos reflejan la API existente.

## Importante
`WORKTIC_STUDIO_CREDITS_ENABLE=1` activa reservas solo si están configurados los presupuestos globales y de Personal/Negocio/Empresa. Debe comprobarse en staging antes de activarlo en producción. `WORKTIC_COMMERCIAL_ENABLE=1` habilita flujos comerciales más amplios y NO debe activarse para este parche solo de interfaz. Las recargas siguen inactivas hasta aprobación.

## Pruebas locales realizadas
- JavaScript verificado con Node.
- Go: sintaxis verificada con el parser estándar. No se ha compilado todo con Go 1.26 en el sandbox.
- UI: Chromium con API simulada; 1366px/390px, sin overflow, historial vacío, recargas bloqueadas, ocultar/mostrar billetera y errores JS=0.
- Manifiesto SHA-256 generado y verificado.

## Pendientes obligatorios antes de activar gasto de IA
- `go test ./...`, `go build ./...` con Go 1.26 en Windows.
- PostgreSQL real: reserva, consumo, liberación, rollover, recarga, idempotencia y aislamiento de tenant.
- Generaciones reales controladas OpenAI/Luma, límites de presupuesto y transacciones verificadas.
- Revisión de la versión Git `main` y respaldo de PostgreSQL + disco persistente `/var/data`.

## Archivos incrementales
VERSION.txt; RELEASE_MANIFEST_V2734.json; RELEASE_V27_34_6_STUDIO_WALLET_PREMIUM.md; main.go; studio_wallet_v27346.go; studio_wallet_v27346_test.go; static/app.html; static/commercial-v2721.js; static/studio-wallet-v27346.js; static/studio-wallet-v27346.css.
