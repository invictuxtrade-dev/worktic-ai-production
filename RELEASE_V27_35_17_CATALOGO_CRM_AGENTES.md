# WorkticAI V27.35.17 — Productos y oportunidades CRM (por agente)

**Base estricta:** V27.35.16.2, después del hotfix PostgreSQL `BYTEA`.

## Cambios

- Mantiene «Agregar a oportunidad» en Catálogo; ya no necesita que el usuario conozca IDs.
- Dentro de **CRM > Oportunidades > Editar**, muestra los productos asociados, permite añadir más productos y quitar asociaciones. No modifica precios ni stock.
- Cada agente puede activar la opción **Asociar productos al CRM automáticamente** en **Agentes IA > Capacidades V22 > [agente]**. **DESACTIVADO por defecto.**
- Para actuar, el agente debe estar activo, tener permiso de Catálogo y CRM en su perfil y disponer de las herramientas `catalog` y `opportunity`.
- WhatsApp, Telegram, Messenger y Chat Web comparten la misma validación; una mención explícita de un producto activo y disponible, junto con intención comercial reconocible, se asocia a la oportunidad abierta del contacto. Si falta contacto CRM, no modifica nada.
- Si la persona ya tiene una oportunidad abierta, se reutiliza; no se crean duplicados de la asociación producto-oportunidad. Las oportunidades cerradas se respetan.
- Registro auditado de asociaciones automáticas y retiradas manuales por empresa, sin registrar el texto íntegro de la conversación.

## Límites de este alcance

- La asociación comercial no registra pagos, no interpreta comprobantes y **no descuenta inventario**.
- La detección conservadora exige que el cliente nombre un producto de forma reconocible: las preguntas genéricas no crean asociaciones.
- No se envían nuevos mensajes ni campañas de seguimiento de manera automática. Eso requiere un flujo de automatización con consentimiento y reglas propios.
- Las comprobaciones de PostgreSQL, de canales reales y la compilación con **Go 1.26** deben superarse en Windows/Render antes de producción. Este entorno no dispone del toolchain Go 1.26.
- Las pruebas de concurrencia entre varias instancias para la creación simultánea de oportunidades siguen pendientes: se reutiliza el motor de detección CRM existente.

## Validación sugerida

1. Desactivar la asociación automática en el agente y consultar «Plan Negocio»: no debe vincularlo.
2. Activar solo ese agente (con Catálogo/CRM y herramientas) y repetir desde un chat asociado a un contacto de la misma empresa.
3. Confirmar un solo artículo dentro de la oportunidad correcta tras repetir la conversación.
4. Probar mensaje genérico y producto agotado: no crear vínculo.
5. Intentar cruzar empresas y probar otro agente sin permiso: no producir asociaciones.
6. Revisar que las ventas, comprobantes, precios y stock no se modifican.
7. Probar añadir y quitar varios productos desde CRM, manteniendo historial de auditoría.

## Corrección del paquete de pruebas — 2026-10-29

Se corrigió únicamente el esquema SQLite usado por TestCatalogAutoLinkIdempotentV273517.
El fixture anterior omitía los campos `phone` y `email` de `crm_contacts` y los campos
`source`, `source_ref`, `channel`, `probability`, `updated_at`, `closed_at` y
`automation_state` de `crm_opportunities`, aunque el código productivo los consulta.

La lógica de producción, permisos, stock y ventas es idéntica a V27.35.17.
El instalador sigue requiriendo como base V27.35.16.2. Si alguna prueba Go falla,
restaura los archivos anteriores. La compilación integral debe confirmarse
con Go 1.26 en Windows antes de publicar.
