# WorkticAI V27.34.9 — Studio: recargas editables y pagos manuales

**Base exacta:** WorkticAI V27.34.8. No sustituir desde una versión distinta sin reconciliar Git.

## Implementado
- `Créditos Studio → Recargar créditos`: catálogo real de paquetes configurables, importe USDT, red y dirección oficial según Render, copiar dirección, registrar hash, estado pendiente.
- Administración Worktic → Commercial Control Center → Recargas y paquetes: activar/desactivar solo la venta Studio, editar o crear paquetes (nombre, código, créditos, USDT, activo), historial de pagos pendientes y revisión.
- La activación reside en PostgreSQL (`studio_topups_settings_v27349`) y NO modifica `WORKTIC_COMMERCIAL_ENABLE`.
- El servidor comprueba plan pagado y titular de la empresa antes de aceptar solicitud. No acredita saldo en la solicitud.
- Aprobar exige confirmar blockchain e introducir independientemente hash verificado, dirección receptora y USDT recibido. Guarda evidencia de verificación en `studio_topup_verifications_v27349`; se auditan cambios del administrador.
- Protección compartida del hash entre suscripciones y recargas cuando la tienda está activa; inserción transaccional e idempotencia.
- Una recarga conserva el precio y créditos originales aunque Administración cambie luego el paquete.
- El presupuesto externo por créditos comprados se mantiene en el controlador de costos. Las recargas desactivadas no eliminan créditos comprados anteriormente.
- UI responsive con panel para cinco paquetes iniciales y ajustes de precios. Sin compras ni credenciales inventadas.

## Requisitos ANTES de aceptar USDT
1. Staging separado (PostgreSQL, redis y backup). Ejecutar `go test ./...` y `go build ./...` **con Go 1.26**.
2. Validar generación real (imagen y video) y débitos / devolución de créditos; comprobar OpenAI y Luma. No damos por verificados estos servicios solo porque haya una clave.
3. Configurar en Render `OPENAI_API_KEY`, `LUMA_AGENTS_API_KEY`, al menos una `USDT_BEP20_ADDRESS` o `USDT_TRC20_ADDRESS`, `WORKTIC_STUDIO_CREDITS_ENABLE=1` y **todos** los presupuestos `WORKTIC_COST_*` requeridos (centavos de USD; positivos).
4. Mantener `WORKTIC_COMMERCIAL_ENABLE=0` si las demás puertas siguen en pruebas. No afecta a la tienda.
5. Administración Worktic → Commercial Control Center → Recargas y paquetes: ajustar importes y cantidades y activar la tienda. Solo entonces se mostrará como disponible. El servidor requiere tanto configuración de proveedores como presupuesto y wallet oficial.
6. Prueba piloto controlada con importe pequeño: un usuario titular de empresa escoge paquete, envía USDT, registra hash real, y administración verifica cantidad, red, contrato del token, dirección destino, confirmaciones y ausencia de doble registro, introduciendo esos datos en el formulario antes de aprobar.
7. Comprobar saldo antes/después, movimientos, rechazo, solicitudes simultáneas, intentos de hash duplicado y revocación sin saldo negativo.

### Precios iniciales (editables desde Administración)
| Paquete | Créditos | Precio USDT |
|---|---:|---:|
| Mini | 30 | 3.50 |
| Creator | 100 | 10.00 |
| Growth | 300 | 27.00 |
| Pro Studio | 1,000 | 80.00 |
| Agency | 3,000 | 210.00 |

El backend rechaza paquetes de menos de US$0.05 por crédito; **ese mínimo no constituye certificación de margen**. Consulta costos de proveedores antes de bajar precios.

## LIMITACIONES / honestidad operativa
- Acreditación **manual**, no es un procesador automático de pagos ni verificación blockchain automática. Los campos revisados por el administrador no prueban por sí solos que el pago existió.
- No incluye cambios a Partners o comisión multinivel sobre créditos.
- Si una API deja de responder o no hay saldo técnico, la generación externa permanece bloqueada incluso si el cliente tiene créditos. No vendas hasta comprobarla.
- No se ejecutaron pagos reales ni pruebas PostgreSQL multiinstancia en este entorno. Realizar pilotaje en staging y activar solo después de comprobar el servicio en Render.
- No hay refund on-chain automático. Un reembolso monetario se coordina por fuera y requiere conciliación administrativa.

## Archivos del parche
`VERSION.txt`, `.env.render.example`, `RELEASE_MANIFEST_V2734.json`, `RELEASE_V27_34_9_RECARGAS_ADMIN.md`, `main.go`, `commercial_credits_v2721.go`, `commercial_public_offer_v2734.go`, `creative_cost_guard_v2720.go`, `studio_topups_v27349.go`, `static/app.html`, `static/commercial-v2721.js`, `static/studio-topups-v27349.js`, `static/studio-topups-v27349.css`.

No modificar secretos en Git ni reejecutar migraciones antiguas de SQLite a PostgreSQL.
