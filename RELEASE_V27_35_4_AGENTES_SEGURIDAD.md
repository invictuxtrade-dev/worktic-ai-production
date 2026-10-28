# WorkticAI V27.35.4 — Parche de aislamiento multiempresa para agentes

Base verificada del paquete: V27.35.3 (Chat Web + Canales unificados + Llamadas IA).

## Hallazgos en el codigo

1. En el arranque se ejecutaban `migrateAgentTenants` y `migrateClassicAgentToUnifiedAgents` sin condición; la primera podía reinterpretar IDs de empresa como IDs de usuarios y la segunda creaba un `Asistente Principal` en estado `draft` a empresas que no tenían agentes.
2. Las respuestas automáticas de WhatsApp QR, Telegram y Messenger recurrían a `worktic_agent`, configuración **global**, cuando faltaba un agente propio válido. Esto permitía que distintas empresas usaran la misma identidad e instrucciones.
3. El selector de rutas no comprobaba la pertenencia y activación de su agente al devolverlo.
4. La creación/conexión de un canal no verificaba de forma consistente el propietario del agente, y ciertos permisos y configuraciones V22 aceptaban IDs de agentes sin validar empresa.

## Correcciones

- Arranque: elimina las dos migraciones automáticas peligrosas. No borra agentes existentes.
- El enrutamiento solo devuelve un agente activo del mismo tenant; sin agente válido no se responde con IA.
- WhatsApp QR, Telegram, Messenger: no usan `worktic_agent` global como respaldo.
- Conexiones, rutas, permisos, configuración V22, conocimiento y WhatsApp Cloud validan propiedad del agente.
- El endpoint legado de configuración del agente global se restringe al superadministrador y las respuestas legacy automáticas quedan bloqueadas en despliegues multiempresa.
- Prueba de regresión con dos empresas en `agent_security_v27354_test.go`.
- `AUDITORIA_AGENTES_SOLO_LECTURA.sql` para inspeccionar datos ya guardados. **No ejecutar scripts que reasignen o eliminen agentes** sin revisar esta auditoría.

## Limitaciones y riesgos restantes

La corrección cubre los flujos de agentes y canales identificados mediante revisión estática, no certifica toda la plataforma. Hay que ejecutar `go test ./...`, `go build ./...` y verificar con dos empresas reales de prueba en staging. Una conversación en curso o una asignación ya incorrecta debe auditarse y corregirse manualmente, tras respaldo de datos y registros. La existencia de dos agentes con el mismo nombre no demuestra por sí sola que sean duplicados; la auditoría permite diferenciarlos por tenant, IDs y fecha.

### Activación recomendada

Antes de desplegar: respaldo de base de datos y entorno; pausar respuestas automáticas en canales afectados. Después de publicar el código: auditar relaciones, crear un agente activo por empresa, reasignar exclusivamente a agentes de su propia empresa, y probar que no hay respuestas cruzadas. Dejar `WORKTIC_WEBCHAT_AUTO_AI=false` hasta el fin de las pruebas.
