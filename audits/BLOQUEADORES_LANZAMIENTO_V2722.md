# Bloqueadores de lanzamiento, V27.22

Estado: **desarrollo aislado; no publicar ni activar comercialización**.

La existencia de una ruta auditada o una puerta de permiso NO prueba que esté costeada, que su cuota sea segura frente a concurrencia, o que todo su acceso a datos esté limitado al tenant.

## P0 — Antes de vender planes

1. **Reglas legacy globales**: `worktic_auto_rules` en `main.go` no contiene `tenant_id`; `rulesHandler` lee y escribe globalmente y `quotaAllowed(kind=rules)` cuenta globalmente. No anunciar cuotas de reglas independientes por empresa hasta migrar los datos legacy y cambiar todas sus rutas de lectura/escritura/ejecución. Migración automatizada de esta tabla puede asignar erróneamente reglas históricas; requiere estrategia de propietario y regresión WhatsApp.
2. **Costos externos incompletos**: llamadas de Meta (WhatsApp, Messenger, Ads, leads), Telegram, Google/YouTube, TikTok, LinkedIn, OpenAI, Luma. Los reportes disponibles estiman parte de Studio/Core; faltan facturas reales, tokenización efectiva, cargos de mensajería por plantilla/país, egress, API quotas. Dejar claro quién paga la pauta (anunciante) y mensajería variable.
3. **Rutas heredadas**: endpoints `/api/send`, `/api/messages`, `/api/rules`, `/api/connect`, y bot WhatsApp histórico requieren revisión completa tenant/roles, seguridad y tarifas. V27.22 no afirma cubrirlos con nuevas puertas.
4. **Pruebas transaccionales PostgreSQL real**: 20+ peticiones paralelas para créditos, cupos de workflow, suscripciones, pagos, cancelaciones. El límite piloto de workflows utiliza consulta antes del INSERT y aún requiere bloqueo transaccional, no se promete como control anti-carrera.
5. **Facturación y pagos**: verificación de transacciones, token/red correctos, reembolsos/chargebacks, cupones, anualidad, vigencia, cambios de plan, referidos y comisiones Partners hasta 50%.
6. **Planes**: confirmar los precios vigentes de `billing_plans` de la BD de staging, derechos `features_json`, cupos reales por módulo y textos de oferta del sitio comercial. Evitar prometer funciones antes de verificar endpoints reales.
7. **Poder de producción**: staging separado con PostgreSQL/Redis, backup y tests E2E; no conectar APIs pagadas ni enviar mensajes reales en pruebas automáticas.

## P1 — Control de consumo por tenant

- Ads Center: límites de conexiones y frecuencia de sincronización, almacenamiento histórico y gastos de integraciones.
- Analytics: límites de eventos de ingestión, retención, exportaciones y consultas SQL costosas.
- Social Hub: contenido generado, publicaciones en masa, cargas, reintentos y API rate limits.
- CRM, Catálogo, Agenda, Landings: cuotas atómicas y tenant-safe en cada creación, importación y archivo; eliminación y reapertura.
- WhatsApp marketing, Meta leads, grupos: recargos por país/categoría/plataforma, destinatarios, límites anti abuso y responsabilidades de pago.
- Automatizaciones: cap de ejecuciones mensuales, fan-out/retries y acciones de pago.
- Partners: liquidación real de comisión, reversos, mínimo retiro y transacciones idempotentes.

## Criterios de aceptación mínimos

Cada módulo debe tener (a) derecho contratado; (b) validación backend que devuelve 403 cuando esté deshabilitado; (c) cuota que resiste concurrencia; (d) medición por empresa; (e) costo del proveedor o asignación de infraestructura; (f) conciliación de fallos y reembolsos; (g) E2E Free/Personal/Negocio/Empresa en PostgreSQL; (h) prueba tenant A no accede datos de tenant B; (i) prueba de regresión de controles anteriores.

V27.22 introduce la matriz de puertas piloto para 17 módulos y clasifica 198 registros del router; **aún NO cumple todos los criterios anteriores**.
