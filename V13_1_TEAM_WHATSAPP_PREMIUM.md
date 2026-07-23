# Worktic AI V13.1 — Team WhatsApp Premium

## Incorporado

- Invitaciones de equipo por WhatsApp con enlace seguro, token de un solo uso y vencimiento de 72 horas.
- El usuario invitado crea su contraseña y entra directamente al tenant de la empresa.
- Control de cupos activos + invitaciones pendientes según `max_users` del plan.
- Estados de invitación: pendiente, aceptada y cancelada.
- Edición, suspensión y eliminación de integrantes con validaciones del backend.
- Formularios visuales para contactos, oportunidades, canales, grupos, revisión de pagos e invitaciones.
- Confirmaciones profesionales en lugar de cuadros nativos del navegador.
- CRUD ampliado para contactos, oportunidades, citas, automatizaciones y miembros.
- Renovación visual premium en tablas, modales, estados vacíos, tarjetas y versión móvil.

## Seguridad

- La invitación está vinculada al `tenant_id` del propietario.
- El token solo puede utilizarse una vez.
- El enlace expira a las 72 horas.
- El integrante no recibe una licencia independiente; utiliza la licencia de la empresa.
- El backend valida el límite de usuarios antes de crear la invitación y al administrar el equipo.

## WhatsApp

Worktic genera una URL `wa.me` con un mensaje prellenado. El propietario revisa y envía la invitación desde su WhatsApp. Esto evita envíos automáticos no autorizados y funciona incluso si todavía no conectó un canal de WhatsApp dentro de Worktic.
