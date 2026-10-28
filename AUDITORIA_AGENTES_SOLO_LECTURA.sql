-- EJECUTAR SOLO SOBRE COPIA DE RESPALDO O CON ACCESO SELECT.
-- SQL sin DELETE/UPDATE. No reasignar agentes automáticamente por coincidencias de IDs.
-- Agentes sin empresa válida
SELECT a.id AS agent_id,a.tenant_id,a.name,a.status,a.created_at
FROM ai_agents a LEFT JOIN tenants t ON t.id=a.tenant_id
WHERE t.id IS NULL ORDER BY a.id;

-- Conexiones asignadas a agentes inexistentes o de otra empresa
SELECT c.id AS connection_id,c.tenant_id AS channel_tenant,
 c.type,c.assigned_agent_id,a.tenant_id AS agent_tenant,a.status AS agent_status
FROM channel_connections c LEFT JOIN ai_agents a ON a.id=c.assigned_agent_id
WHERE c.assigned_agent_id<>0 AND
 (a.id IS NULL OR a.tenant_id<>c.tenant_id OR a.status<>'active');

-- Rutas que seleccionan agentes ajenos, borradores o inexistentes
SELECT r.id AS route_id,r.tenant_id AS route_tenant,r.agent_id,
 a.tenant_id AS agent_tenant,a.status
FROM ai_agent_routes r LEFT JOIN ai_agents a ON a.id=r.agent_id
WHERE a.id IS NULL OR a.tenant_id<>r.tenant_id OR a.status<>'active';

-- Asignaciones de permisos entre empresas
SELECT p.id,p.tenant_id,p.agent_id,a.tenant_id AS agent_tenant,
 p.user_id,u.tenant_id AS user_tenant
FROM ai_agent_permissions p
LEFT JOIN ai_agents a ON a.id=p.agent_id
LEFT JOIN app_users u ON u.id=p.user_id
WHERE a.id IS NULL OR u.id IS NULL OR a.tenant_id<>p.tenant_id OR u.tenant_id<>p.tenant_id;

-- Borradores/posibles agentes de migración repetidos. Inspección manual.
SELECT tenant_id,name,status,COUNT(*) AS copies,MIN(id) AS earliest,MAX(id) AS latest
FROM ai_agents GROUP BY tenant_id,name,status HAVING COUNT(*)>1 ORDER BY copies DESC;

-- Chat Web que apunta a un agente incorrecto
SELECT c.id,c.tenant_id,c.agent_id,a.tenant_id AS agent_tenant,a.status
FROM mobile_web_channels_v1 c LEFT JOIN ai_agents a ON a.id=c.agent_id
WHERE a.id IS NULL OR a.tenant_id<>c.tenant_id OR a.status<>'active';

-- Llamadas IA vinculadas a agente incorrecto
SELECT c.id,c.tenant_id,c.agent_id,a.tenant_id AS agent_tenant,a.status
FROM voice_channels_v2735 c LEFT JOIN ai_agents a ON a.id=c.agent_id
WHERE a.id IS NULL OR a.tenant_id<>c.tenant_id OR a.status<>'active';
