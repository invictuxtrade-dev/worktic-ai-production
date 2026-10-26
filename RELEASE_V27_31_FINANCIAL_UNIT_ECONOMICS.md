# WorkticAI V27.31 — Costeo financiero por plan y paquete, sin falsos márgenes

**Fase 3 de 5** del plan de comercialización. Base exacta: `WorkticAI V27.30 — Tenant Isolation + Legacy Route Guard`. **NO desplegado.** No modifica datos persistentes ni facturación, comisiones ni beneficios reales. No activa banderas.

## Implementación real

- Backend `finance_unit_economics_v2731.go`: `GET /api/admin/finance/v2731/unit-economics` exige sesión de **superadministración**, consulta tablas existentes `billing_plans` + `commercial_rules_v2721` + `commercial_packages_v2721`, medidores `finance_operation_meter_v2726` y facturas `finance_invoices_v2725`/asignaciones verificadas. Solo lectura; no aplica migraciones, no ejecuta pagos ni llama proveedores.
- Frontend `static/financial-scenarios-v2731.js/css` añade sección en Administración; exige que se introduzcan explícitamente todos los supuestos, cotización USDT/USD con fecha y referencia de fuente. No tiene supuestos de infraestructura o costos de IA prefijados. Todas las cifras de utilidad son **simulaciones**, nunca lucro comprobado.
- Escenarios por **cada plan activo en BD**, mensual y anual: ingreso mensual equivalente tras descuento anual, comisión máxima Partners, procesamiento de pagos, cupos completos IA, reserva de variación proveedor, infraestructura y otros costos por tenant. Usa el menor cupo de respuestas entre catálogo y piloto. Estos planes actuales NO sustituyen snapshots de contratos antiguos, ni los configuran.
- Escenarios por paquete de crédito: precios y créditos de la BD, comisión sobre recargas **distinta** de la de suscripciones, costos máximos de proveedor por crédito y buffer de variación. No cambia el programa Worktic Partners ni aprueba una comisión nueva: permite analizar hipótesis.
- Todos los escenarios retornan `public_launch_approved:false` incluso cuando pasan un margen numérico, porque faltan gastos probados, contratos, devoluciones y pruebas de staging. Los importes en USD son cifras simuladas desde precios USDT y una tasa de cambio introducida por el operador, no dinero liquidado ni una cotización consultada automáticamente.
- `tools/v2731/financial_preflight.py` audita offline los 31 grupos históricos de costos y produce `audits/COSTEO_COBERTURA_V2731.json`. No accede al servidor, a proveedores ni a secretos. Al revisar la matriz histórica, los 31 grupos siguen sin certificación integral; ni siquiera los marcados `partial` se tratan como verificados.

## Cómo usar el panel (en staging, después de completar Go 1.26/PostgreSQL)

Administración Worktic → **Rentabilidad por plan y recargas** → indicar cotización USDT/USD, fuente y fecha (máximo siete días), los porcentajes en puntos básicos y todos los costos operativos unitarios → **Calcular escenarios de prueba**.

Ejemplo *estrictamente hipotético* con plan Negocio USDT 75: FX 1 USD por USDT, anualidad 20%, Partners 50%, recargas 10%, procesador 3%, margen objetivo 20%, buffer proveedor 20%, máximo 3 centavos USD/crédito, 1 centavo USD/respuesta, infraestructura 500 centavos USD/empresa/mes y otros 200 centavos USD/empresa/mes:

- 120 créditos × 3¢ + 500 respuestas × 1¢ + buffer 20% = US$10,32.
- Ingreso mensual equivalente anualizado = US$60; Partners US$30, procesador US$1,80, infraestructura US$5 y otros US$2.
- Contribución hipotética = US$10,88 (18,13%); **NO supera** objetivo 20% bajo esos supuestos.
- Mensualidad nominal: US$75 equivalentes con ese FX, contribución hipotética US$17,93 (23,91%).
- Un paquete Creator de 100 créditos / USDT 10, con 10% Partners recargas: US$5,10 de contribución hipotética antes de otros gastos y reversos.

Estas cifras **NO** certifican precios y son solo ejemplos editables. No son recomendaciones de márgenes reales ni asumen que USDT siempre vale US$1.

## Campos obligatorios del endpoint

`usd_per_usdt`, `fx_source`, `fx_asof` (últimos siete días), `annual_discount_bps`, `partners_subscription_bps` (máximo 5000), `partners_topup_bps`, `payment_processing_bps`, `variable_cost_buffer_bps`, `target_margin_bps`, `max_supplier_credit_cents`, `max_supplier_response_cents`, `infra_per_tenant_month_cents`, `other_per_tenant_month_cents`, `period=AAAA-MM` opcional.

`bps`: 100 puntos básicos = 1%. Las entradas de costos son **centavos de USD**. Parámetros ausentes, NaN, negativos o fuera del rango aprobado producen `status=missing_required_financial_inputs`, no un margen artificial.

## Lo que NO está cerrado y bloquea publicación

1. Costos reales de OpenAI: tokens/modelo/calidad y cambios de tarifas, facturas y estimaciones confirmadas.
2. Generación Luma: cargos por modelo, resolución, segundos, reintentos y reembolsos; todavía falta ejecutar API real.
3. Meta/WhatsApp: tarifas por país, categoría y ventana; Ads Center: gasto del anunciante separado de costo Worktic.
4. Render/DB/Redis, archivos, transferencia, almacenamiento, colas, soporte y fiscalidad: medición en staging, costos fijos/variables y costo por tenant.
5. Ejecución real de Worktic Partners 50% techo según casos contractuales, reversos, retiros y comisiones diferentes en recargas; **este cambio no altera Partners**.
6. Pagos reales, USDT FX real recibido, anualidades, cupones, upgrades y depreciación/amortización de créditos acumulados y consumidos.
7. Matriz completa de 31 grupos con evidencia verificada (no inferir rentabilidad por ausencia de cargos), prueba de aislamiento en PostgreSQL y cobro/procesamiento por tenant.
8. Flujos de cliente y operaciones internas que podrían eludir límites o medidores.

## Tests ejecutados y pendientes

- Local aquí: 3 pruebas Go nuevas **en compilación aislada con stubs (Go 1.23)**; 3 tests Python offline; 70 archivos Go pasan AST; 35 JavaScript pasan sintaxis; 461 IDs HTML sin duplicados. Todos los cálculos comprobados son simulaciones.
- Pendiente: proyecto completo con **Go 1.26**, PostgreSQL multiinstancia, Redis, stubs de APIs, UI E2E, facturas verificadas, finanzas y carga.

En terminal de VS Code para la carpeta de staging: `go version`; `go test ./...`; `go build ./...`; `python tools/v2731/financial_preflight.py`; `python -m unittest discover -s tools/v2731 -p 'test*.py' -v`; `node --check static/financial-scenarios-v2731.js`.

## Próxima hoja de ruta (restan 2 fases planificadas)

- **V27.32**: validación integral en staging, seguridad, recorridos de plan y pagos, concurrencia real Go/PostgreSQL/Redis, carga 10/100/500/1000 empresas sin usar proveedores pagados, medir base de costos.
- **V27.33**: resolver hallazgos de V27.32 y construir candidato productivo, preparar rollback, variables/secretos, checklist de habilitación por fases y documentación de despliegue sin sustitución de la versión existente.

**No hay fecha o garantía de despliegue**: depende del resultado de V27.32/33. Se necesitan datos observados y aprobaciones antes de habilitar comercio real.
