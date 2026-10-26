# WorkticAI V27.23 — Restricciones y consumo por empresa (laboratorio)

**Base de código:** V27.22 / V27.21 / V27.20 / V27.19. **No desplegado. No es una certificación comercial.** Se conservaron todas las rutas y módulos existentes, incluidos Worktic Partners, Ads, CRM, WhatsApp y Studio. No se ejecutó migración manual de datos productivos.

## Qué se desarrolló

1. **Reglas legacy: separación segura.** La tabla histórica `worktic_auto_rules` **no registra propietario**. NO se distribuye automáticamente entre empresas. Se añade `worktic_auto_rules_v2723` con `tenant_id`, controles de sesión/rol, filtros de empresa en GET/PUT/DELETE y bloqueo transaccional por empresa al crear. El máximo `billing_plans.max_rules` se comparte entre reglas nuevas y workflows cuando ambos controles están activos. Ruta histórica `/api/rules` usa reglas nuevas únicamente si `WORKTIC_TENANT_RULES_V2723=1` y V27.23+commercial están activadas. El superadministrador puede revisar nombres e IDs sin revelar respuestas con `GET /api/consumption/v2723/legacy-rules`; cada propietario histórico requiere **asignación manual documentada**. La tabla original sigue intacta.
2. **Seguridad de auto-respuesta legacy:** cuando se habilita el modo de reglas por tenant, el despachador histórico `maybeAutoReply` se detiene: sus callbacks de WhatsApp QR/Telegram/Messenger no garantizan qué tenant posee cada mensaje. Evita filtrar respuestas de otro negocio, **pero significa que las reglas nuevas todavía NO contestan automáticamente desde estos canales**. El enrutamiento seguro por canal/tenant debe completarse antes del lanzamiento y antes de habilitar este flag en clientes reales. Los agentes tenant-aware tienen rutas independientes que necesitan pruebas específicas.
3. **Workflows: cupo mensual de ejecuciones:** `automation_monthly_usage_v2723` cuenta solicitudes iniciadas, incluidas fallidas. Si se habilita V27.23, los límites por plan se leen de variables de entorno con valor por defecto **0 (bloqueado)**. Se usa `UPDATE ... WHERE attempts < limite` y transacción atómica; el número de nodos se limita a 50 durante el piloto. No se garantizan los costos monetarios de nodos HTTP/SMS/IA, solo un cupo de intentos. Crear workflows ahora bloquea por tenant en PostgreSQL (`SELECT FOR UPDATE` sobre fila de control) o con escritura en SQLite durante transacción. No hay garantías hasta hacer pruebas E2E PostgreSQL real.
4. **Workers concurrentes:** `automation_events` y `lead_automation_outbox` reclaman pendientes mediante `UPDATE ... WHERE status='pending'` cuando el modo V27.23 está activo. `whatsapp_marketing_recipients_v27` reclama `queued → sending` atómicamente antes de enviar, para que dos instancias no envíen al mismo destinatario. Social Hub reclama `status→publishing` con condición para evitar publicaciones concurrentes del mismo post. **Importante:** un corte de red puede dejar elementos en `processing`, `sending` o `publishing` pendientes de conciliación MANUAL. NO reintentarlos a ciegas: el proveedor pudo haber aceptado el trabajo y cobrarlo.
5. **Medición de uso externo (parcial y explícita):** `external_usage_v2723` registra de forma idempotente, sin teléfonos, destinatarios ni mensajes, IDs devueltos por WhatsApp Cloud (texto), plantillas Marketing, prueba de plantilla, follow-up de leads y publicaciones Social Hub exitosas. El campo `estimated_cost_cents` es **NULL** (desconocido) hasta que se concilie contra la factura; una API aceptada no prueba entrega ni cobro. `GET /api/consumption/v2723/summary` devuelve uso y cupos del propio tenant; el superadmin puede seleccionar otro con `?tenant_id=123`. No es una factura, ni cubre todos los envíos.

## Activación únicamente en staging

Valores por defecto: `WORKTIC_COMMERCIAL_ENABLE=0`, `WORKTIC_MODULE_GATES_V2722=0`, `WORKTIC_CONSUMPTION_GUARD_V2723=0`, `WORKTIC_TENANT_RULES_V2723=0`.

Para probar con datos ficticios y proveedores simulados:

```env
WORKTIC_COMMERCIAL_ENABLE=1
WORKTIC_MODULE_GATES_V2722=1
WORKTIC_CONSUMPTION_GUARD_V2723=1
WORKTIC_TENANT_RULES_V2723=0
WORKTIC_AUTO_RUNS_PERSONAL_V2723=10
WORKTIC_AUTO_RUNS_NEGOCIO_V2723=50
WORKTIC_AUTO_RUNS_EMPRESA_V2723=100
```

`WORKTIC_TENANT_RULES_V2723=1` **solo** cuando hayas verificado la clasificación de reglas legacy y estés preparado para que la auto-respuesta histórica quede desactivada en staging. No copies estos valores a producción sin validación.

Las variables de presupuestos monetarios V27.20/V27.21 son independientes, y deben estar configuradas si se hacen pruebas de generación IA. No se hicieron llamadas de cobro reales a proveedores.

## Aceptación y rollback

- Requiere Go **1.26** para ejecutar `go test ./...` y `go build ./...`, y PostgreSQL/Redis de staging **aislados de producción**. Probar 20+ solicitudes paralelas POST de reglas/workflows, 20+ ejecuciones manuales por tenant, envíos a proveedores simulados, tenants A/B, transacciones de planes y expiraciones. Abrir 2 instancias del worker sobre la misma BD para confirmar las reservas. Ver `scripts/test_consumption_v2723.py` para simulaciones SQLite reproducibles.
- Verificar `GET /api/consumption/v2723/summary` en la sesión de empresa; el reporte muestra exclusivamente su tenant. Verificar que `?tenant_id=` no permita cross-tenant salvo superadmin y que el inventario legacy sea 403 para usuarios comunes.
- Durante la transición, dejar el guard apagado hasta que la migración legacy sea manual y esté confirmado el enrutamiento de los canales. Los cambios de esquema son aditivos; apagar flags restaura rutas antiguas **pero también restaura su comportamiento legacy no aislado**, por lo que NO equivale a remediación definitiva.
- No volver a correr la migración SQLite→PostgreSQL. No cambiar comisiones de Partners ni actualizar licencias en producción por esta entrega.

## Pendientes de lanzamiento P0 / P1

- **P0:** Responder reglas tenant-aware mediante cada conector sin recurrir al worker legacy global; migración explícita de reglas históricas, validación de canales y datos anteriores; pruebas PostgreSQL real bajo concurrencia.
- **P0:** Conciliación de precios Meta por país, plantilla, categoría y ventana de cobro, incluyendo estado de entrega; reglas de facturación al cliente; OpenAI tokens, Luma y servicios externos; costos de Render reales; margen financiero por tenant/plan con Partners y anualidad.
- **P0:** Control monetario de acciones externas iniciadas desde cada workflow (webhooks, mensajes, AI, fan-out); límites 0 no sustituyen una billetera por operación.
- **P0:** Pruebas de renovación y reverso para cada pago, top-up y suscripción, bloqueos de plan, tenant A/B, rollback y colas atascadas; backups verificables.
- **P1:** Cuotas transaccionales completas CRM, Catálogo, Agenda, Ads, Analytics, publicaciones, almacenamiento e importaciones. En esta etapa se endurecieron workflows/mensajería seleccionados; **NO es control integral completo de los 31 grupos**.
- **P1:** Recuperación controlada de tareas `processing/sending/publishing`, observabilidad y alertas; evitar dobles cobros.

Los archivos `audits/*V2722.*` se conservan como registro histórico de V27.22. `audits/ESTADO_V2723.csv` identifica avance de esta etapa sin inflar su cobertura.
