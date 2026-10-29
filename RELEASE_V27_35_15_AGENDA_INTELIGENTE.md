# WorkticAI V27.35.15 — Agenda Inteligente Premium

**Base requerida:** V27.35.14 (menú clásico ordenado). Fecha: 2026-10-29.

## Diseño y flujo
- Vista principal **Calendario** (mes/semana/día/lista), con panel lateral de fecha seleccionada, próximas citas y actividad.
- Indicadores: citas de hoy, confirmadas hoy, pendientes hoy, citas con origen IA identificado y próxima reserva.
- Filtros por profesional, estado, origen y búsqueda de cliente/servicio.
- Estilos responsivos para escritorio y móviles, sin reemplazar las pantallas existentes de Citas, Disponibilidad, Profesionales, Servicios y Configuración.
- Crear, editar, confirmar, cancelar y reprogramar continúa a través del formulario y API existentes.
- El calendario se alimenta de **la misma tabla crm_appointments** que ya usa el backend. No se duplican datos ni se modifican reservas existentes.

## Modelo de datos / API
- GET `/api/appointments`: agrega `source` y `source_ref` a la respuesta de cada cita; mantiene los demás campos y el filtro `tenant_id`.
- POST `/api/appointments`: marca nuevas citas realizadas desde la API de usuario como `source='manual'` por defecto. Los orígenes registrados por otros procesos conservan sus propios valores en base de datos.
- GET `/api/agenda/activity?limit=12`: nuevos eventos de auditoría para la empresa autenticada.
- Se crea `agenda_appointment_audit_v273515` como tabla exclusiva de historial de operaciones sobre citas a través de la API (creación, edición, eliminación), con ID, empresa, actor, acción, foto anterior/posterior y timestamp.
- Las reglas actuales de disponibilidad y las validaciones de solapamiento siguen en el backend existente. No se cambian los servicios, horarios ni límites comerciales.

## Sincronización
- La vista activa consulta el backend cada **15 segundos** y cuando se vuelve a la pestaña del navegador.
- Al guardar una cita desde el formulario existente, el calendario se actualiza de inmediato.
- Esta es una sincronización casi en tiempo real mediante polling; no constituye transporte push por WebSocket/SSE.
- La consulta devuelve datos nuevos cuando ya se hayan escrito en `crm_appointments`. Los procesos de agentes/canales externos deben crear sus citas en esa tabla y aportar su `source` para clasificar el origen. No se ha verificado aquí cada ruta externa de agente.

## Matriz de aceptación antes del despliegue
1. Abrir Agenda: se muestra Calendario por defecto, sin eliminar reservas existentes.
2. Crear cita manual con servicio y profesional reales: aparece al guardar y queda en Citas.
3. Crear cita desde Agente IA en una cuenta de prueba: comprobar que el agente escribe en `crm_appointments` de su propia empresa y que se ve dentro de 15 segundos.
4. Reprogramar, confirmar, cancelar y volver a abrir la cita: calendario, lista y estado coinciden.
5. Acceder con empresa diferente: ninguna cita ni evento de auditoría ajeno debe ser visible.
6. Crear una nueva cita en otra sesión: aparece sin recargar la página, en hasta 15 segundos con la vista abierta.
7. Filtrar por profesional, origen, estado y cliente.
8. Revisar auditoría con cuenta de prueba; comprobar usuario y acción.
9. Probar día/semana/mes/lista y los tamaños de 390 px y 1366 px.
10. Validar que la página pública, Studio, Canales, Llamadas IA y la navegación existente no cambien.

## Límites conocidos que NO deben presentarse como totalmente validados
- Las pruebas visuales con datos de ejemplo se realizaron en Chromium, **no contra la base de producción**.
- La auditoría actual es de mejor esfuerzo para acciones de la API de citas; NO cubre modificaciones realizadas por SQL directo/otros endpoints, y no está acoplada de forma transaccional al cambio principal. Si falla la escritura de auditoría, se escribe un error en los logs. Antes de habilitar auditoría legal estricta, habrá que convertirla en operación atómica.
- La eliminación sigue siendo física en la API original; el nuevo log conserva una instantánea, pero **no permite restaurar** una cita eliminada.
- No se incluyen recordatorios automáticos por WhatsApp/email, sincronización con Google Calendar, bloqueos manuales nuevos ni exportaciones; requieren integración adicional.
- No se pudo realizar `go test ./...` ni `go build ./...` en el entorno de preparación porque faltó Go 1.26 y no hay salida de red. Se debe compilar y ejecutar la batería de pruebas de Go 1.26 en tu Windows.

## Despliegue seguro
1. Respaldar la base de datos persistente en Render.
2. Instalar `WorkticAI_V27_35_15_AGENDA_INTELIGENTE_PARCHE.zip` solo sobre la rama/código V27.35.14 esperado.
3. Ejecutar `node --check static/agenda-command-v273515.js`, `go test -count=1 ./...`, `go build ./...`, `git diff --check`.
4. Validar con dos empresas y con cita creada por agente. Solo entonces publicar.

## Archivos modificados
`VERSION.txt`, `static/app.html`, `main.go`, `agenda_premium.go`, `db_compat.go`.

## Archivos añadidos
`static/agenda-command-v273515.css`, `static/agenda-command-v273515.js`, `agenda_activity_v273515.go`, `agenda_activity_v273515_test.go`, documentación e instalador.
