# WorkticAI V23 — Social Hub Advanced

> **Nota V23.1:** el flujo de aprobación descrito en V23 sigue disponible, pero desde V23.1 es opcional. Social Hub puede operar en Autopilot, Híbrido o Aprobación según la política de cada tenant. Ver `SOCIAL_GOVERNANCE_AUTOPILOT_V23_1.md`.

## Objetivo

V23 convierte Social Hub en un centro profesional de operación social multired sin duplicar CRM, Inbox, Automatizaciones ni Agentes IA. Esta versión se construye de forma acumulativa sobre V22 y **todavía no debe desplegarse de forma aislada**; será parte de la actualización final única.

## Estructura funcional

Social Hub conserva las áreas V15 y añade cuatro espacios avanzados:

- **Multimedia**: biblioteca central de imágenes y videos por tenant.
- **Reutilizar IA**: convierte una pieza existente o un texto maestro en variantes por red/formato.
- **Aprobaciones**: flujo editor → revisor → publicar/programar/borrador.
- **Community**: comentarios oficiales, sugerencia de respuesta con IA, respuesta y resolución.

El Composer existente se amplía con soporte multirecurso y opción **Enviar a aprobación**.

## 1. Biblioteca multimedia

Nueva API: `/api/social/media`

Funciones:

- Indexa automáticamente archivos de `social_uploads/<tenant_id>`.
- Registra cada nueva subida del Social Hub en la biblioteca.
- Permite añadir un recurso por URL pública externa.
- Filtra por tipo y búsqueda.
- Permite reutilizar el recurso directamente en Composer.
- Eliminar de la biblioteca quita el registro, no borra el archivo físico del disco.
- Aislamiento obligatorio por `tenant_id`.

Tabla creada automáticamente:

- `social_media_assets`

## 2. Composer multiformato y carruseles reales

El modelo de publicaciones ahora acepta `mediaURLs` además del `mediaURL` legado. Los recursos se guardan dentro de `media_json`, manteniendo compatibilidad hacia atrás.

El Composer puede manejar hasta 10 recursos por pieza y valida la compatibilidad antes de guardar/publicar.

### Matriz de publicación V23

| Red | Pieza individual | Video/Reel | Multi-asset / carrusel | Estado V23 |
|---|---|---|---|---|
| Facebook | Sí | Sí | Sí, carrusel de imágenes | Oficial implementado |
| Instagram | Sí | Sí, Reels | Sí, 2–10 imágenes/videos | Oficial implementado |
| TikTok | Foto/video | Direct Post video | Sí, photo post/carrusel | Oficial implementado sujeto a capacidades/permisos de TikTok |
| Telegram | Texto/foto | Video | Sí, `sendMediaGroup` 2–10 | Implementado |
| YouTube | — | Un video por publicación | No | Implementado para video individual |
| LinkedIn | Texto/link | El flujo existente se conserva | Multi-asset no se publica automáticamente en V23 | Limitación explícita, sin simulación falsa |

Para LinkedIn y YouTube, el Composer bloquea combinaciones que no soporta este proveedor en V23 en vez de reportarlas falsamente como publicadas.

## 3. TikTok Direct Post actualizado

Antes de un Direct Post, Worktic consulta `creator_info` para obtener las capacidades/opciones vigentes del creador. Después:

- Video/Reel: inicializa publicación mediante endpoint de video.
- Foto/carrusel: inicializa publicación de contenido con `photo_images`.
- Los assets deben ser URLs públicas válidas/permitidas por TikTok.
- La privacidad se elige a partir de las opciones devueltas por el creador, prefiriendo público cuando está disponible.

La publicación pública desde una app para clientes externos sigue dependiendo de los requisitos, revisión/auditoría y permisos vigentes de TikTok.

## 4. Reutilizar contenido con IA

Nueva API: `/api/social/repurpose`

Permite partir de:

- un grupo/publicación existente, o
- texto maestro escrito por el usuario.

Se seleccionan destinos y formatos, por ejemplo:

- Facebook post
- Instagram post/reel/carrusel
- TikTok video/foto
- YouTube Short
- LinkedIn post

La IA devuelve variantes independientes que pueden enviarse al Composer para revisión humana. El prompt prohíbe inventar métricas, resultados, testimonios o datos no aportados.

Tabla de auditoría:

- `social_repurpose_jobs_v23`

## 5. Aprobaciones de equipo

Nueva API: `/api/social/approvals`

Flujo:

1. Editor prepara contenido.
2. Selecciona **Enviar a aprobación**.
3. El contenido queda `pending_approval` y el scheduler no lo publica.
4. Usuario con permiso de publicación revisa.
5. Puede aprobar o rechazar, dejando nota.
6. Si aprueba, Worktic ejecuta la intención original:
   - publicar,
   - programar,
   - o conservar como borrador aprobado.
7. Si rechaza, vuelve a borrador.

Tabla:

- `social_approvals`

Esto evita que un usuario editor publique directamente cuando el equipo exige revisión.

## 6. Community

Nueva API: `/api/social/community`

V23 sincroniza comentarios oficiales de publicaciones ya publicadas en:

- Facebook
- Instagram
- YouTube

Funciones:

- sincronizar comentarios bajo demanda;
- deduplicar por red + ID externo + tenant;
- filtrar por red y estado;
- sugerir respuesta con IA;
- responder oficialmente;
- marcar resuelto;
- abrir la publicación original;
- disparar eventos hacia Automation Engine V20.

Eventos generados:

- `social.comment`
- `social.comment.replied`

La sincronización es manual en V23 para reducir consumo innecesario de API/rate limits. En una fase de workers/producción podrá evolucionar a sincronización programada controlada.

Tabla:

- `social_comments_v23`

### Respuestas oficiales V23

- Facebook: implementadas.
- Instagram: implementadas.
- YouTube: implementadas.
- TikTok/LinkedIn/Telegram: no se presentan como disponibles desde Community en V23 hasta completar un flujo oficial específico para cada proveedor.

## 7. Permisos OAuth añadidos

### Meta Facebook/Instagram

El OAuth acumulativo ahora solicita además:

- `pages_manage_engagement`
- `instagram_manage_comments`

Se conservan los ya acumulados, incluyendo:

- `pages_show_list`
- `pages_read_engagement`
- `pages_manage_posts`
- `pages_manage_metadata`
- `leads_retrieval`
- `instagram_basic`
- `instagram_content_publish`
- `business_management`

**Implicación para despliegue final:** las cuentas Facebook/Instagram conectadas antes de V23 deberán reconectarse/reautorizarse para que Worktic reciba los nuevos permisos concedidos por Meta. La disponibilidad final también depende del App Review/Advanced Access que corresponda a la app de producción.

### YouTube

OAuth acumulativo ahora solicita:

- `youtube.upload`
- `youtube.readonly`
- `youtube.force-ssl`

`youtube.force-ssl` se utiliza para operaciones de comentarios/respuestas.

**Implicación:** canales YouTube conectados antes de V23 deberán volver a autorizarse para conceder el scope nuevo.

## 8. Integración con otras fases

V23 no crea motores paralelos:

- **CRM** sigue siendo el origen comercial.
- **Inbox V21** sigue siendo la bandeja de conversaciones privadas.
- **Agentes IA V22** mantienen memoria/conocimiento.
- **Automation Engine V20** recibe eventos Community.
- **Social Hub** se concentra en contenido orgánico, calendario, publicación, colaboración y community management.

## 9. Base de datos

Se crean automáticamente al iniciar:

- `social_media_assets`
- `social_approvals`
- `social_comments_v23`
- `social_repurpose_jobs_v23`

No se requiere SQL manual para V23.

## 10. Variables de entorno

**V23 no agrega variables de entorno nuevas.**

Los cambios de esta fase son scopes/permisos OAuth y funcionalidad del código.

## 11. Validación ejecutada en el paquete

- `static/social-v23.js`: sintaxis JavaScript válida.
- `static/app.js`: sintaxis JavaScript válida.
- HTML: cero IDs duplicados.
- Inclusión V23 CSS/JS comprobada.
- Archivos Go modificados procesados correctamente por `gofmt`.
- Rutas V23 verificadas.

No se pudo completar `go test ./...` en este entorno porque el proyecto requiere Go 1.24+ y el entorno disponible no puede descargar el toolchain/dependencias necesarias. No se detectó un fallo de V23 a partir de esa limitación; el build completo deberá formar parte del checklist final en Render/CI con Go 1.24+.

## 12. Estado

V23 es la nueva base acumulativa del desarrollo. **No desplegar todavía.** La próxima fase prevista es V24 — Analytics + Attribution, que consolidará el recorrido contenido/campaña → lead → conversación → cita → oportunidad → venta.
