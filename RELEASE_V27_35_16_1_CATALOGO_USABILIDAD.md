# WorkticAI V27.35.16.1 — Catálogo: CRM y ventas usables

## Qué se corrigió

- Se **conserva la regla de V27.35.16**: compartir solo productos activos y disponibles; si están sin stock, no se ofrecen como disponibles.
- **Vincular a CRM:** selector visual de oportunidades reales de la misma empresa, búsqueda por cliente/título/etapa, cantidad y guardado en `/api/catalog/opportunity-link` sin IDs manuales.
- **Registrar pago:** modal guiado con producto, precio estimado y cantidad, oportunidad opcional, referencia obligatoria, adjunto opcional y resumen del pedido. El precio final sigue calculándose en el servidor.
- **Comprobante privado:** JPG/PNG/WEBP/PDF, máximo 4 MiB, almacenado en tabla `catalog_sale_evidence_v2735161`. Se accede mediante `GET /api/catalog/sales/evidence?sale_id=...` con sesión autenticada, permisos de administración del catálogo y pertenencia a empresa; nunca por URL pública de catálogo. Reemplazo de adjunto queda auditado con hash SHA-256.
- **Revisión de pagos:** listado visual por estados, descarga de comprobante, adjuntar a pedidos pendientes, y decisiones aprobar/rechazar con motivo. Aprobación exige confirmación explícita de revisión **manual**.
- **No** modifica el menú, cabecera, navegación, las tarjetas originales de Catálogo ni los límites del plan.

## Qué NO se ha implementado

- La IA **todavía no recibe ni asocia automáticamente fotos de comprobantes a pedidos** desde WhatsApp, Messenger, Telegram o Chat Web. La V27.35.16 daba contexto del catálogo pero **no implementaba una herramienta de pedido autónomo para el agente**. Requiere un flujo autenticado o un vínculo de carga seguro, consentimiento, mapeo de adjuntos a la oportunidad y controles antisuplantación por cada canal.
- No hay comprobación automática del pago contra banco, blockchain o pasarela; referencia y foto **no prueban recepción de fondos**.
- No se cambian reglas de aprobación ni descuento de inventario: se descuenta solo con aprobación manual registrada en la BD.
- No se verifica el resultado en la BD real ni en Render. Solo publicar luego de `go test` y `go build` con Go 1.26 y pruebas E2E.

## Flujo de pruebas manual en empresa de pruebas

1. Desde Catálogo, pulsa **Vincular a CRM**; busca una oportunidad por nombre y vincula 1 unidad. Comprueba que se mantiene en la empresa correcta.
2. Prueba un producto sin stock: compartir sigue bloqueado (regla conservada).
3. Con stock disponible, pulsa **Registrar pago**; introduce referencia única, elige oportunidad si procede y adjunta JPG o PDF de prueba.
4. En **Revisión de ventas**, verifica pedido pendiente, descarga protegida del adjunto y referencia.
5. Como otro tenant, verifica que no pueda descargar el comprobante ni consultar el pedido de la empresa inicial.
6. Confirma que el inventario no cambia con el estado pendiente, y que sí cambia tras revisión manual autorizada.
7. Repite con servicio e inventario ilimitado, y con stock insuficiente al momento de aprobar. Revisa historial de auditoría.
8. Valida el layout desktop y móvil, scroll del modal y mensajes de error del servidor.

## Archivos

- `VERSION.txt`
- `main.go`
- `catalog_commerce_v273516.go`
- `catalog_sale_evidence_v2735161.go` (nuevo)
- `static/app.html`
- `static/catalog-commerce-premium-v2735161.js` (nuevo)
- `static/catalog-commerce-premium-v2735161.css` (nuevo)
- `RELEASE_V27_35_16_1_CATALOGO_USABILIDAD.md` (nuevo)

Al iniciar, `initCatalogCommerceV273516` crea una nueva tabla SQLite para los comprobantes. Respaldar la base de datos persistente antes de desplegar. La reversión de código no elimina los datos de esa tabla.
