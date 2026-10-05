# WorkticAI V27.1 — Organización UX + Copilot Pro

Actualización incremental sobre V27. No requiere migración de base de datos ni variables de entorno nuevas.

## Cambios principales

- Dashboard limpio: WhatsApp Marketing deja de aparecer en el resumen general.
- Estado operativo reorganizado en 2x2 para evitar desbordes.
- Actividad reciente con búsqueda, paginación y 5 eventos por página.
- WhatsApp Marketing reorganizado: una sola acción principal "Nueva campaña" dentro de su módulo y sin CTAs duplicados.
- Worktic Copilot ampliado con modos de ayuda y catálogo de soluciones para todos los módulos principales.
- Contexto del Copilot enriquecido con canales, redes sociales, Ads, CRM, formularios, landings, campañas, agenda, automatizaciones, agentes, mensajes y facturación.
- Deep links ampliados para navegar al módulo correcto desde las respuestas.

## Despliegue

1. Copiar V27.1 sobre el repositorio actual conservando `.git`.
2. Ejecutar `gofmt -w *.go` si se desea normalizar todo el Go.
3. Ejecutar `go test ./...` con Go 1.26.x.
4. `git add -A`
5. `git commit -m "WorkticAI V27.1 UX and Copilot Pro"`
6. `git push origin main`
7. Desplegar el último commit en Render.

No ejecutar `/app/worktic-migrate`: V27.1 no agrega tablas nuevas.
