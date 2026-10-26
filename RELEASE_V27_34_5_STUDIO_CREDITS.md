# WorkticAI V27.34.5 — Créditos Studio configurables

Base acumulativa: V27.34.4. Parche incremental sin modificaciones a comisiones Partners ni a precios de suscripción.

## Planes

Por defecto la tabla `commercial_rules_v2721` contiene: Free 0, Personal 30, Negocio (`business`) 120, Empresa (`enterprise`) 300. La semilla es `ON CONFLICT DO NOTHING` y **respeta los cambios posteriores del administrador**. Administración Worktic → Commercial Control Center permite editar mensualmente esos valores en PostgreSQL. El acceso se comprueba como superadministrador.

## Costes de créditos activos en el servidor

Imagen IA: 4 créditos; video IA 5 s a 720p: 15 créditos; 10 s a 720p: 45 créditos; texto/copy: 2; narración: 5 por bloque de 1.000 caracteres. Las equivalencias son matemáticas, no un paquete de imágenes y videos acumulable:

| Plan | Créditos | Imágenes aprox. (4) | Videos IA 5 s aprox. (15) |
|---|---:|---:|---:|
| Personal | 30 | 7 | 2 |
| Negocio | 120 | 30 | 8 |
| Empresa | 300 | 75 | 20 |

## Activación técnica independiente

La variable de Render `WORKTIC_STUDIO_CREDITS_ENABLE=1` permite activar el débito de Studio **sin activar** el resto de los controles comerciales. El sistema comprueba además que haya presupuestos positivos en:
`WORKTIC_COST_GLOBAL_MONTHLY_CENTS`, `WORKTIC_COST_BUDGET_PERSONAL_CENTS`, `WORKTIC_COST_BUDGET_NEGOCIO_CENTS`, `WORKTIC_COST_BUDGET_EMPRESA_CENTS`.

**Antes de activar**, probar en staging las reservas PostgreSQL, llamadas OpenAI y Luma, emisión de assets, renovaciones, cierre mensual y que `WORKTIC_STUDIO_CREDITS_ENABLE=0` mantenga gasto externo bloqueado por los presupuestos. La variable no prueba que existan credenciales válidas ni que el API de Luma funcione: requieren validación real. Para dar exactamente el presupuesto de 2,5 centavos/credito, los presupuestos iniciales de referencia por empresa son Personal 75, Negocio 300, Empresa 750 centavos; el techo global se define según su capacidad real. Ajustar cuando cambien las tarifas de proveedores.

`WORKTIC_COMMERCIAL_ENABLE` puede seguir en 0: las recargas en USDT permanecen sin vender y el resto de módulos conserva sus flags. No activar simultáneamente `WORKTIC_COMMERCIAL_ENABLE` hasta pasar sus pruebas de compra y permisos.

## Web y checkout

El servidor suministra números configurables por plan y los precios unitarios de créditos. El texto de los planes ya no resalta «previstos»; incluye estimaciones de clips/imágenes sin sumarlas. **Mientras no esté habilitada la generación real, el checkout muestra expresamente que los créditos aún no se pueden gastar** para no representar una función inactiva como actualmente disponible.

## Pruebas mínimas antes de desplegar

- `python tools/v2734/verify_manifest.py`
- `node --check static/commercial-offer-v2734.js`
- `node --check static/commercial-v2721.js`
- `go test ./... && go build ./...` con Go 1.26 (pendiente en el contenedor actual)
- Pruebas en PostgreSQL staging con compras aprobadas, tres planes y dos tenants concurrentes
- Backoffice → Administración → Commercial Control Center: cambiar 30→31 y confirmar actualización en web y checkout sin recargar otros servicios
- Generación de imagen, voz y Luma real con presupuestos y API autorizadas; confirmar liquidación de hold una sola vez y reembolso solo cuando corresponda.

Esta entrega NO modifica producción ni activa por sí sola las variables de Render.
