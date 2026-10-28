# WorkticAI V27.35.13 — Navegación premium por procesos

Base exacta: **V27.35.12**. Solo cambia el menú del backoffice; no modifica rutas, endpoints, tablas, permisos del servidor, créditos o facturación.

## Nueva estructura

**Accesos principales:** Resumen, Conversaciones, Agentes IA, Worktic Studio.

**CRM y Ventas:** Contactos, Oportunidades, Agenda y Catálogo.

**Canales y Automatización:** Canales conectados, WhatsApp Business (configuración avanzada), Automatizaciones. Chat Web y Llamadas IA continúan gestionándose dentro de Canales y no se agregan como enlaces visibles independientes.

**Marketing y Crecimiento:** Social Hub, WhatsApp Marketing, Ads Center, Growth y Campañas, Landings y Funnels, Grupos y Comunidades.

**Analítica y Finanzas:** Analytics, Billetera de Créditos y Plan y membresía.

**Empresa y Recursos:** Equipo, Partner Center, Mi perfil y Guías y ayuda.

**Administración Worktic:** acceso independiente, con la misma regla de visibilidad existente para superadministración.

## Funcionalidad y compatibilidad

- Los 27 botones originales de navegación conservan sus `data-view`, identificadores DOM, controles de permisos y eventos. No se clonan, solo se mueven a grupos nativos `details`.
- Cada grupo puede expandirse/contraerse; abrir uno cierra los demás. La última sección abierta se recuerda en `sessionStorage` del navegador.
- Si una acción de otro módulo abre una vista, su grupo se despliega automáticamente.
- La barra lateral oscura, el contenido de las páginas, el logo, el buscador, el saldo de créditos combinado y el Copilot no se rediseñan.
- La leyenda redundante del pie del menú se oculta visualmente, conservando su nodo para la barra de perfil.
- Accesibilidad: encabezados nativos `summary`, navegación mediante teclado, títulos y estado de página; traducción ES/EN de grupos y los títulos ajustados.
- Móvil: cajón lateral completo y scroll interno. Corrección de la superposición del encabezado móvil.

## Cambios técnicos

- `VERSION.txt`: 27.35.13.
- `static/app.html`: referencias a dos archivos nuevos, sin modificar el menú de origen ni los scripts anteriores.
- `static/navigation-premium-v273513.js`: organización en grupos conservando referencias a los botones originales.
- `static/navigation-premium-v273513.css`: estilos acotados al menú.

## Comprobación antes de publicar

```
node --check static/navigation-premium-v273513.js
go test -count=1 ./...
go build ./...
git diff --check
```

**Escenarios probados en un navegador local aislado:** 1366x768, 920x768, 390x844; apertura y cierre de grupos, clic de navegación, conservación de los 27 IDs de vista y ausencia de excepciones JavaScript. No se probó el backend real, porque este entorno tiene Go 1.23.2 y el repositorio exige Go 1.26.0.

**Tras desplegar:** verificar con una cuenta normal y con superadministrador, así como la navegación desde el buscador, Studio y las rutas profundas. Los módulos no autorizados deben seguir protegidos por el servidor.
