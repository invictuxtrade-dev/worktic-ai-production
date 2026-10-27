# WorkticAI V27.34.9 Lite — Recargas simplificadas

Base: V27.34.8. Reemplaza el formulario anterior con un flujo compacto: seleccionar paquete, consultar dirección oficial configurada mediante `/api/billing/config`, pegar hash y enviar solicitud. El cliente solo puede enviar si el servicio está activado y se confirmó red/destinatario.

- No inventar direcciones, usar solo respuesta del backend.
- No acreditar automáticamente; el administrador debe validar blockchain.
- Cuando las recargas están inactivas, mostrar aviso compacto en lugar de formulario.
- El historial se consulta por empresa autenticada mediante `/api/commercial/topups`.
- La activación independiente de tienda/edición de paquetes desde Administración NO está implementada en esta base: requiere otro cambio de backend; no activar `WORKTIC_COMMERCIAL_ENABLE` solo para probar diseño.
- `go test ./...` y `go build ./...` requeridos en Go 1.26.
- Mantener respaldo de PostgreSQL y /var/data y confirmar Git main antes de cualquier publicación.
