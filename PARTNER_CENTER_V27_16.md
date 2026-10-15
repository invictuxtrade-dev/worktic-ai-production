# WorkticAI V27.16 — Partner Certified Services

## Arquitectura

Partner Center continúa como backoffice independiente dentro de la misma cuenta y sesión WorkticAI. El CRM sigue siendo la fuente de verdad de prospectos, oportunidades comerciales y clientes. Partner Center gestiona atribución, red, Momentum, comisiones, certificación, Co-Sell, payouts y trabajo profesional asignado a Partners.

## Cuatro formas de generar ingresos

1. **Share — Recomienda y gana.** Enlace Partner atribuido y comisión directa según Momentum; las renovaciones elegibles aplican la regla configurada.
2. **Sell — Construye cartera.** Venta activa usando CRM/Copilot y Co-Sell cuando dos Partners acuerdan trabajar un cierre.
3. **Activate Certified — Conviértete en especialista.** Certificación profesional en implementación, automatización y Growth con WorkticAI. Desbloquea proyectos pagados asignados por WorkticAI, bono de adopción y acceso a Human Sales.
4. **Lead — Desarrolla comunidad.** N2/N3 según rango Momentum y mentoría cuando se cumplen las reglas del programa.

No son cuatro membresías: son capacidades que pueden convivir en el mismo perfil Partner.

## Activate Certified

La interfaz explica claramente que certifica al Partner como **Especialista WorkticAI en Implementación, Automatización y Growth** y valida dominio práctico de:

- CRM, pipeline y seguimiento;
- Inbox, WhatsApp y canales;
- Agentes IA;
- automatizaciones;
- Growth y personalización de marketing;
- Analytics y optimización.

La certificación habilita un perfil profesional validado para recibir proyectos de empresas/clientes que necesitan apoyo para configurar, personalizar y operar WorkticAI.

## Oportunidades asignadas por WorkticAI

Las oportunidades de implementación/asistencia **no se toman libremente de un marketplace**. El Superadmin/WorkticAI selecciona y asigna el especialista certificado apropiado.

Flujo:

`WorkticAI valida necesidad -> Admin selecciona especialista -> Asigna alcance + pago -> Especialista recibe oportunidad -> Acepta -> Trabajo inicia -> Entrega -> Revisión -> Completado/pago`

Estados:

`assigned -> in_progress -> submitted -> completed`

Cierres alternativos:

`rejected` o `cancelled`.

Antes de aceptar, el especialista ve:

- empresa/cliente;
- tipo de proyecto;
- objetivo y contexto;
- alcance;
- entregables;
- fecha límite;
- prioridad;
- pago/compensación;
- nota del administrador.

Aceptar cambia inmediatamente el proyecto a `in_progress`. Al terminar, el especialista envía una nota de entrega y pasa a `submitted`. El administrador revisa y completa. En proyectos de pago fijo, la finalización crea un crédito auditable `service_assignment` en el ledger de comisiones Partner.

## Human Sales 80/20

Human Sales permanece separado de las asignaciones profesionales. En Human Sales el especialista trabaja como agente humano de ventas y el reparto 80/20 se calcula sobre el pool de comisión pactado con la empresa. Esto evita mezclar comisiones comerciales con pagos por implementación, personalización o acompañamiento.

## Agentes IA y acciones destructivas

Todos los agentes muestran ahora la acción `Eliminar`. Si se elimina el agente principal, WorkticAI promueve automáticamente otro agente disponible y reasigna las conexiones que apuntaban al anterior. El historial de auditoría no se borra.

Las acciones destructivas de la aplicación usan el patrón visual común: botón rojo con texto blanco.
