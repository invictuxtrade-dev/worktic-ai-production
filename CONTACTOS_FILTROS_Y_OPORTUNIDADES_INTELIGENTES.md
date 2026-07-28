# Contactos compactos y Oportunidades inteligentes

## Contactos
- Barra de filtros compacta y responsive.
- En escritorio utiliza una sola fila cuando hay espacio.
- En resoluciones menores se reorganiza en 4, 2 o 1 columnas sin desbordarse.
- No se modificó la lógica del CRM ni el JavaScript estable del módulo.

## Oportunidades
- Nuevo backend multi-tenant para el pipeline.
- Creación y actualización automática desde:
  - Formularios y landing pages.
  - Conversaciones de WhatsApp, Telegram y Messenger con intención comercial.
  - Cambios del contacto a Interesado, Calificado, Cotización, Negociación, Ganado o Perdido.
- Detección de señales como precio, cotización, compra, pago, agenda, demo y contratación.
- Evita duplicar oportunidades abiertas para el mismo contacto.
- Conserva la creación manual.
- Sincroniza la etapa entre contacto y oportunidad.
- Migración de oportunidades y datos históricos existentes.
- Aislamiento completo por tenant_id.
- Eliminación lógica para no borrar contactos ni conversaciones.

## Interfaz del pipeline
- Estadísticas de oportunidades abiertas, calificadas, valor y origen automático.
- Filtros por búsqueda, etapa, origen y canal.
- Orden por actividad, puntuación, valor o antigüedad.
- Tablero por etapas con drag and drop.
- Puntuación comercial y probabilidad por etapa.
- Contacto relacionado mediante selector, no por ID manual.
- Botones para avanzar, editar y eliminar.

## Archivos principales agregados
- `crm_opportunities_premium.go`
- `static/opportunities-premium.js`
- `static/crm-flow-premium.css`

Los archivos centrales `static/app.js`, `static/styles.css`, `static/contacts-premium.js` y el Landing Builder permanecen idénticos a la base aprobada.
