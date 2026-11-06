/* Worktic AI B8 — Live Social publication status (read-only polling).
 * Does not create posts or call /api/social/publish. Does not change OAuth.
 */
(function () {
  'use strict';
  const PENDING = new Set(['queued', 'publishing', 'retrying', 'processing', 'sending', 'pending', 'in_progress']);
  const TERMINAL = new Set(['published', 'failed', 'error', 'cancelled', 'canceled']);
  const INTERVAL_MS = 3000;
  const MAX_MONITOR_MS = 180000;
  const MAX_ERRORS = 3;
  const normalize = v => String(v == null ? '' : v).trim().toLowerCase();
  const pendingPosts = posts => (Array.isArray(posts) ? posts : []).filter(p => PENDING.has(normalize(p.status)));
  const transitions = (before, after) => {
    const prev = new Map((before || []).map(p => [String(p.id), normalize(p.status)]));
    return (after || []).filter(p => prev.has(String(p.id)) && prev.get(String(p.id)) !== normalize(p.status))
      .map(p => ({id: p.id, from: prev.get(String(p.id)), to: normalize(p.status), platform: p.platform, error: p.error_message || ''}));
  };
  const safeMessage = input => String(input || '')
    .replace(/access[_-]?token\s*[=:]\s*[^\s&"',}]+/gi, 'access_token=[oculto]')
    .replace(/EA[A-Z0-9]{12,}/g, '[token oculto]')
    .slice(0, 160);

  // Export pure helpers for isolated tests, never bootstrap browser on Node.
  if (typeof module === 'object' && module && module.exports) {
    module.exports = {normalize, pendingPosts, transitions, safeMessage, PENDING, TERMINAL};
    return;
  }
  if (typeof document === 'undefined' || typeof window === 'undefined') return;
  if (window.__workticSocialLiveB8Loaded) return;
  const root = document.getElementById('socialhub');
  const library = document.getElementById('social-library');
  const list = document.getElementById('socialPostsTable');
  if (!root || !library || !list || typeof window.renderSocialPosts !== 'function') return;
  window.__workticSocialLiveB8Loaded = true;

  let timer = null, controller = null, running = false, started = 0, pendingKey = '';
  let manualRevision = 0, failures = 0, suspended = false, message = '', messageType = '', messageUntil = 0;
  let lockedGroups = new Set(), lockedPosts = new Set();
  const currentPosts = () => (Array.isArray(window.socialPosts) ? window.socialPosts : []);
  const isVisible = () => root.classList.contains('active') && !document.hidden;
  const isLibrary = () => library.classList.contains('active');
  const statusName = status => ({queued:'En cola',publishing:'Publicando…',processing:'Procesando…',sending:'Enviando…',retrying:'Reintentando…',pending:'Pendiente',in_progress:'En proceso',published:'Publicado',failed:'Con error',error:'Con error'}[normalize(status)] || status);

  const bar = document.createElement('div');
  bar.id = 'socialLiveB8Status';
  bar.className = 'wt-social-live-b8';
  bar.setAttribute('role', 'status');
  bar.setAttribute('aria-live', 'polite');
  bar.hidden = true;
  const header = library.querySelector('.section-head');
  if (header) header.insertAdjacentElement('afterend', bar);
  else library.insertBefore(bar, library.firstChild);

  function display(text, type) {
    bar.hidden = !text || !isVisible() || !isLibrary();
    if (bar.hidden) return;
    bar.className = 'wt-social-live-b8 is-' + (type || 'pending');
    bar.replaceChildren();
    const dot = document.createElement('span');
    dot.className = 'wt-social-live-dot';
    dot.setAttribute('aria-hidden','true');
    const label = document.createElement('span');
    label.textContent = text;
    bar.append(dot, label);
  }

  function decorateCards() {
    if (!list) return;
    for (const card of list.querySelectorAll('.content-library-card')) {
      const status = card.querySelector('.status-chip');
      if (!status) continue;
      const key = normalize(status.getAttribute('data-wt-b8-raw') || status.textContent);
      if (!PENDING.has(key)) continue;
      status.setAttribute('data-wt-b8-raw', key);
      status.textContent = statusName(key);
      status.classList.add('wt-social-b8-processing');
      const actions = card.querySelectorAll('button');
      for (const button of actions) {
        if (!/^publicar$/i.test((button.textContent || '').trim())) continue;
        button.disabled = true;
        button.title = 'Ya hay un envío en curso. No es necesario publicarlo otra vez.';
        button.setAttribute('aria-disabled', 'true');
      }
    }
  }

  function stop(abort) {
    if (timer !== null) { clearTimeout(timer); timer = null; }
    if (abort && controller) controller.abort();
  }

  function statusText(pending) {
    const retrying = pending.filter(p => normalize(p.status) === 'retrying').length;
    if (retrying) return `${retrying} publicación${retrying === 1 ? '' : 'es'} en reintento. Verificando el resultado cada 3 segundos; no pulses Publicar de nuevo.`;
    return `${pending.length} publicación${pending.length === 1 ? '' : 'es'} enviándose. Estado actualizado automáticamente cada 3 segundos.`;
  }

  function sync() {
    decorateCards();
    const pending = pendingPosts(currentPosts());
    if (!isVisible()) {
      stop(true);
      started = 0; pendingKey = ''; failures = 0; suspended = false;
      display('', 'pending');
      return;
    }
    if (!pending.length) {
      stop(false);
      started = 0; pendingKey = ''; failures = 0; suspended = false;
      display(Date.now() < messageUntil ? message : '', messageType);
      return;
    }
    const key = pending.map(p => p.id).sort().join(',');
    if (key !== pendingKey) {
      pendingKey = key;
      started = Date.now(); failures = 0; suspended = false;
    }
    if (suspended) { display(message, messageType); return; }
    if (Date.now() - started >= MAX_MONITOR_MS) {
      suspended = true;
      message = 'La publicación continúa pendiente. Dejamos de consultar automáticamente por ahora; vuelve a Contenido para verificarla más tarde.';
      messageType = 'info';
      display(message, messageType);
      stop(false);
      return;
    }
    display(statusText(pending), 'pending');
    if (!running && timer === null) timer = setTimeout(poll, INTERVAL_MS);
  }

  function noticeFor(changes) {
    const final = changes.filter(c => TERMINAL.has(c.to));
    if (!final.length) return;
    const good = final.filter(c => c.to === 'published').length;
    const bad = final.filter(c => c.to === 'failed' || c.to === 'error').length;
    if (bad) {
      const err = final.find(c => c.to === 'failed' || c.to === 'error');
      message = `Publicación con error${err?.platform ? ' en ' + err.platform : ''}: ${safeMessage(err?.error) || 'abre Editar para consultar el motivo.'}`;
      messageType = 'error';
    } else if (good) {
      message = `${good} publicación${good === 1 ? '' : 'es'} publicada${good === 1 ? '' : 's'} correctamente.`;
      messageType = 'success';
    } else {
      message = 'El estado de la publicación se ha actualizado.';
      messageType = 'info';
    }
    messageUntil = Date.now() + 12000;
  }

  async function requestJSON(path, signal) {
    const response = await fetch(path, {
      method: 'GET', credentials: 'same-origin', cache: 'no-store',
      headers: {'Accept':'application/json'}, signal
    });
    if (!response.ok) throw new Error(response.status === 401 ? 'Tu sesión ha caducado' : `Error HTTP ${response.status}`);
    return response.json();
  }

  async function poll() {
    timer = null;
    if (!isVisible() || running || suspended || !pendingPosts(currentPosts()).length) { sync(); return; }
    running = true;
    const rev = manualRevision;
    controller = new AbortController();
    try {
      const before = currentPosts().slice();
      const updated = await requestJSON('/api/social/posts', controller.signal);
      if (!Array.isArray(updated)) throw new Error('Respuesta de publicaciones no válida');
      if (!isVisible() || rev !== manualRevision) return;
      const changed = transitions(before, updated);
      const additions = updated.length !== before.length;
      const currentIDs = new Set(updated.map(p => String(p.id)));
      const missing = before.some(p => !currentIDs.has(String(p.id)));
      if (changed.length || additions || missing) {
        window.socialPosts = updated;
        noticeFor(changed);
        if (typeof window.renderSocialPosts === 'function') window.renderSocialPosts();
        if (typeof window.renderSocialCalendar === 'function') window.renderSocialCalendar();
        try {
          const overview = await requestJSON('/api/social/overview', controller.signal);
          if (isVisible() && rev === manualRevision) {
            window.socialOverview = overview;
            if (typeof window.renderSocialStats === 'function') window.renderSocialStats();
            if (typeof window.renderSocialOverview === 'function') window.renderSocialOverview();
          }
        } catch (e) { if (e.name !== 'AbortError') console.warn('B8: estadísticas pendientes', e.message); }
      }
      failures = 0;
    } catch (e) {
      if (e.name !== 'AbortError') {
        failures++;
        if (failures >= MAX_ERRORS) {
          suspended = true;
          message = 'No pudimos consultar el estado de publicación. Comprueba tu conexión y vuelve a entrar a Redes Sociales.';
          messageType = 'error';
          console.warn('B8: consultas de estado suspendidas:', e.message);
        }
      }
    } finally {
      controller = null;
      running = false;
      sync();
    }
  }

  // The existing render and load functions remain the source of truth.
  const originalRender = window.renderSocialPosts;
  window.renderSocialPosts = function(...args) {
    const result = originalRender.apply(this, args);
    sync();
    return result;
  };
  const originalLoad = window.loadSocialHub;
  if (typeof originalLoad === 'function') {
    window.loadSocialHub = async function(...args) {
      ++manualRevision;
      const result = await originalLoad.apply(this, args);
      sync();
      return result;
    };
  }

  function preventDuplicate(original, keyFor, pendingFor, lockSet) {
    if (typeof original !== 'function') return original;
    return function(...args) {
      const key = keyFor(args[0]);
      if (lockSet.has(key) || pendingFor(args[0])) {
        if (typeof window.toast === 'function') window.toast('Esta publicación ya se está procesando. Su estado se actualizará automáticamente.');
        return Promise.resolve();
      }
      lockSet.add(key);
      try {
        return Promise.resolve(original.apply(this,args)).finally(() => {
          setTimeout(() => lockSet.delete(key), 2000);
          setTimeout(sync, 800);
        });
      } catch (e) { lockSet.delete(key); throw e; }
    };
  }
  window.publishSocialGroup = preventDuplicate(window.publishSocialGroup,
    id => String(id), id => currentPosts().some(p => Number(p.group_id) === Number(id) && PENDING.has(normalize(p.status))), lockedGroups);
  window.publishSocialPost = preventDuplicate(window.publishSocialPost,
    id => String(id), id => currentPosts().some(p => Number(p.id) === Number(id) && PENDING.has(normalize(p.status))), lockedPosts);

  const observer = new MutationObserver(sync);
  observer.observe(root, {attributes: true, attributeFilter: ['class']});
  document.addEventListener('visibilitychange', sync);
  window.addEventListener('pageshow', sync);
  document.addEventListener('click', function(e) {
    const target = e.target.closest?.('[data-social-tab], #spSave');
    if (!target) return;
    if (target.id === 'spSave' && document.getElementById('spAction')?.value === 'publish') {
      message = 'Enviando contenido a la red social…';
      messageType = 'pending';
      messageUntil = Date.now() + 8000;
    }
    setTimeout(sync, 250);
  }, true);
  sync();
})();
