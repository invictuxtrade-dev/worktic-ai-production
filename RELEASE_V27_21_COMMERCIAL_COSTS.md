# WorkticAI V27.21 — Commercial Credits + Core AI Entitlements

**Base:** V27.20 Creative Cost Guard (que se construyó sobre V27.19).
**ESTADO:** laboratorio / staging. **NO PUBLICAR** en GitHub ni Render mientras no se completen pruebas integrales.
**Arquitectura:** extensión aditiva de WorkticAI, no reemplaza la base existente.

## Objetivo

Cumplimiento real de lo contratado y separación de dos consumos:

1. **Respuestas IA generales**: Copilot, agentes, respuestas automáticas de WhatsApp (oficial/legacy de un solo tenant), Telegram, Messenger, Social Hub, automatizaciones y Partners Autopilot. Asignación mensual por empresa. No son créditos comprables en esta fase.
2. **Créditos creativos**: imágenes, textos de Studio, narraciones IA y video Luma. Se reservan créditos incluidos y luego comprados ANTES de llamar al proveedor. No cobran dos veces una respuesta IA general.

En ambos motores las estimaciones de costo son conservadoras y se cargan al registro de gasto técnico, **no son costos facturados verificados**.

## Matriz de planes — fase piloto

Precios existentes en `main.go`; confirmar valores vigentes `billing_plans` en PostgreSQL, que pueden variar por edición del admin.

| Derecho | Free | Personal | Negocio | Empresa |
|---|---:|---:|---:|---:|
| Precio predeterminado / mes USDT | 0 | 25 | 75 | 150 |
| Usuarios predeterminados | 1 | 1 | 5 | 15 |
| Canales predeterminados | 1 | 2 | 3 | 6 |
| CRM contactos predeterminados | 100 | 500 | 3000 | 10000 |
| Productos predeterminados | 5 | 100 | 1000 | 5000 |
| Reglas de automatización predeterminadas | 1 | 10 | 50 | 250 |
| Respuestas IA generales* | 0 | 150 | 500 | 1500 |
| Créditos creativos / mes | 0 | 30 | 120 | 300 |
| Proyectos Studio | 3 | 30 | 150 | 1000 |
| Almacenamiento Studio+Galería | 100MB | 1GB | 5GB | 20GB |
| Imagen IA, video IA, voz IA | No | Sí, con créditos | Sí, con créditos | Sí, con créditos |
| Editor manual | Sí | Sí | Sí | Sí |

* Los cupos piloto son un máximo adicional (min con lo configurado en `billing_plans`). En modo `WORKTIC_COMMERCIAL_ENABLE=0` los cupos históricos no se modifican. El módulo de administración histórico puede mostrar los valores originales hasta que se haga la migración comercial final.

Cuotas mensuales (UTC) no acumulables. Créditos adquiridos en saldo separado y no se recargan automáticamente. Todos los derechos son por empresa/tenant. No se anuncian tareas ilimitadas.

## Costo del proveedor estimado y crédito

| Studio | Créditos | Reserva USD aprox | Reservado por crédito máx |
|---|---:|---:|---:|
| Imagen estándar con OpenAI (calidad medium) | 4 | $0.08 | $0.020 |
| Texto / guion con IA | 2 | $0.05 | $0.025 |
| Narración IA, por 1.000 caracteres | 5 | $0.10 | $0.020 |
| Video Luma 720p 5 segundos | 15 | $0.36 | $0.024 |
| Video Luma 720p 10 segundos | 45 | $0.99 | $0.022 |
| Respuesta IA general: modelo limitado `gpt-4o-mini`, hasta 512 tokens | cuota separada | $0.01 | N/A |

Tarifas simuladas en **centavos de dólar**: `creativeQuote` y `commercialCreditsQuote`. Si una configuración del proveedor exige costo superior a US$0.025/crédito, la operación se bloquea hasta que el administrador ajuste tarifario. Esto evita subvenciones ilimitadas, no garantiza beneficios sin conocer facturas y costos ajenos.

## Paquetes opcionales con verificación manual

| Paquete | Créditos | Precio USDT |
|---|---:|---:|
| Mini | 30 | 3.50 |
| Creator | 100 | 10 |
| Growth | 300 | 27 |
| Pro Studio | 1000 | 80 |
| Agency | 3000 | 210 |

No están a la venta: `WORKTIC_COMMERCIAL_ENABLE=0` por defecto. No se acreditan créditos por hash proporcionado por usuario. Un superadmin verifica comprobante, red, monto, destinatario y confirmaciones fuera del chat. Solo entonces puede aprobar y acreditar **una sola vez**. No conecta aún oráculos/cadena en tiempo real; ello sigue pendiente.

## Configuración SOLO PARA STAGING

```
WORKTIC_COMMERCIAL_ENABLE=0
WORKTIC_COST_GLOBAL_MONTHLY_CENTS=3000
WORKTIC_COST_BUDGET_PERSONAL_CENTS=225
WORKTIC_COST_BUDGET_NEGOCIO_CENTS=800
WORKTIC_COST_BUDGET_EMPRESA_CENTS=2250
WORKTIC_COST_IMAGE_CENTS=8
WORKTIC_COST_TEXT_CENTS=5
WORKTIC_COST_TTS_1000_CENTS=10
WORKTIC_COST_VIDEO5_CENTS=36
WORKTIC_COST_VIDEO10_CENTS=99
WORKTIC_COST_CORE_RESPONSE_CENTS=1
WORKTIC_CORE_MODEL=gpt-4o-mini
WORKTIC_CORE_MAX_OUTPUT_TOKENS=512
```

`WORKTIC_COMMERCIAL_ENABLE=0`: solo la interfaz y guardia V27.20 (si existen presupuestos); no se inician cobros comerciales ni el nuevo cupo Core AI. Para activar EN STAGING después de pruebas, `WORKTIC_COMMERCIAL_ENABLE=1` y asegurarse de que las variables de presupuesto y las APIs de Luma/OpenAI están configuradas en esa instancia, NO en producción.

El techo global de US$30 aquí es únicamente para el laboratorio: con varios clientes vendidos sería insuficiente para honrar todas las generaciones incluidas y causaría bloqueos. Para comercializar debe dimensionarse según el número real de suscripciones y recargas, con fondos disponibles del proveedor y alarmas.

## Integraciones desarrolladas

- `commercial_credits_v2721.go`: derechos de Studio, billetera, paquetes, reservas atómicas, compra pendiente y aprobación superadmin, límites de almacenamiento/proyectos, conciliación básica de error reembolsable Luma.
- `commercial_core_ai_v2721.go`: cuota de respuestas generales, reserva concurrente, reportes mediante `usage_monthly`.
- `creative_cost_guard_v2720.go`: presupuesto global/tenant, reserva de proveedor y bloqueo previo.
- `agents.go`, `channels_v13.go`, `telegram_channels.go`, `messenger_channels.go`, `social_content_ai.go`, `social_advanced_v23.go`, `automations_v20.go`, `partners_v2715.go`, `growth_command_v27.go` y ruta legacy en `main.go`: intercepción para cuota Core AI.
- `studio_v2711.go`: genera texto ya facturado con créditos creativos, NO lo cuenta nuevamente como IA conversacional.
- `static/commercial-v2721.{js,css}`: billetera y planes en Studio, panel administrativo de reglas/recargas.

## Cobertura y limitaciones, honestamente

| Módulo | Control de cuota/costo | Pendiente |
|---|---|---|
| Studio imágenes/videos/voz/texto | Créditos+presupuesto por proveedor+empresa | Probar la factura OpenAI/Luma real y cancelaciones |
| Copilot/agentes IA/Messenger/Telegram/WhatsApp moderno | Respuesta por tenant, estimado y cuota Core | Prueba real con usuarios, mensajes y proveedores |
| Copilot streaming | Reserva antes de emitir eventos, cuota Core | Ensayo real de desconexiones |
| WhatsApp legacy | Solo un tenant; bloquea en multiempresa sin identificación segura | Migrar completamente al servicio tenant-aware |
| CRM/contactos/productos/usuarios/canales | Límites originales de planes conservados, varios tenant-scoped | Revisión exhaustiva de TODOS los endpoints y carrera en cuotas |
| Automatizaciones | Cuota IA consumida en la acción AI; cupo de reglas original | Bloqueos de workflow/reintentos y fan-out |
| Social Hub/Ads Center | Coste de red/Ads normalmente en cuenta del cliente, límites de infraestructura por auditar | Verificación de cuotas/planes por proveedor y red |
| Email/SMS externo | No incluido | Tarifas, límites, facturas y responsable de pago por proveedor |
| Discos/transferencias/infraestructuras Render | Límite local de Galería/proyectos en Studio; costo no atribuido | CPU/RAM/DB/cache/transferencia y carga real |
| Pagos/anualidades/Partners | Mantiene el motor actual, no modifica compensación | Validar contrato real, descuentos efectivos, pasarela y refunds |
| Recargas USDT | Solicitud y aprobación manual superadmin | Comprobación autónoma de blockchain y conciliación contable |
| Biblioteca compartida | Cuota al subir/sincronizar rutas principales | Auditoría rutas adicionales de importación y medios remotos |

No se puede afirmar todavía que **toda WorkticAI esté costeada y libre de desfases**: sin instancias PostgreSQL/Render en staging y facturación externa, ninguna implementación puede verificar margen neto.

## Pruebas de aceptación imprescindibles

1. Compilar `go test ./...` y `go build ./...` con **Go 1.26 del usuario**; Go1.23 del sandbox no soporta el módulo completo.
2. Crear empresas de los cuatro planes en PostgreSQL de **staging**, comprobar permisos y respuesta `403` por funciones denegadas.
3. Iniciar 20 solicitudes concurrentes para 10 créditos disponibles y verificar **ningún saldo negativo ni doble consumo**.
4. 20 solicitudes simultáneas Copilot y agents con límite bajo, verificar cuota exacta, `usage_monthly` y presupuesto global. Simular respuestas con proveedor falso.
5. Generación Luma exitosa, fallida reembolsable, timeout, reinicio y polling repetido. No reembolsar fallos ambigüos.
6. Simular recarga misma transacción 2 veces; rechazo, reversión y saldo ya gastado.
7. Probar 100MB/1GB/5GB/20GB, varios proyectos y subir archivos de manera concurrente. Validar Render disk `/var/data` real.
8. Simular ingresos mensuales/anuales con máximo 50% Partners, descontar Core IA, Studio, Render, almacenamiento, tasas pasarela y soporte.
9. Auditar promesas de la web comercial y los precios/roles del admin antes de publicar, evitando contradicciones.
10. Pruebas de regresión Worktic Partners, CRM, Social Hub, WhatsApp, Copilot, Ads Center y agenda.

**No se ha hecho deploy. No aplicar nuevas migraciones manuales en producción.**
