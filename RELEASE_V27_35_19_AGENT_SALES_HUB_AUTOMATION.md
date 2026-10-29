# WorkticAI V27.35.19 — Agentes IA + Sales Hub Automation

## Objetivo

Conectar los agentes especializados de WhatsApp (sesión), Telegram, Messenger y Chat Web con el **mismo Sales Hub de Oportunidades V27.35.18**, sin crear CRM duplicado. No se cambia la apariencia general ni el menú clásico.

## Cambios implementados

1. **Configuración individual en Agentes IA → Capacidades V22:** activar/desactivar la automatización, registrar actividad, crear seguimiento, permitir avance de etapa y escoger 4, 12, 24, 48 o 72 horas. Desactivado por defecto para todos los agentes.
2. **Permisos efectivos:** requiere un agente activo de la empresa, CRM habilitado en capacidades V22 y la herramienta `opportunity`. Las tareas requieren adicionalmente `tasks_enabled`; la actualización de etapas, `pipeline_enabled`.
3. **Detección conservadora:** acciones internas solo ante señales explícitas como «me interesa», «cotización», «quiero comprar», «cómo pago». No actúa por palabras vagas como «precio» o «producto» aisladas.
4. **Oportunidades:** relaciona el contacto existente del canal con una oportunidad abierta; crea una mediante el mecanismo CRM existente si no hay una abierta y el agente está habilitado. No actúa si no encuentra contacto identificable.
5. **Seguimiento interno:** cuando el agente tiene permiso, crea una tarea pendiente con vencimiento configurable. Respeta al responsable asignado o crea la tarea sin asignación si no existe un usuario activo. Solo una tarea **abierta** por oportunidad y agente.
6. **Historial y etapas:** registra un evento de origen Agente IA sin almacenar de nuevo el mensaje completo. El avance automático de etapa está apagado por defecto y respeta `manual_lock`; nunca cierra como «Ganada» o «Perdida».
7. **Idempotencia y aislamiento:** la acción se identifica por empresa, agente, canal, conversación y huella del mensaje; comprueba el tenant en cada consulta y escritura.
8. **Compatibilidad:** tres tablas nuevas aditivas para configuraciones, mensajes procesados y relación de tarea de seguimiento. Utiliza tipos SQL compatibles con SQLite y PostgreSQL. No modifica esquema ni datos del catálogo o del libro financiero.

## Limitaciones explícitas

- **No envía mensajes ni llamadas de seguimiento en segundo plano.** Los vencimientos se registran y aparecen en Sales Hub; el contacto posterior corresponde a un asesor hasta que se integre una cola de envíos por canal, consentimiento comercial, políticas de ventanas de mensajería, templates y auditoría de entregas.
- **No confirma pagos ni toca inventario.** Un agente no puede acreditar dinero por recibir una foto o hash.
- **No carga comprobantes automáticamente** desde los canales: sigue pendiente procesar archivos entrantes, vincularlos con el pedido correcto y ponerlos en revisión.
- **No abre el hilo exacto de conversación** desde la ficha del CRM: se mantiene la navegación del módulo de Sales Hub V27.35.18.
- **No añade IA libre para modificar cualquier campo de CRM:** opera mediante decisiones acotadas y reglas verificables, con permisos por agente.
- **No implementa aún WhatsApp Cloud como disparador independiente**: se conectan los cuatro puntos de respuesta especializada ya existentes en el código; debe verificarse el flujo Cloud antes de declarar cobertura universal.

## Pruebas obligatorias

- `node --check static/agents-v22.js`
- `go test -count=1 ./...`
- `go build ./...`
- `git diff --check`
- Comprobar permisos ON/OFF, ausencia de duplicados, cuentas de dos empresas y agentes inactivos; revisar la tarea e historial en Sales Hub.
- Antes del despliegue de tablas nuevas: **respaldo de PostgreSQL** y verificación en una empresa de pruebas.

## Puesta en marcha

Instalar el parche sobre la **versión V27.35.18** validada. Ejecutar `INSTALAR_AGENT_SALES_HUB_V273519.ps1` y detenerse si se encuentra una diferencia en archivos existentes o fallan pruebas. No publicar sin la validación completa de Go 1.26 en Windows y la prueba real del backend PostgreSQL.


## Corrección de compilación (revisión 2026-10-29)
- Se corrigió la reasignación de la variable `e` en `agent_sales_hub_v273519.go` (antes `:=`, ahora `=`), que impedía compilar con Go.
- El instalador mantiene la validación y el respaldo automático.
- Es necesario validar en Windows con Go 1.26 y posteriormente en PostgreSQL; no publicar si falla una prueba.
