# Administración Worktic Full

Base utilizada: `Worktic_AI_Catalogo_Tarjetas_Imagenes_Uniformes.zip`.

## Usuarios

- Buscador por nombre, correo y empresa.
- Filtros por estado, rol y plan.
- Paginación del lado del servidor.
- Creación y edición de usuarios.
- Cambio opcional de contraseña con cierre de sesiones.
- Bloqueo y desbloqueo inmediato.
- Eliminación segura (soft delete) y restauración.
- Visualización de empresa, rol, licencia y vencimiento.

## Licencias de cortesía

- Selección de cualquier usuario mediante buscador.
- Aplicación automática a la cuenta propietaria de la empresa.
- Selección de cualquier plan disponible.
- Periodo por cantidad de días o fecha exacta de vencimiento.
- Nota administrativa.
- Historial, extensión y cancelación de licencias.

## Planes

- Buscador y filtros por estado.
- Creación y edición completa.
- Eliminación segura y restauración.
- Conservación de las licencias existentes hasta su vencimiento.
- El plan `free` se protege porque es el respaldo automático del sistema.

## Pagos

- Buscador por usuario, correo, hash o wallet.
- Filtros por estado, red, plan y fechas.
- Paginación del lado del servidor.
- Resumen de pendientes, aprobados, rechazados y ventas.
- Flujo existente de aprobación y rechazo conservado.

## Archivos nuevos

- `admin_full.go`
- `static/admin-full.js`
- `static/admin-full.css`

Los módulos existentes de Agenda, Catálogo, Contactos, Conversaciones, Oportunidades y Landing Pages no fueron reestructurados.
