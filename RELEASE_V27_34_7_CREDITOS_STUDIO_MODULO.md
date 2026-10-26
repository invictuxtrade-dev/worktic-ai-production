# WorkticAI V27.34.7 — Créditos Studio (módulo independiente)

## Cambios
- Nuevo ítem lateral **Créditos Studio**, junto a Worktic Studio.
- Vista propia de billetera; edición Image/Video y proyectos quedan limpios en Studio.
- Saldo y botón + del encabezado abren la misma billetera; editor incluye acceso «Créditos».
- La sección técnica «Lo que incluye tu plan» y «Mis límites y consumo» sale del Studio y queda en Plan y membresía, plegada bajo detalle técnico.
- Movimientos se consultan en `/api/commercial/movements`, sin ejemplos falsos; recargas solo si `topups_enabled`.
- Permisos, créditos, reservas, presupuestos, pagos y lógica backend no cambian.
- Español/inglés añadido para el nuevo menú y título.

## Instalación
Aplicar sobre V27.34.6 validada. Parche incremental con los archivos especificados. Verificar `node --check` de JS modificados, `go test ./...`, `go build ./...` y navegación de las vistas en el navegador. Realizar backup antes de producción.

## Alcance de las pruebas
Validación estática JS/HTML y navegación de navegador con backend simulado. No certifica operaciones reales de PostgreSQL, OpenAI, Luma, pagos ni pruebas concurrentes en Render. No habilitar cobros/generaciones por el rediseño.
