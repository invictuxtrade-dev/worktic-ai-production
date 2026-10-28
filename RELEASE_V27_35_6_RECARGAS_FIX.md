# WorkticAI V27.35.6 — Billetera de Créditos

Base: **V27.35.5**. Parche de interfaz web, sin cambios en base de datos ni en la contabilidad.

## Cambios
- Corrige el desbordamiento horizontal y la confirmación estrecha del checkout USDT mediante una cuadrícula con columna de texto flexible y tamaños protegidos de CSS global.
- Ajusta la ventana de recarga y el QR para móvil y escritorio sin ocultar campos.
- Unifica el rótulo de la ventana en «Billetera de Créditos».
- Superadministradores encuentran «Administrar recargas de Llamadas IA» desplegable en el dashboard de la billetera. Guarda el ajuste comercial en la API existente; no expone el control a otros usuarios.
- Una solicitud originada en Llamadas IA abre la recarga con la pestaña de voz preseleccionada.
- Actualiza versiones de los recursos para evitar caché.

## Requisitos de venta de voz
El backend **existente** mantiene la protección: `voiceTopupsEnabled()` requiere `WORKTIC_VOICE_ENABLED=true`, `OPENAI_API_KEY` configurada, ajuste `sales_enabled=1` y las instrucciones de pago. Además, el checkout necesita la dirección USDT del método seleccionado y el cliente debe tener plan de pago. No habilitar llamadas salientes (`WORKTIC_VOICE_OUTBOUND_ENABLED=false`) hasta completar pruebas de telefonía.

Configurar en Render: `WORKTIC_VOICE_ENABLED=true`, `WORKTIC_VOICE_OUTBOUND_ENABLED=false`, `OPENAI_API_KEY` (valor secreto real). Confirmar ambas direcciones USDT BEP20/TRC20 si se van a ofrecer. Tras desplegar, como superadmin ir a Billetera de Créditos > Administrar recargas de Llamadas IA, ingresar instrucciones, habilitar venta y guardar. Verificar con un usuario de plan activo y un hash de prueba **sin transferir fondos reales**.

El servidor NO acredita créditos al registrar una solicitud: permanece `pending` hasta aprobación administrativa. No reutilizar hashes entre Studio y Voice. Los créditos de voz no son créditos Studio.

## Pruebas locales
```
node --check static/credits-unified-v27355.js
node --check static/studio-wallet-v27346.js
node --check static/voice-v2735.js
go test -count=1 ./...
go build ./...
git diff --check
```

No se certifica la disponibilidad de OpenAI ni la verificación bancaria real solamente con estas pruebas.
