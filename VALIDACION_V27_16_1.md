# WorkticAI V27.16.1 — Partner Admin Loader Hotfix

## Problema corregido
La pestaña Administración > Partners quedaba en “Cargando administración Worktic Partners…” porque `partners-v2715.js` instalaba un `onclick` al cargar la página, pero `admin-full.js` instalaba sus handlers de pestañas de forma diferida al entrar a Administración y sobreescribía ese `onclick`.

## Corrección
- `partners-v2715.js` expone `window.loadPartnerAdminV2715`.
- `admin-full.js` carga nativamente ese módulo cuando `activateTab("partners")` se ejecuta.
- Se elimina la competencia entre dos handlers `onclick`.
- Cache busting actualizado en `app.html`.

## Sin cambios
No cambia backend, PostgreSQL, Redis, esquema Partner, comisiones, payouts, Academy ni web pública.

## Validación
- `node --check static/admin-full.js`: OK
- `node --check static/partners-v2715.js`: OK
- `git diff --check`: ejecutar en repositorio local antes del push.
