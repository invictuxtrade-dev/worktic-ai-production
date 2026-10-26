# WorkticAI V27.34.8 — Acciones visibles en Créditos Studio

Base: V27.34.7. Cambios restringidos a la presentación de la billetera y caché.

## Corrección
- **Ver movimientos** abre un panel emergente accesible con consulta GET `/api/commercial/movements`, lista real de movimientos por empresa o mensaje de historial vacío/error.
- **Recargar créditos** abre panel emergente: si `topups_enabled=false`, explica el bloqueo y presenta solo paquetes de referencia sin aceptar pagos; si el backend autoriza recargas, utiliza el formulario existente, sujeto a verificación administrativa.
- El botón `+` de la barra superior abre el mismo panel de recargas.
- El panel es responsive, soporta Escape, cierre, y no modifica registros ni privilegios.
- Se actualizó el versionado de caché de CSS/JS.

## Verificaciones obligatorias antes del despliegue
1. `python tools/v2734/verify_manifest.py`
2. `node --check static/studio-wallet-v27346.js`
3. `go test ./...`
4. `go build ./...`
5. `git diff --check`
6. Comprobar ambos botones en el navegador autenticado, escritorio y móvil.

**Seguridad:** no se cambian endpoints de cobro, la billetera, créditos, reglas comerciales o presupuesto. Los cobros siguen desactivados hasta habilitación oficial.
