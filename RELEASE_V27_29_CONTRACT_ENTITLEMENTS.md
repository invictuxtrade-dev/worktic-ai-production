# WorkticAI V27.29 — Contractual Entitlements & Plan Versioning

**Base inmutable:** V27.28. **GitHub/Render:** ningún despliegue. **Entorno:** STAGING ONLY.

## Qué hace

- En la vía de aprobación segura V27.25/V27.28, el servidor captura **en la misma transacción** el plan, cuota CRM, productos, usuarios, canales, reglas, respuestas IA, `features_json`, derechos de Studio y precio de catálogo; genera un SHA-256 reproducible que identifica la versión contractual.
- Guarda snapshots históricos inmutables (`contract_plan_versions_v2729`) y su relación con la suscripción (`contract_subscription_terms_v2729`). La misma versión puede reutilizarse en varios contratos sin sobrescribir documentos anteriores.
- Con el piloto activado, los permisos de módulos avanzados, cuotas principales de `activePlan` y derechos principales de Studio se leen del snapshot, no del plan modificado posteriormente. Reserva IA general usa la cuota contractual.
- Una renovación vigente con versión distinta se **bloquea** antes de aprobar el pago; igual para contrataciones históricas vigentes sin snapshot. Esto impide reducir prestaciones mientras hay días pagados, pero **no es todavía prorrateo ni renovación con cambio de condiciones**.
- `GET /api/billing/v2729/contract`: titular/usuario autenticado, devuelve estado contractual de su empresa; para contratos sin snapshot informa revisión necesaria. `GET /api/admin/billing/v2729/audit`: superadmin, solo lectura, cantidad de contratos antiguos activos sin versión.

## Activación controlada (NO PROD)

```env
WORKTIC_COMMERCIAL_ENABLE=1
WORKTIC_BILLING_V2725_SAFE_APPROVAL=1
WORKTIC_BILLING_LIFECYCLE_V2728=1
WORKTIC_CONTRACT_ENTITLEMENTS_V2729=1
```

Todos los flags de contratos están APAGADOS por defecto. No activar sobre contratos reales hasta migrar con revisión manual. La activación sobre cualquier contrato pagado preexistente sin snapshot **impide resolverlo**, intencionalmente. Superadmin debe auditar y aprobar un procedimiento de migración; esta fase no reasigna antiguos clientes.

## Qué NO hace / riesgos de lanzamiento

- Los nuevos snapshots nacen solo desde la **aprobación segura**: las licencias de cortesía, alta histórica Free, actualizaciones manuales de suscripciones y migraciones no generan snapshot. Todos esos caminos requieren homologación antes del lanzamiento.
- Los beneficios todavía no cubren todas las columnas extendidas de planes (por ejemplo, sublímites específicos WhatsApp, Telegram, Messenger y agentes); las cuotas implementadas en otros módulos deben unificarse con el contrato en V27.30.
- La cuota IA general todavía aplica un límite piloto `commercialCoreCap`, incluso si el catálogo era más generoso. Antes de vender, la oferta deberá declarar ese mínimo y sus excepciones. Una renovación con cambio de tarifa/catálogo se detiene; no hay consentimiento ni prorrateo automatizados.
- El uso de créditos de un mes en que cambia la suscripción y las reservas ya abiertas requieren ensayos PostgreSQL reales.
- No hay verificación on-chain automática, contabilidad fiscal, pruebas de costo vendor ni validación en Render. No alterar Worktic Partners ni migrar SQLite/PostgreSQL otra vez.

## Pruebas antes de desplegar

```powershell
go version
go test ./...
go build ./...
node --check static/app.js
```

Crear staging independiente con PostgreSQL, Redis, storage y usuarios ficticios. Probar: alta, pago manual verificado, captura de snapshot, edición posterior de catálogo, acceso CRM/Studio, renovación idéntica, renovación con derechos cambiados (debe bloquear), rollback por excepción, pago duplicado, cortesías, contrato histórico sin snapshot, derechos al vencer, reembolsos y varios workers. Solo después definir una migración contractual específica para usuarios existentes.
