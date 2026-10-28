# WorkticAI V27.35.14 - Menu clasico ordenado

**Base obligatoria:** V27.35.13 (Navigacion Premium).  **Alcance:** solo navegacion visual.

## Cambios
- Devuelve el menu lateral plano (sin encabezados de grupo ni acordeones) con el mismo diseno anterior.
- Reordena los 27 botones originales, sin modificar cada elemento, su icono, data-view, permisos, estado oculto ni traducciones.
- Desactiva las cargas del JS y CSS de navegacion premium V27.35.13. Los archivos fisicos pueden permanecer en el repositorio, pero no se ejecutan.
- Conserva la cabecera, barra de creditos, footer, todas las vistas, APIs, roles y la base de datos.

## Nuevo orden
Resumen, Conversaciones, Agentes IA, Worktic Studio, Contactos, Oportunidades, Agenda, Catalogo, Canales, Chat Web (sigue oculto), Llamadas IA (sigue oculto), WhatsApp Business, Automatizaciones, Social Hub, WhatsApp Marketing, Ads Center, Growth & Campanas, Landing Pages, Grupos & Comunidades, Analytics, Billetera de Creditos, Plan y membresia, Equipo, Partner Center, Mi perfil, Administracion Worktic, Guias.

Los canales Chat Web y Llamadas IA continuan configurandose desde Canales, igual que en la base V27.35.13; sus accesos individuales conservan el atributo `hidden`.

## Instalacion
Se recomienda usar `INSTALAR_MENU_CLASICO_V273514.ps1` como se indica en el chat. Requiere rama limpia y `HEAD=origin/main`, y compara huellas del archivo original.

## Verificaciones recomendadas
- Validar el orden, las 27 entradas, logo, iconos y footer.
- Abrir Resumen, Conversaciones, Agentes IA, Studio, Canales y Billetera.
- Probar escritorio y movil, sin grupos o menus desplegables.
- Verificar que el menu respete permisos reales para usuario normal y administrador.

No hay cambios Go ni migraciones de base de datos. `go test ./...` y `go build ./...` deben ejecutarse antes del despliegue.
