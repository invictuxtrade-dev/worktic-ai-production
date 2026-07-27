# Worktic AI v2.0 Premium Enterprise — Stable Release

## Correcciones principales

- Respuesta automática de WhatsApp conectada al motor IA.
- Si el agente especializado asignado fue eliminado, está inactivo o pertenece a otro tenant, el canal usa automáticamente el Asistente Principal.
- Mensajes claros en `last_error` cuando falta `OPENAI_API_KEY` o no existe ningún asistente activo.
- Trazas `[WA-AI]` para recepción, generación y envío.
- Menú móvil real en la landing: hamburguesa, panel lateral, overlay, idioma, acceso, registro y documentación.
- Sistema visual premium para formularios: espaciado, etiquetas, ayudas, inputs, textareas, foco, modales y responsive.

## Prueba recomendada

1. Configura `OPENAI_API_KEY` en Render.
2. Activa el Asistente Principal.
3. En Canales, asigna un agente activo o deja la conexión sin agente para usar el principal.
4. Envía un mensaje desde otro número de WhatsApp.
5. Revisa los logs de Render buscando `[WA-AI]`.
