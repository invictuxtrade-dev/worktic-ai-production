# WorkticAI V27.28 — Commercial Lifecycle & Payment Claim Guard (STAGING ONLY)

**Base:** ZIP V27.27 exacto. **GitHub/Render:** no se modificaron. **Estado:** validación técnica parcial; no certificado para ventas públicas. Conserva Worktic Partners, Studio, CRM, Marketing, Ads y demás módulos de V27.27; cambios incrementales y limitados a pagos, administración, preflight y una nueva tabla.

## Cambios de seguridad comercial

1. **Vista previa de compra (`GET /api/billing/v2728/preview`)** autenticada y limitada al titular: informa plan/ciclo seleccionado, importe **antes** de promociones, vencimiento proyectado y si el checkout piloto está permitido. No verifica ni efectúa pagos.
2. **Renovación del mismo plan:** conserva el tiempo no utilizado si se aprueba manualmente el pago y el ciclo es explícito (mensual o anual), aprovechando la aprobación segura V27.25.
3. **Cambio de plan con pago previo:** bloquea upgrades/downgrades mientras quede tiempo pagado; aún no existe una política aprobada de prorrateo/diferencias. Los pagos antiguos sin ciclo no se deducen por importe.
4. **Referencia blockchain compartida:** tabla nueva `billing_payment_claims_v2728`, llave única (`network`,`tx_hash`) normalizada. El `INSERT` del claim y el `INSERT` del pago/recarga comparten **una transacción SQL**, para impedir doble uso del hash entre suscripción y créditos. Rechaza hashes ya usados en tablas antiguas sin asignarlos automáticamente.
5. **Cortesías:** bajo el flag V27.28, una nueva cortesía no puede reemplazar una suscripción pagada aún vigente (tampoco con el mismo plan), ni extenderse manualmente un pago existente por la ruta de cortesías; evita romper la referencia del pago antes de un reembolso. La extensión de cortesías reales sigue disponible con validación.
6. **Auditoría de solo lectura** `GET /api/admin/billing/v2728/audit` para superadministración: ciclos sin verificar, pagos/recargas pendientes, solapamiento de suscripciones activas y colisiones históricas de hash entre productos.
7. **Frontend:** antes de presentar comprobante muestra el tipo de operación y posibles bloqueos. El servidor revalida todo; ocultar un botón nunca sustituye un guard backend.
8. **Preflight V27.27:** muestra el estado de la protección de ciclo V27.28 y el resumen de anomalías en Administración.

## Flags

**No activar aún en producción.** El código requiere AMBAS en **staging** para habilitar los controles de ciclo:

```env
WORKTIC_BILLING_V2725_SAFE_APPROVAL=1
WORKTIC_BILLING_LIFECYCLE_V2728=1
```

Para probar recargas de Studio además se necesita `WORKTIC_COMMERCIAL_ENABLE=1`, solo en staging con credenciales de prueba. Sin flags el comportamiento anterior se conserva, salvo la migración aditiva de la tabla y el endpoint de auditoría de solo lectura.

## Validación sugerida (Visual Studio Code / PowerShell)

1. Descomprimir este ZIP **en una carpeta nueva**, jamás encima de producción ni de un worktree Live.
2. Instalar Go 1.26 y ejecutar desde el directorio de `go.mod`:

```powershell
go version
go test ./...
go build ./...
node --check static/app.js
node --check static/commercial-validation-v2727.js
python tools/v2728/claim_concurrency_sim.py
```

3. Usar staging con PostgreSQL/Redis/disco separados y datos ficticios. Activar flags ahí y probar `preview`, checkout, aprobación, renovación, doble hash, cortesías, reembolso y conciliación con Partners. La simulación incluida con SQLite **no prueba** los handlers ni reemplaza concurrencia real multiinstancia PostgreSQL.
4. Verificar backup y plan de reversión antes de cualquier despliegue posterior.

## Bloqueos aún pendientes de lanzamiento

- **Prorrateo de cambio de plan** y política contractual de upgrades/downgrades. Actualmente se bloquea para evitar perjudicar al cliente, no se ofrece como funcionalidad lista.
- **Conciliación blockchain real** (red, contrato token, importe, dirección, confirmaciones, ventanas y posibles reorgs). El check `verified_on_chain` representa confirmación manual, no una consulta on-chain de la plataforma.
- **Reembolsos completos** de pagos y créditos, deudas/saldo consumido y reversos Partners en casos mixtos; Worktic Partners conserva su motor previo.
- **E2E con PostgreSQL y múltiples workers**. Solo lógica pura y simulación SQLite de referencia se validan localmente.
- **Entitlements inmutables por vigencia:** los planes se consultan actualmente como configuración vigente. Editar límites del catálogo puede modificar derechos de suscripciones existentes; hace falta una política de versiones y snapshots.
- **Reglas heredadas globales**, importaciones automáticas, senders externos y 31 grupos de costo siguen sujetos a la matriz de V27.27.
- **Facturación e infraestructura real** de Meta/Luma/OpenAI/Render y datos de 10/100/1000 tenants simulados no han sido medidos ni certificados.

**No se realizó ninguna publicación ni se utilizaron API pagadas.**
