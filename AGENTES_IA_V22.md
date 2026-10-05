# WorkticAI V22 — Agentes IA 2.0

V22 convierte el módulo unificado de Agentes IA en una capa operativa conectada con Inbox V21 y Automation Engine V20.

## Incluye
- Agente Principal + especialistas.
- Perfil operativo por agente.
- Memoria persistente y aislada por tenant + agente + cliente.
- Base de conocimiento empresarial compartida o específica por agente.
- Capacidades configurables: catálogo, CRM, agenda, pipeline, tareas y handoff humano.
- Palabras de escalado y política de handoff.
- Vista de memoria y borrado controlado.
- Integración del contexto V22 en respuestas reales de WhatsApp QR, Telegram y Messenger.
- Compatibilidad con el handoff IA/humano de V21.
- Compatibilidad con invocaciones desde workflows V20.

## Nuevas tablas
- ai_agent_profiles_v22
- ai_agent_memory_v22
- ai_agent_knowledge_v22
- ai_agent_tool_audit_v22

Las tablas se crean automáticamente al iniciar. No requiere SQL manual.

## Nuevas rutas
- GET /api/agents/v22/overview
- GET/POST /api/agents/v22/profile?agent_id=ID
- GET/POST/DELETE /api/agents/v22/knowledge
- GET/DELETE /api/agents/v22/memory

## Variables
V22 no agrega variables de entorno nuevas.

## Nota de privacidad
La memoria se almacena por tenant + agente + contact_key. El equipo puede borrar memorias individuales desde Agentes IA > Memoria.
