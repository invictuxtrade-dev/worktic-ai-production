/* Worktic AI · B5 · Social accounts premium UI
 * Presentation only. Does not change OAuth, API calls, permissions, or event handlers.
 * Enhances the existing cards after each Social Hub render.
 */
(() => {
  'use strict';
  const ROOT_ID = 'socialConnectionsGrid';
  const platformLabels = {
    facebook: 'Facebook', instagram: 'Instagram', tiktok: 'TikTok',
    youtube: 'YouTube', linkedin: 'LinkedIn', telegram: 'Telegram'
  };
  const platformMarks = {
    facebook: 'f', instagram: '◎', tiktok: '♪',
    youtube: '▶', linkedin: 'in', telegram: '➤'
  };
  const statusLabels = {
    connected: 'Conectado', active: 'Activo', pending: 'Pendiente',
    draft: 'Pendiente', paused: 'Pausado', error: 'Con error',
    failed: 'Con error', disconnected: 'Desconectado', expired: 'Vencido'
  };

  function element(tag, className, text) {
    const el = document.createElement(tag);
    if (className) el.className = className;
    if (text !== undefined) el.textContent = String(text);
    return el;
  }
  function platformOf(card) {
    const badge = card.querySelector('.connection-head .platform-badge');
    const p = Object.keys(platformLabels).find(name => badge && badge.classList.contains(name));
    return p || 'other';
  }
  function textOf(card, selector) {
    return (card.querySelector(selector)?.textContent || '').trim();
  }
  function metadata(card) {
    const spans = [...(card.querySelector('.connection-meta')?.children || [])];
    return {
      provider: (spans[0]?.querySelector('b')?.textContent || '').trim(),
      scopes: (spans[1]?.querySelector('b')?.textContent || '').trim(),
      lastSync: (spans[2]?.querySelector('b')?.textContent || '').trim()
    };
  }
  function formatProvider(provider) {
    const p = provider.toLowerCase();
    if (['official','api','api_official','oauth','meta','oauth2'].includes(p)) return 'API oficial';
    if (p === 'sandbox') return 'Entorno de pruebas';
    return provider || 'No informado';
  }
  function parsedPermissions(raw) {
    if (!raw || ['pendientes', '—', '-'].includes(raw.toLowerCase())) return [];
    return [...new Set(raw.split(',').map(s => s.trim()).filter(Boolean))];
  }
  function statusStyle(raw) {
    if (raw === 'connected' || raw === 'active') return 'is-connected';
    if (raw === 'error' || raw === 'failed' || raw === 'expired') return 'is-error';
    return 'is-pending';
  }
  function badgeFor(label) {
    return element('span', 'wt-social-b5-perm', label);
  }

  function upgrade(card) {
    if (card.dataset.wtSocialB5 === '1') return;
    const originalHead = card.querySelector('.connection-head');
    const originalMeta = card.querySelector('.connection-meta');
    if (!originalHead || !originalMeta) return; // Unknown layout: do not alter.

    const platform = platformOf(card);
    const name = textOf(card, 'h3') || 'Cuenta social';
    const id = textOf(card, '.connection-id') || 'No informado';
    const oldStatus = originalHead.querySelector('.status-chip');
    const statusKey = String(oldStatus?.textContent || '').trim().toLowerCase();
    const meta = metadata(card);
    const permissions = parsedPermissions(meta.scopes);
    const oldActions = card.querySelector('.actions');
    const oldNotice = card.querySelector('.notice');
    const oldManagedMessage = card.querySelector('small.muted');
    const oldButtons = oldActions ? [...oldActions.querySelectorAll('button')] : [];
    const testButton = oldButtons.find(b => /probar/i.test(b.textContent || ''));
    const idMatch = (testButton?.getAttribute('onclick') || '').match(/testSocialConnection\((\d+)\)/);
    const connectionId = idMatch ? Number(idMatch[1]) : 0;
    const deleteButton = oldButtons.find(b => /eliminar/i.test(b.textContent || ''));

    const cardHead = element('div', 'wt-social-b5-head');
    const brand = element('div', 'wt-social-b5-brand');
    const mark = element('span', 'wt-social-b5-brand-mark wt-social-b5-' + platform, platformMarks[platform] || '◎');
    mark.setAttribute('aria-hidden', 'true');
    const brandText = element('div', 'wt-social-b5-brand-text');
    brandText.append(element('span', 'wt-social-b5-brand-title', platformLabels[platform] || platform), element('span', 'wt-social-b5-brand-sub', 'Conexión de red social'));
    brand.append(mark, brandText);
    const status = element('span', 'wt-social-b5-status ' + statusStyle(statusKey), statusLabels[statusKey] || (statusKey || 'Sin estado'));
    cardHead.append(brand, status);

    const body = element('div', 'wt-social-b5-body');
    body.append(element('h3', 'wt-social-b5-account-title', name));
    const identity = element('div', 'wt-social-b5-identity');
    const identityText = element('div', 'wt-social-b5-identity-text');
    identityText.append(
      element('span', 'wt-social-b5-kicker', platform === 'facebook' ? 'PÁGINA VINCULADA' : 'CUENTA VINCULADA'),
      element('strong', 'wt-social-b5-identity-name', name),
      element('small', 'wt-social-b5-identity-id', 'ID: ' + id)
    );
    identity.append(identityText, element('span', 'wt-social-b5-identity-check', statusKey === 'connected' ? '✓' : '•'));
    body.append(identity);

    const facts = element('div', 'wt-social-b5-facts');
    const pFact = element('div', 'wt-social-b5-fact');
    pFact.append(element('span', 'wt-social-b5-fact-label', 'TIPO DE CONEXIÓN'), element('strong', '', formatProvider(meta.provider)));
    const sFact = element('div', 'wt-social-b5-fact');
    sFact.append(element('span', 'wt-social-b5-fact-label', 'ÚLTIMA SINCRONIZACIÓN'), element('strong', '', meta.lastSync && meta.lastSync !== '—' ? meta.lastSync : 'Sin registro aún'));
    facts.append(pFact, sFact);
    body.append(facts);

    const perms = element('details', 'wt-social-b5-permissions');
    const permSummary = element('summary');
    permSummary.append(element('span', '', 'Permisos registrados'), element('span', 'wt-social-b5-perm-count', String(permissions.length)));
    perms.append(permSummary);
    const permList = element('div', 'wt-social-b5-permission-list');
    if (permissions.length) permissions.forEach(p => permList.append(badgeFor(p)));
    else permList.append(element('span', 'wt-social-b5-permission-unknown', 'El proveedor aún no informó permisos para mostrar.'));
    perms.append(permList);
    body.append(perms);
    if (oldNotice) {
      oldNotice.classList.add('wt-social-b5-notice');
      body.append(oldNotice);
    }
    if (oldManagedMessage) body.append(oldManagedMessage);

    if (platform === 'tiktok') {
      const hint = element('p', 'wt-social-t2-capability', 'Perfil conectado mediante Login Kit. La publicación y los mensajes requieren permisos adicionales de TikTok.');
      body.append(hint);
    }
    const footer = element('div', 'wt-social-b5-footer');
    if (connectionId) {
      const manage = element('button', 'wt-social-t2-manage', 'Administrar');
      manage.type = 'button';
      manage.addEventListener('click', () => window.workticOpenSocialAccountDetails?.(connectionId));
      footer.append(manage);
    }
    if (testButton) {
      testButton.className = 'wt-social-b5-test';
      testButton.textContent = 'Probar conexión';
      footer.append(testButton);
    }
    if (deleteButton) {
      const more = element('details', 'wt-social-b5-more');
      const summary = element('summary', '', '⋯');
      summary.setAttribute('aria-label', 'Más acciones de ' + name);
      summary.title = 'Más acciones';
      const menu = element('div', 'wt-social-b5-more-menu');
      if (connectionId) {
        const reconnect = element('button', 'wt-social-t2-reconnect', 'Reconectar');
        reconnect.type = 'button';
        reconnect.addEventListener('click', () => window.workticReauthorizeSocial?.(platform));
        menu.append(reconnect);
      }
      deleteButton.className = 'wt-social-b5-delete';
      deleteButton.textContent = 'Eliminar conexión';
      menu.append(deleteButton);
      more.append(summary, menu);
      footer.append(more);
    }

    card.replaceChildren(cardHead, body, footer);
    card.classList.add('wt-social-b5-card');
    card.dataset.wtSocialB5 = '1';
  }
  function upgradeGrid(grid) {
    grid.querySelectorAll(':scope > article.connection-card').forEach(upgrade);
  }
  function mount() {
    const grid = document.getElementById(ROOT_ID);
    if (!grid || grid.dataset.wtSocialB5Observer === '1') return;
    grid.dataset.wtSocialB5Observer = '1';
    upgradeGrid(grid);
    new MutationObserver(() => upgradeGrid(grid)).observe(grid, {childList: true});
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mount, {once:true});
  else mount();
})();
