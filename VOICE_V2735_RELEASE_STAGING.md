# WorkticAI V27.35.0 STAGING — Llamadas IA

Base: repositorio V27.34.9 Lite suministrado el 27/10/2026. No sobrescribe datos de Studio, ni modifica saldos, consumo o aprobaciones Studio.

## Entregado en el código

- Nuevo menú **Llamadas IA** y acceso desde Canales; billetera Worktic con acceso a Studio y Voz.
- Gestión de canales de voz por tenant y asignación a agente existente de `ai_agents`. Ocho voces con selección por canal.
- WebRTC para prueba de voz con micrófono desde HTTPS, llamado al endpoint oficial OpenAI GPT-Live, sin revelar `OPENAI_API_KEY` al navegador.
- Telefonía entrante: Twilio Programmable Voice `<Dial><Sip>` a OpenAI SIP, con correlación x-worktic-call, validación de firma Twilio y webhook verificado OpenAI (`live.transport.incoming`).
- Telefonía saliente mediante SIP trunk desde `POST /v1/live/sessions` (deshabilitada por defecto), evidencia de consentimiento por número y bloqueo conservador de horarios y países.
- Billetera de voz independiente: créditos mensuales de Empresa con suscripción activa, créditos comprados, reservas atómicas por empresa, liquidación con devolución de no consumidos, limitador por saldo y 2 llamadas paralelas como comprobación preliminar.
- Cargos por duración: redondeo por minuto, tarifa guardada en cada llamada, devolución idempotente, conciliación conservadora de reservas antiguas. La duración se obtiene del proveedor, nunca del cliente.
- Recargas manuales con paquetes configurados a partir de la tarifa, instrucciones/enlace de pago ajustables por superadmin y aprobación transaccional idempotente (sin abono antes de revisión real).
- Página comercial dinámica: Empresa muestra créditos de Llamadas IA, otros planes de pago pueden adquirir paquetes y Free no está habilitado. Mientras el proveedor esté deshabilitado la página indica **activación pendiente**, no uso inmediato.
- `voice_v2735_test.go`: formatos telefónicos, paquetes y verificación de firma de webhooks.

## Lo que NO se verificó ni se promete

1. **No se realizaron llamadas reales**: falta `OPENAI_API_KEY` con GPT-Live, project ID, proveedor de números/SIP, webhook publicado por HTTPS y presupuesto del proveedor. Las conexiones, precios y permisos externos deben validarse antes de afirmar servicio listo para vender.
2. No existe transferencia en caliente a un humano, ejecución de acciones CRM/Agenda durante la llamada, ni transcripción/grabación persistida; la configuración actual utiliza las instrucciones y conocimiento del agente, y evita simular resultados de acciones no conectadas.
3. No hay cobro automático Stripe/USDT para recargas de voz. Las solicitudes son manuales con aprobación por superadministrador. Solo activar ventas con proveedor listo y datos de pago reales.
4. No se corrió el `go test ./...` integral: este entorno dispone de Go 1.23.2 y no puede descargar toolchain/dependencias Go 1.26. Se validaron sintaxis y pruebas unitarias aisladas con stubs, NO sustituyen una prueba de integración.
5. La conciliación después de caída puede cobrar hasta la reserva máxima aprobada en casos de duración no verificable. Revisar disputas y conciliación con la factura del proveedor antes de poner saldos en producción.
6. Los roles adicionales y las campañas masivas requieren política granular y límites de coste/destino más avanzados antes de activarse.

## Activación / Render

1. Subir **en staging**, con copia de seguridad de DB, este proyecto completo y misma persistencia que la base.
2. Colocar `WORKTIC_VOICE_ENABLED=false` inicialmente; configurar `OPENAI_API_KEY`, `OPENAI_PROJECT_ID=proj_...` habilitado para GPT-Live y `WORKTIC_PUBLIC_BASE_URL=https://<tu-dominio-staging>`.
3. Configurar webhook OpenAI para `live.transport.incoming` en `https://<tu-dominio>/webhooks/voice/openai`; guardar `OPENAI_WEBHOOK_SECRET` en Render, nunca en JS.
4. Configurar número de Twilio y `TWILIO_AUTH_TOKEN`. En la consola del número: Voice Webhook HTTP POST `https://<tu-dominio>/webhooks/voice/twilio/inbound`. Confirmar troncal SIP con TLS y SRTP, permisos de proveedor, caller IDs y costos aplicables.
5. Crear el canal en **Llamadas IA**, elegir agente activo, voz y número E.164. El mismo número no se puede asignar a dos empresas. Habilitar `WORKTIC_VOICE_ENABLED=true` cuando la configuración esté lista.
6. Antes de una llamada, verificar créditos suficientes para la **duración máxima**; por ejemplo, 3 minutos × 30 créditos/minuto = 90 créditos reservados, que se liquidan al terminar.
7. Probar desde navegador (HTTPS y micrófono), luego llamadas entrantes. Verificar saldo inicial/final, latencia, calidad voz español, silencio, interrupciones y cierre. Probar fallos, repetición de webhooks, cancelaciones y liquidación real, conciliando con factura de OpenAI/Twilio.
8. En módulo Voice Admin (cuenta superadmin), configurar tarifa de 10–1000 créditos/minuto, asignación mensual Empresa, instrucciones de cobro y opcional enlace HTTPS. **No marcar Habilitar recargas** hasta que las llamadas y los pagos se hayan verificado. Antes de aprobar cada recarga, revisar comprobante real.
9. Salientes: solo si se verifican permisos y destino, establecer `WORKTIC_VOICE_OUTBOUND_ENABLED=true`, `WORKTIC_SIP_PROVIDER_URL=sips:...`, `WORKTIC_SIP_USERNAME`, `WORKTIC_SIP_PASSWORD`. Prefijos salientes permitidos: `WORKTIC_VOICE_ALLOWED_OUTBOUND_PREFIXES=+57` por defecto. Verificar cumplimiento normativo antes de habilitarlo.

## Modelo comercial de referencia (NO validado con costes del operador)

Un crédito de voz vale US$0,01, tarifa inicial 30 créditos/minuto = US$0,30/minuto. Empresa recibe 300 créditos/mes (hasta 10 minutos si el servicio está activo y la suscripción vigente). Paquetes 10 / 30 / 100 minutos = 300 / 900 / 3000 créditos = US$3 / US$9 / US$30. No se cambia el precio de los planes de suscripción preexistentes. Tener en cuenta costo GPT-Live aprox. US$0,05/minuto (API oficial, tarifa consultada 27/10/2026), minutos telefónicos, países de destino, impuestos, comisiones y consumo de backend. Modificar la tarifa por costo medido antes de vender.

## Checklist de puesta en producción

- [ ] `go test ./...` Go 1.26 sin errores en entorno real
- [ ] Migración SQLite/Postgres validada en staging con backup y rollback
- [ ] Navegación móvil y desktop confirmada en browser real
- [ ] Voces reales, barge-in, audio correcto y español latino
- [ ] Twilio webhook y OpenAI webhook verificados con firmas
- [ ] 1 llamada entrante + 1 saliente consentida, precios reales y conciliación
- [ ] Idempotencia de recargas/llamadas validada bajo concurrencia
- [ ] Presupuesto externo, tarifas país, protección DNC y reglas legales aprobados
- [ ] Publicación web comercial solo después de activar servicio

Fuentes técnicas: https://developers.openai.com/api/docs/guides/voice-sip ; https://developers.openai.com/api/docs/guides/voice-webrtc ; https://developers.openai.com/api/docs/pricing ; https://www.twilio.com/docs/voice/twiml/sip
