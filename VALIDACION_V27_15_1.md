# Validación WorkticAI V27.15.1 — Partner Workspace UX

Base usada: `WorkticAI_V27_15_Rebased_Current_Repo.zip`.

## Alcance
Actualización exclusivamente visual/UX del Partner Center.

- CRM principal colapsado a iconos al entrar a Partner Center en desktop.
- Partner rail completo conservado.
- Workspace Partner ampliado.
- Controles: Expandir/Colapsar CRM, Modo enfoque Partner, Pantalla completa y Volver al CRM.
- Partner menu tipo drawer en tablet/móvil.
- Cards a una columna en <=900 px.
- Árbol de red con canvas scrollable ampliado.
- Tablas con scroll horizontal táctil.

## No modificado
- Backend Go.
- Esquema / PostgreSQL / Redis.
- Comisiones / Momentum / Co-Sell / Human Sales / payouts.
- CRM y demás módulos funcionales.
- `static/index.html`, `static/partners.html`, `static/marketing-v2713.css`, `static/marketing-v2713.js`.

## Validaciones
Ejecutar antes de push:

```powershell
node --check .\static\partners-v2715.js
git diff --check
git status
```

No ejecutar `/app/worktic-migrate`.
