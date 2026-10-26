# WorkticAI V27.25 — Cost Accounting & Billing Reconciliation

**Base exacta:** `WorkticAI_V27.24_Resource_Quotas_Analytics_Ads_Base_V27.23.zip`, no repositorio antiguo. **Sin despliegue ni cambios a producción.**

## Alcance implementado

- Registro aditivo de facturas comprobadas por proveedor/periodo en `finance_invoices_v2725`, con `invoice_ref` único dentro de proveedor, divisa USD obligatoria, evidencia referenciada y superadministrador revisor.
- Contabilidad de asignaciones a operaciones/empresa en `finance_actuals_v2725` con referencias de costo irrepetibles, reserva Studio opcional, comprobación de `tenant_id`/proveedor cuando hay reserva, y transacción que impide asignar más que el importe documental. El costo general sin empresa se registra con `tenant_id=0`. **No se asignan facturas globales entre clientes automáticamente.**
- Aritmética en **centavos USD enteros**, no floats; ingresos facturados **USDT** separados y sin mezclar con USD. No se muestran falsos márgenes o costos de plataforma a usuarios finales.
- Superadmin: vista `Cost Accounting & Reconciliation` en Administración, con resumen, estado de facturas, formulario de documento, formulario de costos asignados y solo lectura por defecto.
- API superadmin: `GET /api/admin/finance/v2725/summary`, `GET|POST /api/admin/finance/v2725/invoices`, `GET|POST /api/admin/finance/v2725/actuals` (parámetro GET `period=AAAA-MM`). La importación POST requiere `WORKTIC_FINANCE_V2725_WRITE=1`, sesión superadmin, `verified_source=true`, factura registrada, referencias válidas y condición de no exceder el total.
- API tenant: `GET /api/billing/v2725/statement` ofrece solo conteos de operaciones, **nunca costos internos del proveedor ni referencias de facturación**, separada de la billetera creativa.
- Checkout de suscripción registra `billing_cycle` explícito (`monthly`/`annual`). Los pagos anteriores quedan marcados `unverified` y no se convierten o migran mediante inferencias basadas en el precio.
- Con flag `WORKTIC_BILLING_V2725_SAFE_APPROVAL=1`, aprobación/rechazo transaccional, verificación expresa `verified_on_chain=true` por superadmin, fechas de renovación sin perder tiempo ya pagado para el mismo plan, y un lock por empresa para pagos concurrentes. Reutiliza el procesador idempotente de comisiones Partners **sin tocar el plan de compensación**. Si el procesado Partners falla, se informa que requiere conciliación; el pago no se revierte automáticamente.
- Una devolución de créditos comprados **ya utilizados o reservados** no puede crear saldo negativo; la operación se rechaza y requiere resolución administrativa.
- Rutas heredadas de cortesía y cambios de licencia del superadmin siguen existiendo; sus efectos financieros no se han conciliado. La aprobación segura permanece desactivada hasta cubrir todas esas rutas.

## Instrucciones de staging, no de producción

1. Crear copias de PostgreSQL, Redis y almacenamiento segregadas de producción. No ejecutar ninguna migración histórica SQLite→PostgreSQL.
2. Verificar que la versión base realmente corresponda a la **V27.24** empaquetada. Compilar con Go 1.26: `go test ./...` y `go build ./...`. Verificar JS con `node --check`.
3. **No configurar inicialmente las nuevas variables de habilitación.** El esquema puede crearse de forma aditiva al iniciar, pero los formularios estarán bloqueados y se conserva el flujo anterior de aprobación de planes.
4. Para simular facturas con datos NO reales, hacerlo solamente en staging y etiquetar cada `invoice_ref` con `test-`. Después de comprobar roles y CSRF/sesión, activar temporalmente `WORKTIC_FINANCE_V2725_WRITE=1` en staging. Para importar costos reales, revisar documentos de OpenAI/Luma/Meta/Render con referencias verificadas; importar totales y asignaciones **sin superar la factura**.
5. Para probar ciclos, activar solo en staging `WORKTIC_BILLING_V2725_SAFE_APPROVAL=1`. Crear checkout mensual y anual NUEVOS. Verificar caso de renovación del mismo plan, cambio de plan, pagos concurrentes del mismo tenant, status `pending`/`approved`/`rejected`, reversos/Partners. Los pagos existentes marcados `unverified` deben auditarse manualmente y nunca aprobarse de forma automática.
6. Confirmar que los roles tenant NO acceden a endpoints `api/admin/finance/` y que `api/billing/v2725/statement` no revela costos de proveedores.
7. Auditar reversos de créditos y pagos en ambos sentidos, con operaciones concurrentes y compras ya gastadas, antes de habilitar recargas reales.
8. No publicar nuevas tarifas, promesas de utilidad o «costo real por cliente» hasta conciliar facturas e infraestructura y probar multi-worker PostgreSQL.

## Ejemplos de API (solamente superadmin, staging autorizado)

Factura verificada:

```json
{"provider":"luma","invoice_ref":"test-invoice-2026-10","period":"2026-10","total_cents":100,"currency":"USD","evidence_ref":"test-document-2026-10","verified_source":true}
```

Asignación a empresa (importe ≤ restante en la factura):

```json
{"provider":"luma","source_ref":"test-request-001","invoice_ref":"test-invoice-2026-10","tenant_id":12,"period":"2026-10","operation":"video","hold_id":"","actual_cents":30,"verified_source":true}
```

Estos JSON son **ejemplos de datos sintéticos**: no constituyen facturas, pruebas de cobro ni evidencia de proveedor.

## Rutas y datos que requieren vigilancia financiera

| Área | Qué se dispone | Pendiente para costo completo |
|---|---|---|
| Studio OpenAI/Luma | reserva estimada, factura externa y conciliación manual | automatizar IDs de generación, costos de tokens reales y reembolsos del proveedor |
| Copilot/Agentes | cuota de respuestas previa | tokens por modelo y costo real agrupado por tenant/proyecto |
| WhatsApp/Meta | logs de mensajes y eventos | detalles de facturación por país, categoría, plantilla, reintentos, devoluciones |
| Automatizaciones | cuota de ejecuciones | costo por cada acción externa de workflow, fan-out y reintentos |
| Ads Center | solicitudes de sincronización | gasto de publicidad **pagado por anunciante** separado, rate limits y fan-out |
| Analytics | conteo de eventos | espacio BD, retención sin pérdida y costo de agregaciones y lecturas |
| CRM/Agenda/Catálogo | límites piloto en rutas manuales | importaciones, uploads, exportación, espacios y notificaciones |
| Partners | comisiones y reversos heredados | pruebas reales de payouts, descuentos y anualidades bajo carga |
| Suscripciones y recargas | ciclo explícito, aprobación opcional segura | conciliación de cadena, pago real, facturación diferida e impuestos |
| Infraestructura | permite facturas globales sin atribución | medir CPU, RAM, PostgreSQL, Redis, disco, backups, salida de red y prorrateo justificado |

**Definición de listo:** dinero realmente cobrado y costeado en el período contable, reglas de negocio cubiertas por E2E, simultaneidad en PostgreSQL real con dos workers, créditos de prueba sin bypass, márgenes comparables en USD solo al disponer de FX documentado y retención de eventos validada.

## Riesgos conocidos

- `WORKTIC_FINANCE_V2725_WRITE` no prueba de forma automática que una factura sea legítima: el superadministrador debe verificar el documento. Los importes «verificados» son *atestaciones manuales* con trazabilidad.
- La distribución automática de facturas globales de OpenAI/Render por tenant **no es posible** si el proveedor no entrega granularidad o no hay medidor inequívoco; debe quedar sin asignar.
- Pago anual se registra correctamente en checkout nuevo, pero pagos antiguos con ciclo no identificado requieren revisión; activar aprobación segura con ellos impide su aprobación hasta que se corrijan.
- **No se implementó la contabilidad de ingresos devengados** (anualidad prorrateada) ni la tasa USDT/USD. El resumen informa ingresos aprobados en USDT, NO el margen empresarial.
- El cálculo de costos internos no debe convertirse en saldo facturable del cliente. Los débitos de créditos Studio siguen el módulo comercial V27.21.
- Las pruebas con Go completo, PostgreSQL, Render y proveedores reales siguen pendientes. Esta versión NO es certificación de comercialización.

## Referencias de proveedores consultadas

- OpenAI API — Usage y Costs: https://platform.openai.com/docs/api-reference/usage
- Luma Agents — Pricing: https://docs.agents.lumalabs.ai/guides/pricing
- Luma Agents — FAQ de cobros y fallos: https://docs.agents.lumalabs.ai/guides/faq/
