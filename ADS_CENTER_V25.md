# WorkticAI V25 — Ads Center

V25 añade una capa unificada de paid media sobre V24 sin mezclar los modelos de datos de cada proveedor. Esta versión es acumulativa y **no debe desplegarse todavía**.

## Objetivo

Centralizar en WorkticAI:

- Meta Ads
- TikTok Ads
- Google Ads / YouTube Ads

manteniendo dos fuentes de verdad separadas:

1. **Métricas de plataforma**: gasto, impresiones, clics, conversiones y valor que devuelve la API publicitaria.
2. **Resultados Worktic**: leads, citas, oportunidades ganadas e ingresos realmente registrados en CRM.

Worktic nunca convierte una conversión del proveedor en una venta CRM automáticamente.

## Módulo Ads Center

Vistas:

- Resumen
- Cuentas publicitarias
- Campañas
- Estructura (campaña → ad set/ad group → anuncio)
- Atribución

### Resumen

KPIs normalizados:

- gasto
- impresiones
- clics
- CTR
- CPC
- conversiones del proveedor
- ROAS del proveedor
- leads Worktic
- citas Worktic
- ventas CRM
- revenue CRM
- CPL Worktic
- CAC Worktic
- ROAS Worktic

### Cuentas publicitarias

Cada tenant puede conectar varias cuentas por proveedor. Las credenciales se guardan cifradas con `CHANNEL_ENCRYPTION_KEY`.

- Meta: descubre cuentas publicitarias autorizadas.
- TikTok: descubre advertiser IDs autorizados.
- Google: descubre cuentas accesibles y clientes de primer nivel bajo MCC cuando están disponibles.

### Campañas

Se sincronizan campañas externas y se permite asociarlas a una campaña interna de Worktic. Ese mapeo conecta el gasto oficial con V24 Analytics.

Acciones soportadas en V25:

- sincronizar
- mapear a campaña Worktic
- activar/pausar cuando el proveedor lo permite
- ver presupuesto reportado
- ver estado y objetivo/tipo

Las campañas Google de tipo VIDEO heredado se mantienen en solo lectura porque la Google Ads API permite reporting, pero no su mutación completa.

### Estructura

Se sincroniza inventario actual:

- Meta: campaigns → ad sets → ads
- TikTok: campaigns → ad groups → ads
- Google: campaigns → ad groups → ads

La estructura sirve para diagnóstico y auditoría. V25 no modifica ad groups/ads desde esta vista.

## Sincronización

- Manual desde Ads Center.
- Worker automático cada 6 horas.
- Manual: 30 días por defecto, máximo 90 días.
- El worker conserva histórico de métricas y mapeos de campañas.
- Grupos/anuncios se reconstruyen como inventario actual.

TikTok Reporting puede tener latencia; Worktic muestra los datos disponibles y no genera valores faltantes.

## Multi-moneda

V25 **no hace conversión FX automática**.

Analytics tiene una moneda operativa por tenant. Solo el gasto de campañas cuya moneda coincide con esa moneda se usa para CPL/CAC/ROAS consolidado. Campañas en otras monedas siguen visibles y se marcan como no consolidables.

Esto evita sumar, por ejemplo, COP + USD + EUR como si fueran la misma unidad.

## Integración V24

Ads Center alimenta `marketing_metrics` únicamente con:

- gasto
- impresiones
- clics
- leads del proveedor

No escribe ventas ni revenue de plataforma dentro del CRM.

V24 calcula ventas/revenue usando oportunidades `Ganado` reales.

Además V24 evita doble contabilizar gasto: usa gasto oficial de V25 para campañas conectadas y conserva gasto manual solo en campañas no mapeadas.

## Nuevas tablas automáticas

- `ads_connections_v25`
- `ads_campaigns_v25`
- `ads_entities_v25`
- `ads_daily_metrics_v25`
- `ads_oauth_states_v25`
- `ads_sync_log_v25`

No se requiere SQL manual.

## Nuevas rutas

```text
/api/ads/v25/config
/api/ads/v25/connections
/api/ads/v25/oauth/start
/api/ads/v25/oauth/callback/{provider}
/api/ads/v25/sync
/api/ads/v25/overview
/api/ads/v25/campaigns
/api/ads/v25/entities
/api/ads/v25/mapping
/api/ads/v25/campaign-status
/api/ads/v25/internal-campaigns
```

## Variables nuevas V25

```env
TIKTOK_BUSINESS_APP_ID=
TIKTOK_BUSINESS_SECRET=
TIKTOK_BUSINESS_AUTH_URL=
GOOGLE_ADS_DEVELOPER_TOKEN=
```

Se reutilizan:

```env
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
CHANNEL_ENCRYPTION_KEY=
BASE_URL=https://TU-DOMINIO
```

### TikTok

`TIKTOK_BUSINESS_AUTH_URL` es la URL de autorización de advertiser generada/configurada en TikTok API for Business. Su redirect debe apuntar al callback V25 de producción.

### Google Ads

`GOOGLE_ADS_DEVELOPER_TOKEN` es independiente del OAuth Client ID/Secret. El OAuth usa el scope de Google Ads y el developer token autoriza llamadas a la Google Ads API.

## Redirects a registrar al despliegue final

```text
{BASE_URL}/api/ads/v25/oauth/callback/meta
{BASE_URL}/api/ads/v25/oauth/callback/google
{BASE_URL}/api/ads/v25/oauth/callback/tiktok
```

## Permisos / revisiones externas

### Meta

La app debe tener los permisos y nivel de acceso necesarios para las cuentas que gestionará, incluyendo los permisos de Marketing API requeridos por el caso de uso.

### TikTok

Se necesita una app de TikTok API for Business con acceso de Marketing API y autorización de advertiser.

### Google

Se necesita Google Ads API habilitada, OAuth configurado y un developer token con nivel de acceso adecuado al uso de producción.

## Límites deliberados

- V25 no hace conversión de moneda.
- V25 no crea campañas externas completas todavía; sincroniza, analiza, mapea y controla estados soportados.
- Ad groups/ad sets y anuncios son de lectura/diagnóstico en esta fase.
- Google VIDEO heredado es solo lectura desde Worktic.
- Una métrica del proveedor y una venta CRM se muestran como conceptos diferentes.

## Validaciones realizadas

- `gofmt` en código Go modificado.
- `ads_v25.go` compilado aisladamente con stubs usando Go 1.23 y librería estándar: OK.
- JavaScript V25: `node --check` OK.
- JavaScript principal e i18n: OK.
- HTML: IDs duplicados = 0.
- Rutas y tablas V25 presentes.
- Integridad del ZIP se valida al empaquetar.

El build completo del repositorio continúa requiriendo Go 1.24+ y dependencias que este entorno no puede descargar.
