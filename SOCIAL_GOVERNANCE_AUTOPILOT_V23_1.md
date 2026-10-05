# WorkticAI V23.1 — Social Governance & Autopilot

## Objetivo

V23.1 corrige una decisión de producto importante de V23: **las aprobaciones no son obligatorias ni dependen del tamaño de la empresa**. Cada tenant decide cómo quiere operar Social Hub.

Esta versión se construye sobre V23 y continúa siendo parte de la rama acumulativa que no debe desplegarse todavía de forma aislada.

## Modos de operación

### 1. Autopilot

- Publicar/programar no requiere revisión humana por defecto.
- Puede ser usado por una pyme, una empresa mediana o una gran organización.
- Se respetan excepciones configuradas por red/campaña y palabras bloqueadas.
- La pausa global detiene la salida automática/programada.

### 2. Híbrido

- Worktic opera automáticamente por defecto.
- Pasa a aprobación cuando una regla lo exige.
- Reglas disponibles:
  - red específica en modo aprobación;
  - campaña específica marcada para revisión;
  - palabras sensibles;
  - solicitud manual de aprobación desde Composer.

### 3. Aprobación

- Todo intento de publicar o programar se convierte en `pending_approval`.
- El usuario puede seguir creando/editando borradores.
- Un aprobador autorizado decide aprobar/rechazar.

## Reglas por red

Cada red puede:

- heredar el modo general;
- forzar Autopilot;
- forzar Aprobación.

Redes contempladas:

- Facebook
- Instagram
- TikTok
- YouTube
- LinkedIn
- Telegram

## Reglas por campaña

La pantalla de Gobernanza carga las campañas reales del tenant y permite marcar campañas que siempre requieren aprobación. No se obliga al usuario a conocer/copiar IDs manualmente.

## Palabras sensibles y bloqueadas

### Sensibles

En modo Híbrido, si el contenido contiene una palabra configurada como sensible, la publicación pasa a revisión.

Ejemplos:

- cambio de precio
- descuento especial
- comunicado oficial
- términos legales

### Bloqueadas

Estas palabras impiden publicación automática independientemente del modo.

El bloqueo se valida también en backend antes de enviar al proveedor.

## Pausa global de emergencia

Botón disponible para owner/admin:

- `Pausar automatización`
- `Reanudar automatización`

Al pausar:

- los posts programados/queued que lleguen al publisher quedan `governance_hold`;
- no salen hacia los proveedores;
- se conserva el contenido y el historial.

Al reanudar:

- los posts retenidos específicamente por `GOVERNANCE_PAUSE` vuelven a cola;
- los bloqueados por reglas de contenido no se liberan automáticamente salvo que se cambien esas reglas.

## Enforcement en backend

La política no depende solo de la interfaz.

Se aplica en:

- creación de publicaciones;
- edición de publicaciones;
- programación;
- publicación directa;
- publisher programado.

Esto evita que un usuario, agente IA o workflow se salte accidentalmente la política.

## Aprobaciones automáticas por política

Cuando una acción `publish` o `schedule` requiere aprobación:

1. el grupo/post pasa a `pending_approval`;
2. Worktic crea una solicitud en `social_approvals`;
3. guarda la intención original (`publish` o `schedule`);
4. tras aprobar, ejecuta esa intención;
5. si se rechaza, vuelve a borrador.

## Auditoría

Nueva tabla:

- `social_governance_audit_v231`

Registra:

- tenant;
- group/campaign;
- acción solicitada;
- decisión (`autopilot`, `approval`, `blocked`, `paused`);
- motivo;
- redes involucradas;
- usuario;
- fecha.

## Base de datos

Nuevas tablas automáticas:

- `social_governance_v231`
- `social_governance_audit_v231`

No requiere SQL manual.

## Variables de entorno

V23.1 **no agrega variables de entorno nuevas**.

## Compatibilidad

- Mantiene V23 Multimedia.
- Mantiene Reutilizar IA.
- Mantiene Aprobaciones manuales.
- Mantiene Community.
- Mantiene carruseles/multi-asset.
- Mantiene el scheduler existente.
- Mantiene permisos/roles existentes.

## Comportamiento recomendado por tipo de operación

El tamaño de la empresa no determina el modo. Algunos ejemplos:

- Empresa grande con marketing 100% automatizado: **Autopilot**.
- Agencia que necesita aprobación del cliente: **Aprobación**.
- Corporación con alto volumen, pero promociones/comunicados controlados: **Híbrido**.
- Pyme con dueño que quiere revisar todo: **Aprobación**.
- Pyme que quiere máxima automatización: **Autopilot**.

## Checklist para despliegue final

1. Abrir Social Hub → Autopilot.
2. Probar Autopilot con una publicación programada.
3. Cambiar a Aprobación y confirmar que `publish` termina en `pending_approval`.
4. Aprobar y comprobar que se publica/programa.
5. Probar Híbrido con una palabra sensible.
6. Configurar una red en Aprobación y otra en Autopilot.
7. Marcar una campaña para aprobación y comprobar la regla.
8. Añadir una palabra bloqueada y confirmar `governance_hold`.
9. Activar Pausa global y comprobar que el scheduler no publica.
10. Reanudar y comprobar que los elementos retenidos por pausa vuelven a cola.
11. Confirmar que el audit log registra las decisiones.

