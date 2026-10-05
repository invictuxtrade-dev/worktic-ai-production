# WorkticAI V21 — Inbox Omnicanal definitivo

## Objetivo
V21 convierte Conversaciones en una central operativa conectada con CRM, Agentes IA y Automation Engine V20.

## Funciones implementadas
- Bandeja multi-canal para WhatsApp QR, WhatsApp Cloud, Messenger y Telegram.
- Filtros por canal, estado, modo de atención, fechas, texto, no leídos y prioridad.
- KPIs de abiertas, pendientes, cerradas, atención humana y no leídos.
- Handoff IA ↔ humano por conversación.
- Cuando una conversación está en modo humano, los auto-replies IA de WhatsApp QR, Messenger y Telegram se detienen para ese chat.
- Estados: open, pending, closed.
- Prioridades: low, normal, high, urgent.
- Asignación de responsable del mismo tenant.
- Departamento libre (Ventas, Soporte, etc.).
- Etiquetas de conversación.
- SLA con vencimiento configurable.
- Notas internas persistentes.
- Envío manual desde Inbox por WhatsApp QR, WhatsApp Cloud, Messenger y Telegram.
- WhatsApp Cloud guarda también los mensajes salientes enviados desde Inbox.
- Aislamiento multi-tenant reforzado en listado de conversaciones, mensajes, lectura y edición de contacto.

## Eventos conectados con V20
V21 inserta eventos en `automation_events`:
- `message.received`
- `conversation.assigned`
- `conversation.mode_changed`
- `conversation.open`
- `conversation.pending`
- `conversation.closed`

Esto permite crear workflows que reaccionen a mensajes, asignaciones y cambios operativos.

## Persistencia nueva
- `inbox_conversations`
- `inbox_notes`
- `inbox_departments`

Las tablas se crean automáticamente al iniciar la aplicación.

## Canales futuros
La arquitectura de Inbox ya es genérica. Instagram DM y Web Chat deben entrar como proveedores/canales en una fase de conectores, sin rehacer la bandeja. V21 no simula recepción por esos canales si todavía no existe el conector oficial correspondiente.

## Variables de entorno
V21 no añade variables de entorno nuevas.
