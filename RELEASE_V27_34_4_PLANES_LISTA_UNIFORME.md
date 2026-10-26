# WorkticAI V27.34.4 — Lista de beneficios uniforme

Base exacta: V27.34.3. Parche solo para presentación pública y resumen del checkout.

- Retirados los chips Growth / Landing / Agentes IA y la tarjeta destacada de Worktic Studio.
- Todos los beneficios — Growth, Landing Pages, Agentes IA, límites CRM, Studio y créditos previstos — aparecen juntos bajo **Capacidades de tu plan** con renglones del mismo estilo.
- Funciones no incluidas se muestran con una raya gris, las disponibles con el mismo check morado existente.
- Los créditos **previstos** conservan la advertencia “aún no activos” para no prometer una asignación comercial no habilitada.
- Sin cambios al catálogo de backend, precios, límites, suscripciones, pago, Partners, datos o procesos de IA.
- Cache bust de CSS/JS `27344` tanto en landing como en backoffice.

## Aplicar
Extraer el parche en carpeta temporal y copiar los archivos respetando estructura (véase instrucciones de entrega).
Validar `python tools/v2734/verify_manifest.py` y `node --check static/commercial-offer-v2734.js`; compilar y probar el proyecto antes de publicar.
