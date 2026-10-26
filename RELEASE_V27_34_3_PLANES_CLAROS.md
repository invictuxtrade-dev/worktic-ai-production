# WorkticAI V27.34.3 — Planes completos y organizados

Base: V27.34.2. Parche incremental de web comercial y API de catálogo público.

## Ajustes
- Los planes destacan Growth & Campañas, Landing Pages y Agentes IA mediante permisos leídos del catálogo vigente (`features_json` y `commercialModuleCatalogV2722`).
- Los agentes permitidos se obtienen de `billing_plans.max_agents`. No se inventan cifras.
- El backend expone `studio_credits_planned`, tomado de `commercial_rules_v2721.monthly_credits`, **separado** de `monthly_credits`, que permanece en cero hasta activar WORKTIC_COMMERCIAL_ENABLE.
- Los créditos previstos se presentan con "no incluidos en la compra actual" cuando aún no están habilitados. No se anuncian como créditos disponibles.
- El checkout usa la misma función `benefits()` con las tres áreas y los créditos, sin generar derechos artificiales.
- El resumen de compra muestra las mismas áreas y sus créditos; scripts públicos y del backoffice actualizados con cache busting.
- Tabla comparativa reorganizada por áreas con etiquetas en ES/EN. Los grupos de capacidades respetan flags reales.
- Eliminados los tres textos finales no deseados del bloque Planes. Ajustada la distancia con FAQ.

## No modificado
- Precios, suscripciones, pasarelas de pago, comisiones Partners, backend de generación, validación de límites y facturación.

## Validación local
Ejecutar `node --check static/commercial-offer-v2734.js`, `python tools/v2734/verify_manifest.py`, `go test ./...`, `go build ./...` desde PowerShell con Go 1.26.

**Precaución:** V27.34.3 necesita `commercial_public_offer_v2734.go` actualizado para entregar `studio_credits_planned`. No publicar solo el frontend; de lo contrario la web mostraría un saldo previsto de cero.
