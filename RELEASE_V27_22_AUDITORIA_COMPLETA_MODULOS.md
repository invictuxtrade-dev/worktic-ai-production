# WorkticAI V27.22 — Auditoría integral, matriz de módulos y permisos comerciales (STAGING)

**Base comprobada:** `WorkticAI V27.21 — Commercial Credits + Entitlements`, fuente V27.20→V27.19. No se publicó nada en GitHub ni Render. Este ZIP es una base de desarrollo para la fase comercial, **NO un release listo para clientes**.

## Alcance real de esta entrega

- **198 registros HTTP del router** inventariados en `audits/RUTAS_COMPLETAS_V2722.csv` mediante `scripts/audit_all_routes_v2722.py` (script local de solo lectura). 31 grupos técnicos en `audits/RESUMEN_RUTAS_V2722.json`. La clasificación incluye legacy/webhooks/administración, no equivale a validación funcional o de seguridad.
- **Matriz transversal de costos** de 31 grupos: `audits/MATRIZ_COSTOS_TODOS_MODULOS_V2722.csv`, con pagador, unidad de medida, estado de facturación y señalización cuando el importe NO está medido. **No se inventaron precios** para llamadas Meta/Google/TikTok/WhatsApp o infraestructura.
- **17 módulos con puertas de plan piloto**: se invocan inmediatamente tras autenticar sesión y antes de ejecutar el handler. `WORKTIC_MODULE_GATES_V2722=1` y `WORKTIC_COMMERCIAL_ENABLE=1` deben estar ambas activadas; por defecto **ambas apagadas**. Se pueden negar Ads Center, Analytics, agentes avanzados, workflows, Social Governance, grupos, WhatsApp Marketing, Meta Leads, y demás módulos en el catálogo; el cliente puede consultar la matriz desde `/api/commercial/full-entitlements` con sesión. No afecta rutas de callbacks públicos, pagos, administración, Worktic Partners ni webhooks; estos requieren su propia auditoría.
- Derechos configurables (opt-in) por plan mediante `billing_plans.features_json` para claves del catálogo con valores booleanos. Los nombres alternos `business/negocio` y `enterprise/empresa` se reconocen; admin debe confirmar plan almacenado en DB. Ejemplo `{"ads":false,"analytics":true}`. No se modifican los valores de la BD por defecto.
- UI responsiva de beneficios reales en Worktic Studio, usando la API del backend (muestra explícitamente cuando las puertas de plan están desactivadas).
- **Corrección de Marketing:** `marketingLimits()` resolvía `activePlan(tenant_id)` como si fuera `owner_user_id`, y por tanto podía aplicar restricciones equivocadas. Ahora consulta al titular de facturación desde la sesión del tenant.
- **Límite de número de workflows** por empresa ligado a `max_rules` cuando los gates están activados. **No es todavía a prueba de concurrencia**: se requiere row-lock transaccional al crear en PostgreSQL antes de publicar.
- Conserva estructura de V27.21: billetera transaccional de Creative Credits, Core IA y Cost Guard (con las limitaciones ya documentadas).

## Matriz resumida de puertas (propuesta piloto, NO texto de oferta pública)

| Módulos | Free | Personal | Negocio | Empresa |
|---|---|---|---|---|
| CRM, Catálogo, Agenda, Inbox, canales básicos, Marketing orgánico, Landings, Social Hub básico, Studio manual | Sí (cuotas históricas) | Sí | Sí | Sí |
| Agentes IA avanzados, Workflows, Analytics | No | Sí (cupos) | Sí | Sí |
| Ads Center, Social Governance, Meta Lead Ads, WhatsApp Marketing, grupos/prospección | No | No | Sí (cupos pendientes) | Sí |
| Studio IA | Sin créditos | 30/mes | 120/mes | 300/mes |
| IA conversacional general | 0 | 150/mes | 500/mes | 1500/mes |
| Límites originales usuarios/contactos/productos/canales | 1/100/5/1 | 1/500/100/2 | 5/3000/1000/3 | 15/10000/5000/6 |

El motor revisa `features_json` cuando corresponde. La tabla es una hipótesis que debe reconciliarse con los textos actuales y las expectativas comerciales antes de fijar precios.

## Modelo financiero preliminar — no equivale a margen real

El escenario histórico de descuento anual del 20%, comisión máxima Partners del 50% y reservas IA V27.21 deja después de esas partidas US$7,75 por Personal, US$22 por Negocio y US$37,50 por Empresa, **ANTES de pasarela, Render, Meta, mensajes, GB, soporte, impuestos y costos fijos**. Ver `audits/ESCENARIOS_FINANCIEROS_V2722.csv`. Todos esos saldos pueden reducirse o volverse negativos.

## Cómo hacer las verificaciones sin clientes

1. **NO modificar producción:** usar worktree de laboratorio y PostgreSQL/Redis de staging aislados. Dejar `WORKTIC_COMMERCIAL_ENABLE=0` y `WORKTIC_MODULE_GATES_V2722=0` mientras se revisa la lógica.
2. Ejecutar `go version`, `go test ./...`, `go build ./...` con Go 1.26 y `node --check static/commercial-modules-v2722.js`. En este entorno solo se ejecutaron pruebas Go aisladas con Go1.23 (PASS) y validación de JS; **no se pudo compilar el proyecto completo**.
3. Ejecutar `python scripts/audit_all_routes_v2722.py` y `python scripts/create_cost_matrix_v2722.py`. Confirmar que el número de rutas y grupos no disminuyó y revisar la lista de excepciones.
4. Hacer cuentas Free/Personal/Negocio/Empresa en staging, configurar permisos con administrador y activar flags allí únicamente.
5. Validar 403 en módulo no incluido, 200/estado esperado cuando está incluido, superadmin correctamente restringido por roles en sus rutas, y tenant A ≠ tenant B. Confirmar `features_json` y expiración de suscripción.
6. Prueba de concurrencia PostgreSQL sobre saldo creativo, respuestas IA, alta de usuarios, conexiones, productos y workflows. Ver `audits/BLOQUEADORES_LANZAMIENTO_V2722.md`.
7. Ensayar con **proveedores falsos** para estrés y luego un presupuesto pequeño de API real con topes configurados por proveedor. Registrar respuestas fallidas, pagadas, reembolsos, tiempos, modelo y tokens.
8. Cargar facturación de Render, OpenAI/Luma y volumen de Meta a la matriz de costos. Definir límite de gasto por módulo y unidad; decidir qué partidas paga el cliente aparte (anuncios y WhatsApp de pago variable).
9. Homologar pagos anuales, promos, aprobaciones y Partners. Verificar *todas* las promesas de la web comercial antes de activarlas.

## Riesgos y limitaciones que impiden anunciar “100% listo”

- Rutas legacy y `worktic_auto_rules` son globales; **necesitan tenantización segura de datos históricos**. No alterar directamente las tablas reales sin backup y reconciliación.
- Los gates de módulos bloquean rutas HTTP cuando están activados; no reemplazan el control fino de acciones/roles, webhooks, ejecuciones asíncronas ni los límites de volumen transaccionales. Hacen falta pruebas end-to-end para cada grupo.
- La tarificación actualmente registrada para Studio/Core es estimada. No se incluye medición completa de tokens, mensajería por categoría/país ni metering de ancho de banda.
- Hay modelos de terceros que cambian precio, disponibilidad y políticas; la matriz de créditos debe ajustarse contra facturas, no con un importe único universal.
- Worktic Partners no se modifica: sus derechos, comisiones, reversos y flujos de payout requieren pruebas contables aparte.

## Archivos nuevos / cambiados en V27.22

- NUEVOS: `commercial_module_entitlements_v2722.go`, `commercial_module_entitlements_v2722_test.go`, `static/commercial-modules-v2722.js`, `static/commercial-modules-v2722.css`, `scripts/audit_all_routes_v2722.py`, `scripts/create_cost_matrix_v2722.py`, archivos `audits/*` y este informe.
- MODIFICADOS: `main.go` (ruta, gate tras auth), `marketing.go` (plan correcto), `automations_v20.go` (límite piloto), `static/app.html` (assets) y `VERSION.txt`.

**No hay despliegue. No hay migración manual de BD. No hay cobros reales ni consumo de IA externa en las pruebas.**
