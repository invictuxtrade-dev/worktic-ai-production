# WorkticAI V27.35.2 — Canales unificados

**Base:** V27.35.1 (Chat Web y Llamadas IA). **Alcance:** interfaz web de Canales, sin cambios de esquema SQL, APIs, pagos ni límites de servidor.

## Cambios
- Desaparecen los banners de Chat Web y Llamadas IA dentro de Canales.
- El selector «Nueva conexión» permite WhatsApp Cloud, WhatsApp QR, Telegram, Messenger, Chat Web y Llamadas IA.
- Cada tipo abre la configuración pertinente sin introducir formularios duplicados de almacenamiento.
- Lista uniforme: tipo, estado, nombre, cuenta o dominio, agente asignado y acciones disponibles.
- Chat Web: dominio HTTPS, agente, activación, código de instalación y acceso a Conversaciones.
- Llamadas IA: voz, agente, número, límite de duración y pruebas desde la herramienta avanzada existente.
- Oculta los dos accesos laterales redundantes, sin eliminar las rutas internas que sostienen la configuración avanzada.
- Mantiene WhatsApp Business Center, Studio y Billetera, sin rediseñarlos.

## Límites explícitos
- Chat Web, Llamadas IA y canales legados **siguen usando sus tablas y endpoints actuales**. La UI los reúne, no migra sus registros a una tabla común.
- La política de límites de canales de voz sigue siendo la del backend V27.35.1, distinta del contador de mensajería; no se modifica la facturación ni el guard de activación comercial.
- Chat Web requiere WORKTIC_WEBCHAT_ENABLED=true para crearse; las respuestas de agente automáticas continúan condicionadas por WORKTIC_WEBCHAT_AUTO_AI y las capacidades reales del servidor.
- Los canales de voz necesitan los servicios y requisitos comerciales que el backend ya exige; este parche no habilita llamadas ni recargas por sí solo.

## Prueba funcional en navegador
1. En Canales, comprobar que no haya banners y que Nueva conexión ofrezca seis tipos.
2. Confirmar que el listado conserve WhatsApp, Telegram y Messenger existentes.
3. Crear Chat Web desde el selector; registrar un dominio HTTPS y un agente activo. Comprobar la nueva tarjeta y el botón Instalar widget.
4. Enviar un mensaje real desde el sitio autorizado y verificar la Bandeja.
5. Crear/editar un canal de voz con un agente y voz válidos solo si la configuración comercial lo permite; verificar su tarjeta y acceso a Probar voz.
6. Validar que Studio, Billetera, Agentes, CRM y las conversaciones actuales no cambien.

## Validaciones realizadas
- node --check en JS nuevo y modificado
- HTML: identificadores únicos y carga de recursos
- Smoke test con respuestas simuladas para WhatsApp, Chat Web y voz.
- **No se ejecutó una compilación Go completa, pruebas E2E en Render ni conexiones reales de proveedores desde este entorno.**

## Despliegue
Antes de publicar, ejecutar `go test ./...`, `go build ./...`, `git diff --check` en Windows. Después crear commit y enviar a GitHub con comprobación del HEAD remoto; desplegar por Manual Deploy en Render. Mantener backups de BD y no cambiar variables de voz o Chat Web automáticamente.
