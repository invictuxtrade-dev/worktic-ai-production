# WorkticAI V27.35.10 - Reparacion de planes publicos

## Causa confirmada en el codigo

La landing llama `GET /api/public/offer/v2734` sin sesion. La ruta estaba registrada en main.go, pero `authMiddleware` no la incluia entre las rutas publicas. En una visita anonima respondia HTTP 401, lo cual activaba el mensaje de indisponibilidad y ocultaba los precios.

## Cambio

Se permite **unicamente** `GET /api/public/offer/v2734` sin sesion. El backend sigue calculando precios y beneficios desde `billing_plans` y `commercial_rules_v2721`. No se abren las API administrativas, compras, usuarios, creditos ni pagos. POST y subrutas adicionales continuan protegidos. No se cambia el esquema ni ninguna suscripcion.

## Pruebas de regresion

Se agrego `public_offer_auth_v273510_test.go` con rutas permitidas y bloqueadas. Ejecutar en Windows con Go 1.26:

```
go test -count=1 ./...
go build ./...
```

## Verificacion luego del despliegue

1. Abrir `https://www.workticai.com/api/public/offer/v2734` en incognito. Debe mostrar JSON con `plans`; nunca debe retornar `sesion requerida` (HTTP 401).
2. Verificar codigos `free`, `personal`, `business`, `enterprise` (si se encuentran activos y configurados).
3. Abrir `https://www.workticai.com/#planes` en incognito y recargar sin cache; confirmar precios correctos antes de habilitar compras reales.
4. Una llamada sin sesion a `/api/admin/plans` debe seguir respondiendo HTTP 401.

Si la ruta ya no responde 401 pero retorna 503, revisar logs de Render: el segundo error seria esquema/consulta de catalogo. Si retorna 200 con `plans: []`, revisar `commercial_rules_v2721`, sin inventar precios ni forzar la venta.

Actualizacion sobre V27.35.9, sin cambios en Render Environment.
