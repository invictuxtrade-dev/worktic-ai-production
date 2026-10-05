# WorkticAI V20 — Automation Engine

## Objetivo
V20 reemplaza la experiencia principal de reglas simples por un motor de workflows visual y orientado a eventos, manteniendo `worktic_auto_rules` únicamente como compatibilidad heredada.

## Arquitectura nueva
- `automation_workflows`: definición multitenant del flujo, trigger, grafo, versión y métricas.
- `automation_events`: cola persistente de eventos de negocio.
- `automation_executions`: ejecución auditable de cada workflow.
- `automation_execution_steps`: detalle por nodo, salida y error.
- Worker persistente que consume eventos y ejecuta workflows activos.
- Adaptador V19: `lead_automation_outbox` -> `automation_events`.

## Disparadores preparados
- manual
- lead.created
- message.received
- appointment.created
- opportunity.stage_changed
- form.submitted
- social.comment

`lead.created` ya queda conectado automáticamente con Meta Lead Ads V19.

## Nodos / acciones V20
- Condición / filtro
- Espera controlada
- WhatsApp Cloud (texto)
- Tarea IA
- Etiquetar contacto CRM
- Mover oportunidad de pipeline
- Asignar responsable
- Crear tarea (registro de intención; se enlazará al gestor dedicado)
- Webhook (reservado para hardening de salida HTTP en V26)

## Seguridad y operación
- Workflows aislados por `tenant_id`.
- Solo owner/admin pueden crear, editar, activar, pausar o eliminar.
- Ejecuciones y pasos quedan registrados.
- Los errores no se ocultan: quedan en historial y métricas.
- La salida webhook externa no se ejecuta todavía para evitar abrir SSRF/salidas arbitrarias antes del hardening de producción.

## Compatibilidad
- `worktic_auto_rules` no se elimina.
- Las reglas simples antiguas siguen pudiendo actuar en conversaciones heredadas.
- La UI principal muestra Automation Engine V20.

## Nota de escalabilidad
El worker V20 funciona con la base SQLite actual y una sola instancia de Render. En V26 se migrará la cola a Redis/worker dedicado y PostgreSQL para escalado horizontal.
