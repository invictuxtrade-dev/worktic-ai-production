# Worktic V12 Multiagent Premium

## Incluido

- CRUD de agentes por tenant.
- Tipos: general, ventas, soporte, agenda, campañas, comunidad y personalizado.
- Agente principal de respaldo.
- Límites por plan: Free 1, Personal 2, Negocio 5, Empresa 15.
- Estado borrador, activo o pausado.
- Conocimiento, instrucciones, herramientas, canales y presupuesto por agente.
- Simulador individual conectado a la clave central de OpenAI.
- Enrutamiento por prioridad, canal, intención, palabra, campaña, landing o grupo.
- Métricas mensuales por agente.
- Permisos específicos por integrante.
- Auditoría de creación, edición, prueba y eliminación.
- Interfaz móvil integrada.

## Flujo recomendado

1. Crear y activar un agente principal.
2. Crear agentes especializados.
3. Definir herramientas y fuentes de conocimiento.
4. Crear reglas de enrutamiento específicas.
5. Probar cada agente.
6. Asignar permisos al equipo.
7. Revisar métricas y consumo.

## Nota de producción

La selección multiagente está implementada en el backend mediante `resolveAgent`. Para que el enrutamiento opere sobre mensajes reales de múltiples empresas, cada conexión de canal debe entregar el `tenant_id` correcto. La arquitectura heredada todavía conserva una conexión global de WhatsApp/Telegram, por lo que antes de una operación SaaS pública debe completarse la separación física de clientes de canal por tenant.
