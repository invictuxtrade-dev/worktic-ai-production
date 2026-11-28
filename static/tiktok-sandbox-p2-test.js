/* Worktic AI TikTok P2 Sandbox R1.3.1 — visibility and safe refresh UX.
   Pure UI improvement: OAuth/creator/submit API contracts are unchanged.
   Explicit manual actions only. Never auto-submits content. */
(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const api = '/api/social/tiktok/sandbox';
  let ready = false;
  let canSubmit = false;
  let hasCreator = false;
  let privateAllowed = false;
  let requestId = '';
  let alreadySubmitted = false;
  let refreshing = false;
  let refreshCount = 0;
  let creatorLoading = false;
  let activeAccount = '';

  function output(value) {
    $('result').textContent = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
  }

  function errorText(err) {
    return err instanceof Error ? err.message : String(err);
  }

  async function call(action, method = 'GET', data) {
    const options = {
      method,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { Accept: 'application/json' }
    };
    if (method === 'POST') {
      options.headers['Content-Type'] = 'application/json';
      options.headers['X-Worktic-TikTok-Sandbox'] = action;
      options.body = JSON.stringify(data || {});
    }
    const response = await fetch(api + '?action=' + encodeURIComponent(action), options);
    const body = await response.json().catch(() => ({ error: 'El servidor no devolvió JSON' }));
    if (!response.ok) throw new Error(body.error || 'Error HTTP ' + response.status);
    return body;
  }

  function showFeedback(message, kind = 'info') {
    const feedback = $('auth-feedback');
    feedback.textContent = message;
    feedback.className = 'auth-feedback type-' + kind;
  }

  function paintConnection(kind, title, description) {
    $('connection-badge').className = 'connection-badge state-' + kind;
    $('connection-badge').textContent = kind === 'ok' ? 'Conectado' :
      kind === 'loading' ? 'Consultando' : kind === 'error' ? 'Sin verificar' : 'Sin conectar';
    $('connection-title').textContent = title;
    $('connection-description').textContent = description;
    $('connection-panel').className = 'connection-panel state-' + kind;
  }

  function updateButton() {
    const mode = $('mode').value;
    $('submit').disabled = !(
      ready && canSubmit && !alreadySubmitted && $('consent').checked &&
      (mode === 'inbox' || (hasCreator && privateAllowed))
    );
    $('submit').textContent = mode === 'inbox'
      ? 'Enviar a bandeja TikTok'
      : 'Publicar prueba privada (Solo yo)';
    $('creator').disabled = !ready || creatorLoading;
    $('connect').textContent = ready ? 'Reconectar @workticai' : 'Autorizar @workticai';
  }

  async function refresh(origin = 'manual') {
    if (refreshing) return;
    refreshing = true;
    $('refresh').disabled = true;
    $('refresh').textContent = 'Consultando TikTok…';
    $('last-refresh').textContent = 'Comprobando la autorización en el servidor…';
    if (!ready) {
      paintConnection('loading', 'Verificando autorización', 'Consultando el estado del Sandbox.');
    }

    try {
      const data = await call('readiness');
      if (data.ok !== true || data.environment !== 'sandbox' || typeof data.connected !== 'boolean') {
        throw new Error('Respuesta de estado inesperada del servidor Sandbox');
      }

      const nextAccount = data.connected ? String(data.account_name || 'Cuenta autorizada') : '';
      if (!data.connected || activeAccount !== nextAccount) {
        hasCreator = false;
        privateAllowed = false;
        $('privacy').textContent = 'Consulta creator_info para verificar las opciones de privacidad.';
      }
      activeAccount = nextAccount;
      ready = data.connected;
      canSubmit = Boolean(data.can_submit);
      refreshCount += 1;

      $('account-name').textContent = ready ? nextAccount : 'No autorizada';
      $('scopes').textContent = ready ? String(data.granted_scopes || 'No informados') : '—';
      $('sending-status').textContent = canSubmit ? 'Habilitados para pruebas manuales' : 'Desactivados (modo seguro)';
      $('status').textContent = ready
        ? 'Sandbox activo · Cuenta conectada · Envios ' + (canSubmit ? 'habilitados' : 'desactivados')
        : 'Sandbox activo · Cuenta pendiente de autorización';

      if (ready) {
        paintConnection('ok', 'TikTok conectado correctamente',
          'La autorización OAuth está guardada en el Sandbox independiente de Worktic AI.');
      } else {
        paintConnection('idle', 'TikTok pendiente de autorización',
          'Utiliza «Autorizar @workticai» para conectar esta cuenta al Sandbox.');
      }
      const checkedAt = new Date().toLocaleTimeString('es-CO', { hour12: true });
      $('last-refresh').textContent = 'Última comprobación: ' + checkedAt + ' · Verificación n.º ' + refreshCount;
      if (origin === 'oauth-success') {
        showFeedback(ready
          ? 'Autorización completada y comprobada: ' + nextAccount + ' está conectada.'
          : 'TikTok devolvió la autorización, pero el servidor aún no confirma una conexión. Actualiza el estado.',
          ready ? 'success' : 'warning');
      } else if (origin === 'oauth-error') {
        // Keep the callback error visible even if another older authorization exists.
      } else {
        showFeedback(ready
          ? 'Estado actualizado: ' + nextAccount + ' continúa conectada. Esta consulta no envía videos.'
          : 'Estado actualizado: todavía no hay una cuenta autorizada. Esta consulta no envía videos.',
          ready ? 'success' : 'info');
      }
    } catch (err) {
      ready = false;
      canSubmit = false;
      hasCreator = false;
      privateAllowed = false;
      activeAccount = '';
      $('account-name').textContent = 'No verificable';
      $('scopes').textContent = '—';
      $('sending-status').textContent = 'Sin confirmar';
      $('status').textContent = 'El estado del Sandbox no pudo comprobarse.';
      $('last-refresh').textContent = 'Última consulta: fallida';
      $('privacy').textContent = 'Consulta creator_info después de restablecer la conexión.';
      paintConnection('error', 'No se pudo comprobar TikTok', errorText(err));
      showFeedback('Error al actualizar estado: ' + errorText(err) + '. Revisa sesión y conexión.', 'error');
    } finally {
      refreshing = false;
      $('refresh').disabled = false;
      $('refresh').textContent = 'Actualizar estado';
      updateButton();
    }
  }

  async function authorize() {
    $('connect').disabled = true;
    try {
      const data = await call('authorize', 'POST', {});
      if (!data.authorization_url) throw new Error('Falta la URL OAuth');
      window.location.assign(data.authorization_url);
    } catch (err) {
      showFeedback('No se pudo iniciar la autorización: ' + errorText(err), 'error');
      $('connect').disabled = false;
    }
  }

  async function creator() {
    if (!ready || creatorLoading) return;
    creatorLoading = true;
    $('creator').disabled = true;
    $('creator').textContent = 'Consultando…';
    $('creator-feedback').textContent = 'Consultando opciones de privacidad de TikTok…';
    try {
      const data = await call('creator');
      hasCreator = true;
      privateAllowed = Boolean(data.private_allowed);
      const options = data.creator?.privacy_level_options || [];
      const name = String(data.creator?.creator_username || 'desconocida');
      $('privacy').textContent = 'Cuenta: @' + name + ' · Privacidades: ' +
        options.join(', ') + ' · Solo yo: ' + (privateAllowed ? 'permitido' : 'no permitido');
      $('creator-feedback').textContent = privateAllowed
        ? 'Consulta completada. SELF_ONLY disponible. No se ha enviado contenido.'
        : 'Consulta completada. SELF_ONLY no disponible: Direct Post permanece bloqueado.';
      output('creator_info cargado correctamente. No se ha enviado contenido.');
    } catch (err) {
      hasCreator = false;
      privateAllowed = false;
      $('privacy').textContent = 'No fue posible consultar las opciones de privacidad.';
      $('creator-feedback').textContent = 'No se pudo consultar creator_info: ' + errorText(err);
      output(errorText(err));
    } finally {
      creatorLoading = false;
      $('creator').textContent = 'Consultar creator_info';
      updateButton();
    }
  }

  function newRequest() {
    alreadySubmitted = false;
    requestId = crypto.randomUUID();
    $('request').textContent = requestId;
    output('Nueva solicitud creada. No se ha enviado contenido.');
    updateButton();
  }

  async function submit() {
    if ($('submit').disabled) return;
    if (!confirm('Se enviará el MP4 exclusivamente a TU cuenta TikTok del Sandbox. En Direct Post será privado (Solo yo). ¿Confirmas?')) return;
    const payload = {
      video_url: $('video_url').value.trim(), title: $('title').value.trim(),
      privacy_level: 'SELF_ONLY', mode: $('mode').value, request_id: requestId,
      ai_generated: $('ai').checked, brand_organic: $('brand').checked,
      confirmed: $('consent').checked
    };
    alreadySubmitted = true;
    $('submit').disabled = true;
    try {
      const result = await call('submit', 'POST', payload);
      output(result);
    } catch (err) {
      output('Error o respuesta no confirmada: ' + errorText(err) + '\nNO vuelvas a enviar sin comprobar el estado.');
    }
    // Never retry the same request automatically.
    updateButton();
  }

  async function checkRequest() {
    if (!alreadySubmitted) {
      output('Esta solicitud aún no se ha enviado. No hay un estado remoto que consultar.');
      return;
    }
    try {
      const r = await fetch(api + '?action=status&request_id=' + encodeURIComponent(requestId),
        { credentials: 'same-origin', cache: 'no-store', headers: { Accept: 'application/json' } });
      const data = await r.json().catch(() => ({ error: 'El servidor no devolvió JSON' }));
      if (!r.ok) throw Error(data.error || 'HTTP ' + r.status);
      output(data);
    } catch (err) {
      output('No se pudo confirmar el estado: ' + errorText(err) + '\nNo repitas el envío.');
    }
  }

  $('refresh').onclick = () => refresh('manual');
  $('connect').onclick = authorize;
  $('creator').onclick = creator;
  $('mode').onchange = updateButton;
  $('consent').onchange = updateButton;
  $('submit').onclick = submit;
  $('check').onclick = checkRequest;
  $('reset').onclick = () => {
    if (confirm('¿Crear un nuevo identificador de solicitud? No reenvíes videos con estado desconocido.')) newRequest();
  };
  $('video_url').onchange = () => { $('preview').src = $('video_url').value.trim(); };
  $('preview').src = $('video_url').value.trim();

  // Do not overwrite the result of OAuth with "Nueva solicitud creada".
  // OAuth belongs in its dedicated feedback area, not the submission log.
  newRequest();
  const params = new URLSearchParams(location.search);
  const oauth = params.get('oauth');
  if (oauth) {
    const u = new URL(location.href);
    u.searchParams.delete('oauth');
    history.replaceState(null, '', u.pathname + u.search + u.hash);
    if (oauth === 'success') {
      showFeedback('TikTok devolvió la autorización. Verificando que el servidor guardó la conexión…', 'info');
      refresh('oauth-success');
    } else {
      const messages = {
        denied: 'Autorización cancelada en TikTok.',
        failed: 'TikTok no completó la autorización o no concedió todos los permisos.',
        profile_failed: 'No fue posible obtener el perfil después de autorizar TikTok.'
      };
      showFeedback(messages[oauth] || 'No se completó la autorización de TikTok.', 'error');
      refresh('oauth-error');
    }
  } else {
    refresh('initial');
  }
})();
