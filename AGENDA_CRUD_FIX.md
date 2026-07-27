# Agenda CRUD Fix

Correcciones incluidas:

- Edición y eliminación de bloques de disponibilidad.
- Edición y eliminación de profesionales.
- Edición y eliminación de servicios.
- Guardado de configuración con manejo visible de errores.
- Citas editables conservando su horario actual.
- Validación de cruces al actualizar una cita, excluyendo la propia cita.
- Uso del intervalo, anticipación mínima, días máximos y fines de semana configurados.
- Horarios específicos por profesional reemplazan el horario general para ese día.
- Validación de bloqueos de agenda y buffers de servicios.
- Fechas de agenda almacenadas como hora local de la zona configurada para evitar desplazamientos UTC en la interfaz.
- Permisos de gestión para owner, admin, superadmin y supervisor.

Verificaciones realizadas:

- `static/app.js` validado con `node --check`.
- Archivos Go formateados con `gofmt`.
- No fue posible ejecutar `go test` en este entorno porque el proyecto exige Go 1.24 y el entorno local dispone de Go 1.23.2 sin acceso para descargar el toolchain.
