# Worktic AI — Social Hub Premium

## Incluido en esta entrega

- Nuevo módulo **Social Hub** integrado al menú principal.
- Dashboard editorial con redes conectadas, publicaciones programadas, publicadas y fallidas.
- Compositor multicanal con vista previa en tiempo real.
- Adaptación inicial del copy para Facebook, Instagram, LinkedIn, Telegram, TikTok y YouTube.
- Estados: draft, scheduled, queued, published y failed.
- Calendario editorial mensual.
- Biblioteca de publicaciones con filtros y acciones.
- Gestión de conexiones sociales aisladas por tenant.
- Modo sandbox para probar el flujo completo sin enviar contenido a redes reales.
- Base de datos para conexiones, grupos de contenido, publicaciones y métricas.
- API REST para overview, conexiones, publicaciones y despacho.
- Diseño responsive premium.

## Importante sobre publicación real

El modo sandbox simula correctamente el despacho y permite validar UX, datos y flujo completo. Para publicar en redes reales se deben implementar los adaptadores oficiales OAuth/API de cada proveedor y aprobar los permisos correspondientes.

Prioridad recomendada:

1. Meta OAuth + Facebook Pages + Instagram Graph API.
2. Telegram Bot API para canales autorizados.
3. LinkedIn OAuth y Posts API.
4. TikTok Content Posting API.
5. YouTube Data API.

## Archivos principales

- `socialhub.go`: esquema, modelos y endpoints.
- `static/app.html`: interfaz Social Hub.
- `static/app.js`: compositor, calendario, conexiones, biblioteca y analítica.
- `static/styles.css`: diseño premium y responsive.

## Prueba rápida

1. Inicia Worktic AI normalmente.
2. Abre **Social Hub**.
3. Ve a **Conexiones** y crea una red en modo Sandbox.
4. Abre **Compositor IA**.
5. Escribe contenido, selecciona la red y guarda o publica.
6. Revisa Biblioteca, Calendario y Vista general.

## Validaciones realizadas

- `node --check static/app.js`: correcto.
- `gofmt`: correcto.
- La compilación Go no se ejecutó porque el proyecto exige Go 1.24 y el entorno disponible tiene Go 1.23.2 sin acceso de red para descargar el toolchain.
