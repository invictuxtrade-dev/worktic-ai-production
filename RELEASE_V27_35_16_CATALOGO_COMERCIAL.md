# WorkticAI V27.35.16 — Catálogo Comercial Inteligente (candidato para pruebas)

Base: V27.35.15.2 (Agenda Inteligente y corrección del botón Disponibilidad). No publicar antes de ejecutar el instalador y verificar los flujos en entorno de pruebas.

## Backend

- `GET /api/products` y `GET /api/catalog/commerce`: fichas con `effective_price`, `promotion_active` y `available`, calculadas por el servidor. Promociones fuera de las fechas especificadas no se aplican. Las fechas sin zona horaria utilizan UTC (ajustar zona horaria empresarial antes de ofrecer promociones de inicio a medianoche en Colombia).
- `POST /api/catalog/share`: selección de un producto (`product_id`) o catálogo completo (`product_id: 0`). Devuelve texto, enlaces de fotos y URL de catálogo compartido, revocable por vencimiento automático de 7 días. Requiere usuario autenticado, limita catálogo por empresa y rechaza agentes sin permiso explícito (`agent_id`).
- `GET /catalogo/{token}`: ficha visual pública solo con productos activos/disponibles y precios actualizados; URL aleatoria de alta entropía, expiración 7 días, `noindex` y no-store. La distribución del enlace debe ser consentida por la empresa.
- `POST /api/catalog/opportunity-link`: relaciona producto + oportunidad CRM existentes de la misma empresa y cantidad. No modifica la etapa de la oportunidad.
- `POST /api/catalog/service-link`: crea o actualiza un servicio de Agenda a partir de una ficha `type=service`; mantiene mapa empresa/producto/servicio. No crea citas automáticamente.
- `POST /api/catalog/sales`: registra solicitud de pago con precio determinado en servidor, estado `pending_review`, referencia única por empresa; no cambia inventario.
- `POST /api/catalog/sales/review`: administración aprueba o rechaza con motivo, tras confirmar explícitamente revisión manual del pago. En aprobación descuenta stock dentro de la misma transacción y registra auditoría. Evita doble aprobación. Servicios e inventario ilimitado no descuentan stock. No constituye conciliación automática bancaria/blockchain.
- `GET /api/catalog/sales`, `GET /api/catalog/audit`, `GET /api/catalog/metrics`: administración y trazabilidad de pedidos por empresa; ingresos reportados por moneda separada.
- `DELETE /api/products?id=`: pasa a archivado reversible `active=0`, no borrado físico. Para reactivar, editar ficha desde la aplicación.
- `agentV22Context()` incluye productos únicamente con agente activo y permiso `catalog_enabled=1` y herramienta `catalog` activada explícitamente en el formulario de agente y tenant coincidente. Aplica a WhatsApp QR, Telegram, Messenger y Chat Web que utilizan ese constructor de contexto, y al simulador. La IA heredada usa los precios efectivos filtrados.
- Respuesta automática ante solicitud explícita de fotos, por producto o catálogo: agrega enlaces válidos; Telegram envía además fotos nativas mediante `sendPhoto`. **WhatsApp QR/Cloud, Messenger y Chat Web aún envían enlaces en texto; no hay envío binario nativo de imágenes implementado para esos canales**. Confirmarlo en pruebas reales antes de ofrecerlo como capacidad nativa.

## Interfaz

Se mantiene el backoffice original. Se agregan botones de compartir una ficha o catálogo, asociar a CRM/Agenda, registrar pago y consultar revisiones. No se altera la barra superior ni el menú. El flujo inicial de vincular por ID es funcional pero aún no es un selector premium por nombre.

## Pendientes críticos antes de venta real

1. `go test -count=1 ./...` y `go build ./...` con Go 1.26, más pruebas E2E sobre Render y copia de seguridad de la BD.
2. Conectar webhooks **firmados y verificables** del proveedor de pagos. Hoy solo existe aprobación manual; nunca aceptar un hash/captura como confirmación automática.
3. Agregar envío multimedia nativo para WhatsApp Cloud, WhatsApp QR, Messenger y representación enriquecida en Chat Web, de acuerdo con las API de cada proveedor.
4. Añadir selección de productos en CRM y Agenda por nombre (en vez de ID) y un panel de pedidos de interfaz completamente integrado.
5. Pruebas de seguridad multitenant, stock concurrente, pago duplicado e imagen inaccesible; evaluar auditoría transaccional del CRUD de fichas.
6. Revisar licencias, políticas de medios y la URL pública con dominio propio/URLs firmadas en empresas que lo requieran.

## Instalación

El parche incluye `INSTALAR_CATALOGO_V273516.ps1`. Exige versión base y huellas conocidas, respalda los archivos modificados, ejecuta JS/Go/Git y revierte los cambios locales si ocurre un error. Las migraciones SQL se crean al arrancar el servidor después de publicar; un rollback de código no borra tablas ni datos nuevos.

**WhatsApp Cloud:** el webhook actual registra mensajes, pero no encontré ejecución automática de Agentes IA en esa ruta. Por seguridad, esta versión no afirma que Cloud responda automáticamente con fichas o imágenes: queda pendiente conectar y probar esa rama del motor de canales.
