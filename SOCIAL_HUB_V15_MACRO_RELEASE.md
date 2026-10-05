# WorkticAI Social Hub V15 — Macro Social Operating Center

Fecha: 2026-09-27

## Objetivo
Transformar Social Hub en el centro operativo multired de WorkticAI sin romper la persistencia, OAuth, publicación y analítica existentes.

## Cambios incluidos
- Nuevo hero y jerarquía visual de Social Hub.
- Selector global de red: Todas, Facebook, Instagram, TikTok, YouTube, LinkedIn y Telegram.
- El selector filtra Resumen, Contenido, Calendario y Cuentas.
- Resumen renovado con KPIs de alcance, interacciones, clics, leads y conversiones provenientes del endpoint real de analítica.
- Navegación interna reorganizada: Resumen, Composer, AI Content Studio, Calendario, Contenido, Analytics y Cuentas.
- Nuevo AI Content Studio conectado a `/api/social/generate`.
- AI Content Studio permite definir brief, objetivo, tono, producto, público y redes.
- Resultado IA puede enviarse al Composer preservando redes, texto, CTA y hashtags.
- Biblioteca de contenido convertida de tabla a tarjetas editoriales visuales.
- Cuentas/conexiones mejoradas con estado, proveedor, scopes y última sincronización.
- Calendario y contenido respetan el filtro de red activo.
- Analytics se sincroniza con el filtro global de red.
- Nueva composición responsive para tablet y móvil.

## Compatibilidad
No se modificó el esquema SQLite de Social Hub ni se eliminaron endpoints existentes. Se conservan las rutas actuales de publicaciones, conexiones, OAuth, publicación, analítica, generación IA y subida de medios.

## Validaciones realizadas
- `node --check static/app.js`: OK.
- HTML parseado correctamente y sin IDs duplicados: OK.
- 7 paneles internos de Social Hub detectados: OK.
- `go test ./...`: no ejecutable en el entorno de empaquetado porque Go intenta descargar el toolchain 1.24 desde proxy.golang.org y la red del entorno lo bloquea.

## Próxima etapa recomendada
1. WhatsApp Cloud API oficial / Meta Business Hub.
2. Biblioteca multimedia reutilizable.
3. Variantes IA específicas por plataforma en una misma publicación.
4. Comentarios e interacción social donde las APIs oficiales lo permitan.
5. Atribución social -> CRM -> oportunidad -> venta.
6. Workers/colas para programación a escala.
