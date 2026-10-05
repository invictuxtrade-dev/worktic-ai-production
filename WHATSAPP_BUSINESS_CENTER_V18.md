# WorkticAI V18 — WhatsApp Business Center

V18 convierte la integración WhatsApp Cloud de V16/V17 en un módulo operativo dentro de WorkticAI.

## Navegación

Nuevo menú: **WhatsApp Business**

Pestañas:
- Resumen
- Número & Calidad
- Plantillas
- Contactos
- Analítica
- Operación
- Configuración

## Principios

- No reemplaza Inbox: lo reutiliza.
- No reemplaza Automatizaciones: las reutiliza.
- No reemplaza Growth & Campañas: las reutiliza.
- No elimina WhatsApp QR.
- Todo está aislado por `tenant_id` y `connection_id`.

## Nuevos endpoints

- `GET /api/whatsapp/business/overview?connection_id=`
- `GET /api/whatsapp/business/contacts?connection_id=`
- `GET /api/whatsapp/business/analytics?connection_id=&days=`
- `GET|POST|PUT|DELETE /api/whatsapp/templates/drafts`
- `POST /api/whatsapp/templates/submit`
- `POST /api/whatsapp/templates/send-test`
- `DELETE /api/whatsapp/templates/delete`
- `POST /api/whatsapp/templates/preview`

## Template Builder

Soporta:
- Categoría MARKETING / UTILITY / AUTHENTICATION.
- Idioma.
- Header de texto opcional.
- Body.
- Variables numeradas.
- Ejemplos para variables del Body.
- Footer.
- Botones Quick Reply, URL y Phone Number.
- Borrador local.
- Envío a Meta.
- Estado obtenido desde Meta.

## Analítica

La analítica local muestra únicamente datos registrados en Worktic:
- mensajes entrantes;
- mensajes salientes;
- entregados;
- leídos;
- histórico por día.

No se inventan métricas que Meta no haya enviado o Worktic no haya almacenado.
