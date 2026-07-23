# Worktic AI V13.0.2 — Corrección de registro y sesión

- El registro ahora crea usuario, tenant, membresía y plan Free en una sola transacción.
- Se evita que una cuenta recién creada quede con tenant_id=0.
- Los errores de configuración de tenant ya no se reportan como sesión vencida.
- La aplicación valida la sesión antes de cargar el dashboard.
- Se evita el bucle infinito entre login.html y app.html.
- Se añadió el import faltante de io en agents.go.
