# Worktic AI Social Hub — Multiempresa, permisos y analítica avanzada

## Cambios principales

- El Social Hub ahora utiliza el `tenant_id` real del usuario autenticado, no su `user_id`.
- Todos los miembros de una empresa comparten conexiones, calendario, publicaciones y métricas.
- Las conexiones siguen totalmente aisladas entre empresas.
- Se añadió control de permisos por rol en backend y frontend.

## Matriz de permisos

| Rol | Ver | Borradores | Editar | Programar | Publicar | Conectar redes | Eliminar |
|---|---:|---:|---:|---:|---:|---:|---:|
| Propietario | Sí | Sí | Sí | Sí | Sí | Sí | Sí |
| Administrador | Sí | Sí | Sí | Sí | Sí | Sí | Sí |
| Supervisor | Sí | Sí | Sí | Sí | Sí | No | Sí |
| Asesor | Sí | Sí | Sí | No | No | No | No |

Los permisos se validan en el servidor. Ocultar botones en la interfaz no sustituye estas validaciones.

## Analítica avanzada

Nuevo endpoint:

- `GET /api/social/analytics?from=YYYY-MM-DD&to=YYYY-MM-DD&platform=instagram`

Incluye:

- alcance e impresiones;
- interacciones, engagement y CTR;
- clics, leads y conversiones;
- evolución diaria;
- embudo social;
- rendimiento por plataforma;
- mejores publicaciones;
- filtros por fecha y red.

Endpoint interno/administrativo para sincronización de métricas:

- `POST /api/social/metrics/ingest`

Solo propietario y administrador pueden usarlo. Los conectores oficiales o jobs de sincronización pueden persistir aquí los datos recuperados de las APIs.

## Nota sobre datos reales

La interfaz no inventa métricas. Cuando una plataforma todavía no haya concedido permisos de insights o el job de sincronización no haya recibido datos, los indicadores permanecen en cero.
