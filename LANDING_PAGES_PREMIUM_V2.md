# Worktic AI — Landing Pages Premium V2

Esta versión profesionaliza tanto el constructor como la página pública sin modificar los módulos globales de Agenda, Catálogo, Conversaciones, Contactos u Oportunidades.

## Constructor aislado

- Nuevo archivo `static/landing-premium-v2.js`.
- Nuevo archivo `static/landing-premium-v2.css`.
- No se modificaron `static/app.js` ni `static/styles.css`.
- Cuatro pasos: contenido, multimedia, secciones y publicación.
- Vista previa visual en escritorio.
- Borrador local y guardado verificado conservados.

## Multimedia

- Imagen por URL o carga directa.
- JPG, PNG, WEBP o GIF, máximo 8 MB.
- Recomendación visual: 1600 × 1200 px; mínimo recomendado 1200 × 800 px.
- Ajuste `cover` o `contain`.
- Video mediante URL pública de YouTube, YouTube Shorts o Vimeo.
- Video en el hero o en una sección independiente.

## Contacto directo

- Detecta conexiones activas de WhatsApp, Telegram y Messenger.
- Detecta redes activas del Social Hub cuando tienen una URL pública posible.
- Permite elegir qué canales aparecen.
- WhatsApp acepta un mensaje inicial personalizado.
- Iconos dentro del hero, bloque de contacto, botones flotantes y CTA móvil.

## Landing pública

- Diseño completamente responsive.
- Hero profesional con multimedia en proporción estable.
- Estadísticas, beneficios, características, testimonios, video, contacto, formulario y preguntas frecuentes.
- Cuatro estilos: Aurora, Minimal, Bold y Midnight.
- CTA fija en móvil.
- Metadatos básicos para compartir la página.

## Backend

- Nueva columna `premium_json` con migración automática.
- Endpoint de canales: `/api/marketing/landings/channels`.
- Endpoint de carga: `/api/marketing/landings/upload`.
- Archivos públicos: `/uploads/landings/{tenant_id}/{archivo}`.
- Cada carga y consulta conserva el aislamiento por `tenant_id`.
