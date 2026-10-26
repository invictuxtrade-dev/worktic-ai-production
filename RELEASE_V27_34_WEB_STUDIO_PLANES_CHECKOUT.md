# WorkticAI V27.34 — Web comercial + Studio + precios y checkout coherentes

**Base exacta:** ZIP V27.33 Production Candidate. **Estado:** desarrollo local, **sin publicar**.

## Desarrollo

- Conserva estructura original SEO, hero, secciones, animaciones, contactos y programa Partners.
- Nueva sección **Worktic Studio** con vista conceptual CSS, editor de imagen/video, videos 9:16 y Galería/Social Hub; no afirma que todos los modelos IA estén activados.
- Los cuatro planes se leen desde `/api/public/offer/v2734`, sin precios codificados en el frontend. El servidor lee `billing_plans`, derechos de `commercial_rules_v2721` y elegibilidad de `commercialModuleCatalogV2722`.
- La respuesta IA pública usa el **mínimo del cupo nuevo y el legado** cuando está activo `WORKTIC_COMMERCIAL_ENABLE=1`; desactivado conserva la cuota anterior y no promete créditos IA activados.
- Precios mensuales/ anuales calculados con la misma fórmula del checkout: tarifa admin * 12 * 0.8 para el anual (pago único). La moneda realmente exigida por el backend es **USDT** (evita texto comercial que prometa un cargo distinto).
- Reemplaza listas históricas de 10.000 respuestas / 5 canales en Negocio que no coinciden con los límites efectivos.
- Checkout de la app recibe **la misma oferta** y presenta beneficios exactos antes del formulario de pago; reserva y validación de pagos se mantienen del backend V27.28.
- Comparativa desplegable, bilingüe ES/EN, sección informativa de recargas solo cuando está activado el servicio; planes no verificables deshabilitan CTAs públicas.
- `static/i18n.js` incorpora textos EN/ES. Menú móvil conserva navegación y agrega Studio.

## Qué NO hace / condiciones

- No cambia precios, paquetes ni comisiones de Worktic Partners.
- No activa `WORKTIC_COMMERCIAL_ENABLE` ni flags de pasarela.
- La API **no** sustituye los snapshots contractuales de clientes activos; refleja oferta de compras nuevas.
- La elegibilidad teórica de un módulo no garantiza permisos OAuth, integración externa ni QA completo; la web indica que esas funciones requieren activación y las cuotas son por empresa.
- No prueba la conciliación USD/USDT, tokens OpenAI, pagos on-chain, PostgreSQL concurrente ni Luma real.
- No basta un ZIP visual para autorizar venta: staging debe aprobar la matriz de 16 escenarios de V27.33 y verificar que cliente reciba exactamente lo comprado.
- El manifiesto V27.33 es histórico; **NO usar su verificador contra el código modificado V27.34**. Ejecutar `python tools/v2734/verify_manifest.py`.

## Verificaciones en VS Code con PowerShell (en carpeta raíz)

```powershell
node --check static/commercial-offer-v2734.js
node --check static/i18n.js
node --check static/app.js
python tools/v2734/verify_manifest.py
go version
go test ./...
go build ./...
```

## QA en staging (sin datos reales)

1. PostgreSQL propio, distinto de producción; importar las migraciones aditivas.
2. API pública `GET /api/public/offer/v2734` responde 4 planes, límites administrables y ningún dato interno privado.
3. Actualizar precio/cuota de prueba del plan en superadmin, y comprobar cambio en web, comparativa y checkout.
4. Alternar mes/año y confirmar importe **idéntico** al backend para cada plan y al checkout.
5. Con `WORKTIC_COMMERCIAL_ENABLE=0` no debe publicitar créditos IA como incluidos ni permitir recargas.
6. Con la bandera `1` **solo staging**, mostrar cupo real y paquetes habilitados, verificar saldo y todas las modalidades de compra/renovación con datos simulados.
7. Probar contrato histórico versionado: derechos vigentes de compradores antiguos NO se alteran por editar oferta nueva.
8. Probar en Chrome/Edge escritorio y Android móvil; registro/login → Membresía → checkout completo → pago pendiente verificado por admin.
9. Certificar QA Go 1.26, PostgreSQL/Redis, Meta/WhatsApp, IA, Partners y costos con la puerta de lanzamiento. Ninguna acción en producción.

## Cambios locales en V27.34

- Modificados `VERSION.txt`, `main.go`, `static/index.html`, `static/app.html`, `static/app.js`, `static/i18n.js`.
- Nuevos `commercial_public_offer_v2734.go`, `static/commercial-offer-v2734.css`, `static/commercial-offer-v2734.js`, release notes, `tools/v2734/verify_manifest.py`, `RELEASE_MANIFEST_V2734.json`.
