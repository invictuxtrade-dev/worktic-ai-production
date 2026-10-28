# WorkticAI V27.35.7 — Administración de Billetera de Créditos

**Base exacta:** V27.35.6. El instalador compara huellas SHA-256 de contenidos normalizados LF/CRLF antes de escribir, crea una rama nueva y deja la publicación pendiente.

## Funcionalidades

- Pestaña **Administración Worktic → Billetera de Créditos** (solo superadministrador) con estilo del administrador existente.
- **Studio IA:** control de venta mediante la API ya existente `/api/admin/studio/topups`. Editor de los cinco paquetes actuales (créditos, precio USDT y estado activo), cada uno con su valor efectivo por crédito. No reescribe precios ni balances existentes.
- **Llamadas IA:** control de venta, instrucciones de pago, enlace HTTPS opcional, valor comercial de cada crédito, créditos por minuto y créditos mensuales Empresa. Configuración guardada en `voice_config_v2735` mediante el endpoint superadmin `/api/admin/voice/v2735`.
- **Valores iniciales preservados:** Studio Mini 30 créditos / 3,50 USDT; Creator 100 / 10 USDT; Growth 300 / 27 USDT; Pro 1000 / 80 USDT; Agency 3000 / 210 USDT. Voz: **1 crédito = 0,01 USDT**, **30 créditos/minuto**, **Empresa 300 créditos/mes**.
- Las ofertas de voz futuras calculan su precio en función de los centavos por crédito; solicitudes ya registradas conservan su `price_cents` original.
- El requisito de al menos una dirección USDT oficial se valida antes de considerar activas las ventas de voz.
- Se elimina el control comercial redundante de la billetera del cliente, y se oculta el control legado en el módulo de Llamadas IA. La billetera del cliente mantiene saldos, recargas, QR e historial.
- Se corrige el botón de guardado: **se guarda aunque falte una condición técnica** (por ejemplo clave OpenAI), pero la venta efectiva permanece bloqueada hasta satisfacer todas las condiciones. Se muestra un mensaje real de éxito/error del servidor.

## Antes de publicar

1. Guardar un respaldo de la base de datos persistente de Render.
2. Ejecutar `node --check static/wallet-admin-v27357.js` y `node --check static/credits-unified-v27355.js`.
3. Ejecutar `go test -count=1 ./...` y `go build ./...` con **Go 1.26**.
4. Ejecutar `git diff --check` y revisar `git diff --stat`.
5. Confirmar que no existen cambios ajenos pendientes, hacer commit y luego `git push origin HEAD:main`.
6. Desplegar en Render solo cuando las pruebas pasen.

## Pruebas obligatorias en plataforma

- Superadmin: Administración → Billetera de Créditos carga los cinco paquetes Studio y las tarifas de voz.
- Guardar un ajuste de precio en un paquete Studio, recargar la pantalla y comprobar persistencia; restaurar el valor si era una prueba.
- Guardar voz con **venta deshabilitada** y modificar únicamente instrucciones; confirmar éxito y persistencia.
- Habilitar venta de voz y comprobar que no se considera activa sin `WORKTIC_VOICE_ENABLED=true`, `OPENAI_API_KEY` y una dirección USDT. `WORKTIC_VOICE_OUTBOUND_ENABLED` puede continuar `false`.
- Desde cuenta de empresa probar Billetera de Créditos → Recargar → Llamadas IA: el precio y QR coinciden con el valor guardado. No transferir fondos reales hasta la validación E2E.
- Verificar que una cuenta normal no pueda consultar ni modificar `/api/admin/studio/topups` ni `/api/admin/voice/v2735`.
- Aprobación manual: nunca se acreditan saldos al registrar únicamente un hash; una solicitud pendiente conserva su precio original tras un cambio tarifario.

## Seguridad y límites

El sistema utiliza los endpoints existentes protegidos por `requireSuperadmin`. Las operaciones de compra siguen sujetas al plan del tenant, verificación de hash compartida y aprobación manual. No cambia la unidad de crédito consumida por imágenes o videos (costes creativos por operación); esto controla **precios de venta**, no el coste del proveedor. El precio de voz se puede ajustar, pero no implica que exista un margen positivo: verificar costos reales antes de habilitar clientes.

**Estado de validación en el entorno de preparación:** JavaScript comprobado con Node, Go analizado sintácticamente con `gofmt` y `go/parser`, vista de administrador renderizada en Chrome de escritorio y móvil sin desbordamientos. Compilación integral con Go 1.26 y pruebas de API contra Render no ejecutadas aquí.
