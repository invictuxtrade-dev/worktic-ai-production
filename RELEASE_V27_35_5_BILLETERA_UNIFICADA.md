# WorkticAI V27.35.5 — Billetera de Créditos

Base: V27.35.4 (corrección de aislamiento de agentes). Sin cambios al esquema de Studio, a la asignación mensual ni al catálogo de planes.

## Interfaz

- Un único menú **Billetera de Créditos**, sin banner independiente de Llamadas IA.
- Panel de saldos: Studio y Llamadas IA en dos tarjetas del mismo dashboard. Los saldos siguen aislados; no hay conversiones o transferencias.
- Los detalles y métricas existentes de Studio continúan disponibles debajo del resumen.
- Un único diálogo de recarga con selector **Studio IA / Llamadas IA**, paquete oficial del servidor, BEP20/TRC20, dirección y QR escaneable, hash y consentimiento.
- Un solo diálogo de movimientos con historial combinado. Los movimientos de llamadas se muestran a partir de llamadas y recargas existentes en API V27.35.
- El centro de llamadas mantiene su configuración y pruebas de voz, pero deja de ofrecer una pantalla paralela de recargas.

## Reglas de pago

- Los QR se generan **localmente en el navegador** y codifican **solo la dirección de recepción oficial**. No incluyen el token, red ni importe, que el usuario debe seleccionar y confirmar por separado.
- No se muestra QR si el backend no entrega una dirección configurada. No se utiliza un servicio externo de QR.
- Studio mantiene POST `/api/commercial/topups` y Llamadas IA mantiene POST `/api/voice/v2735/topups`, ahora con `{package,network,tx_hash}`.
- Las recargas de voz requieren activación comercial, plan compatible, dirección oficial y un hash válido. El backend utiliza `billing_payment_claims_v2728` para evitar reutilizar un hash en **planes, Studio o Voz**.
- Las recargas siguen en estado `pending` hasta aprobación administrativa; escanear el QR no acredita créditos.
- Las solicitudes antiguas de voz no se borran ni se migran automáticamente.
- Conservar `WORKTIC_VOICE_ENABLED=false` o ventas voz deshabilitadas hasta probar proveedor, costos y validación bancaria.

## Cambios

`VERSION.txt`, `billing_lifecycle_v2728.go`, `voice_v2735.go`, `static/app.html`, `static/i18n.js`, `static/studio-wallet-v27346.js`, `static/voice-v2735.js`, `static/credits-unified-v27355.js`, `static/credits-unified-v27355.css`, `static/qr-local-v27355.js`.

## Validación

Realizadas aquí: sintaxis JS; parseo de Go (no compilación completa); QR generado y decodificado correctamente con un lector independiente; contraste del ZIP. **Pendientes:** `go test -count=1 ./...`, `go build ./...` con Go 1.26 y pruebas reales de los dos cobros en staging y en Render.

**Pruebas de aceptación recomendadas:**
1. Empresa A ve exclusivamente sus saldos Studio y Voz.
2. Alternar entre servicios cambia paquetes e importe sin alterar el otro saldo.
3. Cambiar BEP20 ↔ TRC20 cambia el QR y la dirección oficial.
4. La tienda deshabilitada impide solicitar recargas y no invita a transferir.
5. Hash inválido, repetido o utilizado en planes/Studio/Voz se rechaza.
6. Solicitud pendiente no acredita créditos; aprobar una vez acredita solo la billetera elegida.
7. El menú lateral ya dice Billetera de Créditos y no aparecen banners de voz en esa página.
8. La activación de Chat Web y Llamadas IA desde Canales sigue operativa.
