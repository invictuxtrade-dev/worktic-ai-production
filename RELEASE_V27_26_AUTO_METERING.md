# WorkticAI V27.26 — Costeo operativo automático (STAGING ONLY)

**Base exacta:** `WorkticAI_V27.25_Cost_Accounting_Billing_Reconciliation_Base_V27.24.zip`. No hay conexión ni cambio a GitHub ni Render. La producción permanece en la versión anterior.

## Qué quedó conectado

- `agents.go`: respuestas OpenAI API, capturando `usage.input_tokens`, `usage.output_tokens`, ID y modelo cuando los devuelve el proveedor. Incluye los canales/agents que utilizan esa función central.
- `growth_command_v27.go`: Copilot streaming, captura del objeto `response` en el evento `response.completed`; los streams incompletos se registran sin asignarles un costo inventado.
- `studio_v2711.go`: 1 imagen OpenAI por solicitud, modelo, resolución; el texto de Studio utiliza su reserva de créditos propia y se atribuye a la empresa correcta, **sin doble débito de respuesta IA**.
- `studio_v2718.go`: caracteres de entrada de voz, modelo y estado de éxito/error.
- `studio_video_v2719.go`: duración, modelo, ID de generación de Luma, recepción de estados (`queued/processing/completed/failed`) y vínculo con reserva creativa; el estado no se cobra dos veces.
- `external_usage_v2723`: resumen superadmin de registros existentes de Meta/mensajería; siempre **costo desconocido** si no hay tarifa o factura acreditada.

## Datos y privacidad

Tabla aditiva `finance_operation_meter_v2726`: evento irrepetible, tenant, proveedor, operación, modelo, estado, referencia del proveedor, reserva, tokens de entrada/salida, unidades, tipo de unidad, **estimación opcional** en micro-USD y estado del precio. **NO** almacena prompts, textos, teléfonos, imágenes, credenciales o datos secretos. No representa cobro al cliente ni factura del proveedor. Los costes verificados siguen reservados a V27.25 tras comprobar evidencia externa.

## Variables de entorno

Consultar `METER_V2726_STAGING.env.example` (sin secretos).

- `WORKTIC_METER_V2726=0`: valor predeterminado. No registra eventos nuevos; compatible con V27.25.
- `WORKTIC_METER_V2726=1`: habilita registro operativo de solo escritura aditiva, en staging aislado.
- Si falta tarifa para el modelo exacto, el costo es `NULL` y `pricing_status=unpriced` (no US$0).
- Las tarifas configuradas en micro-USD son **estimaciones**, incluso cuando se capturen unidades reales; USD 1 = 1.000.000 micro-USD.
- Costos por tokens usando precio por 1M de tokens. Las entradas en caché y otras clases de uso pueden tener precios distintos; por tanto el resultado puede ser conservador e inexacto.
- Tarifa de TTS por carácter: indicador **aproximado**, no equivalente al costo real basado en tokens del proveedor.
- Luma: tarifa 720p/5 y 10 segundos únicamente para modelo `ray-3.2`, si está configurada; el trabajo puede fallar o generar crédito reembolsable, y requiere conciliación posterior.
- No se puede derivar el costo real de Meta por mensaje sin país, categoría y tarifa correspondiente; tampoco se inventa una tarifa para Ads ni infraestructura.

## APIs / interfaz

`GET /api/admin/finance/v2726/metering?period=AAAA-MM` únicamente para **superadministrador**. Devuelve grupos operativos, tokens y estimaciones opcionales y eventos externos sin tarifa. Se añade a la página administrativa V27.25. Ningún usuario ordinario ve tarifas mayoristas.

`GET /api/admin/finance/v2725/summary` y los demás endpoints de V27.25 quedan sin alterar; estos siguen siendo la fuente de facturas/atribuciones **verificadas**, no V27.26.

## Validación antes de autorizar nuevos planes

1. En VS Code sobre una carpeta nueva derivada de esta versión, usar Go **1.26** para `go test ./...` y `go build ./...`. No fue posible en el entorno de creación (Go 1.23).
2. Staging con PostgreSQL **separado** de producción: arrancar con el switch OFF, revisar migraciones aditivas, backup y rollback.
3. Activar `WORKTIC_METER_V2726=1`; probar una solicitud de respuesta, una imagen, una narración, un video y el streaming; cotejar unidades con paneles de proveedores y meter V27.26.
4. Validar multi-tenant y solicitudes concurrentes en PostgreSQL; revisar exactamente una fila por reserva y que se atribuya al tenant correcto.
5. Simular fallos de OpenAI/Luma; confirmar los estados `vendor_error`, `incomplete` o `submission_uncertain` sin equipararlos a reembolsos automáticos.
6. Importar facturas de laboratorio con revisión independiente mediante V27.25 y cotejar IDs y costos sin mezclar USD y USDT.
7. Confirmar cuotas, permisos y cobros del resto de módulos antes de activar comercialmente; mantener `WORKTIC_COMMERCIAL_*`, importación de facturas y presupuestos del proveedor desactivados para la producción hasta la auditoría completa.

## Límites reconocidos (NO es certificación del 100 %)

- No hay integración directa con facturación real de OpenAI/Luma/Meta/Render ni conciliación automática de extractos bancarios.
- Algunas rutas heredadas o SDK de terceros pueden llamar a IA sin pasar por el punto central: inventario adicional y pruebas de integración aún necesarios.
- Los cargos de redes sociales y publicidad, proveedores de mensajería, almacenamiento, Redis, PostgreSQL y Render necesitan asignaciones de costos verificadas.
- No se ha realizado prueba Go 1.26/PG real/Render/Meta/OpenAI/Luma de extremo a extremo ni carga productiva.
- La monetización y las limitaciones comerciales de V27.21-V27.24 siguen siendo pilotos protegidos por flags; no se publican aún.

El endpoint de medición también agrega operaciones por `tenant_id`, y presenta contadores de automatizaciones, Analytics/Ads de V27.24 y eventos externos. Se muestran como **consumo sin costo atribuido** hasta que existan facturas o un modelo de costos documentado, nunca como una deuda de US$0.
