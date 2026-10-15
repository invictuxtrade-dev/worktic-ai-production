# Validación WorkticAI V27.11 — Commercial Expansion + Worktic Partners Preview

## Base
WorkticAI V27.10.1 — SEO-Preserved Commercial Enhancement.

## Principio de esta entrega
- Se conserva el SEO, título, meta description, H1, lead y CTAs principales de la web comercial existente.
- No se modifica backend, PostgreSQL, Redis, billing anual, Copilot ni módulos internos.
- La expansión es frontend/comercial e informativa.

## Añadido a la web principal
- Social Hub y conexiones sociales explicadas en profundidad.
- Animación de orquestación social.
- WhatsApp Marketing destacado con segmentación, templates, programación, métricas y opt-out.
- Agentes IA + Automation Engine explicado mediante flujo animado.
- Mapa de módulos de la plataforma.
- Worktic Partners enlazado como programa informativo.
- Precios públicos expresados en USD.
- Nota de transparencia sobre medios de pago y total final.
- Burbuja WhatsApp profesional para Ventas y Soporte.
- Enlace visible a la SIC / protección al consumidor en el footer de la web comercial.
- WhatsApp usado: +57 333 245 0481, número público de Worktic; ventas y soporte usan mensajes prellenados diferentes.

## Worktic Partners
Nueva página: `static/partners.html`.

Incluye:
- Share / Sell / Activate / Lead.
- Worktic Momentum.
- Directa / N2 / N3.
- adopción, renovación y mentoría.
- techo combinado de 50%.
- Worktic Partner Autopilot (concepto, no implementado en CRM aún).
- Worktic Co-Sell.
- Worktic Impact Score.
- simulador educativo dinámico.
- pre-lanzamiento informativo y advertencias de no garantía de ingresos.
- enlaces a fuentes regulatorias oficiales.

## Arquitectura financiera recomendada
Ver `WORKTIC_PARTNERS_FINANCIAL_LEGAL_FRAMEWORK.md`.

Puntos clave:
- INE como base comisionable.
- 50% como techo, no payout automático.
- no compresión de bonos no asignados.
- annual billing reconocido mes a mes para comisiones.
- separación entre compra de SaaS y derecho a comisionar.
- chargeback/reversos y payout auditado.
- guardrail de economía unitaria por plan antes de publicar/activar porcentajes.

## Archivos nuevos/modificados
- `static/index.html`
- `static/i18n.js`
- `static/marketing-v2711.css`
- `static/marketing-v2711.js`
- `static/partners.html`
- `static/partners-v2711.css`
- `static/partners-v2711.js`
- `static/assets/v2711/worktic-social-orchestration.mp4`
- `static/assets/v2711/worktic-agent-automation-flow.mp4`
- `WORKTIC_PARTNERS_FINANCIAL_LEGAL_FRAMEWORK.md`
- `VERSION.txt`
- `VALIDACION_V27_11.md`

## No modificado
- Go/backend.
- checkout/billing anual V27.10.
- login.
- app interna.
- Copilot.
- DB/migraciones.
- infraestructura Render.
