/* Worktic AI V27.35.92 - UI only: navigacion y accion de Landing Pages.
 * No crea elementos de menu, no modifica permisos, rutas ni listeners del boton.
 */
(function () {
  'use strict';
  if (window.__workticNavLanding273592) return;
  window.__workticNavLanding273592 = true;

  const ORDER = [
    'Resumen', 'Agentes IA', 'Atracción de Clientes', 'Automatizaciones',
    'Canales Automatizados', 'Redes Sociales', 'WhatsApp Business',
    'WhatsApp Marketing', 'Agenda IA', 'Catálogo de ventas',
    'Oportunidades', 'Conversaciones', 'Contactos',
    'Grupos & Comunidades', 'Worktic Studio', 'Ads Center',
    'Analytics', 'Mis Créditos', 'Plan y membresía',
    'Partner Center', 'Mi perfil', 'Equipo', 'Administración Worktic', 'Guías'
  ];
  const alias = new Map();
  function norm(s) {
    return String(s || '').normalize('NFD').replace(/[\u0300-\u036f]/g, '')
      .replace(/\s+/g, ' ').trim().toLocaleLowerCase('es');
  }
  ORDER.forEach(label => alias.set(norm(label), label));
  [
    ['Social Hub', 'Redes Sociales'],
    ['Billetera de Créditos', 'Mis Créditos'],
    ['Billetera de Creditos', 'Mis Créditos'],
    ['Mis Creditos', 'Mis Créditos'],
    ['Plan y Membresía', 'Plan y membresía'],
    ['Administracion Worktic', 'Administración Worktic'],
    ['Guia', 'Guías'], ['Guias', 'Guías'],
    ['Grupos y Comunidades', 'Grupos & Comunidades']
  ].forEach(([oldName, desired]) => alias.set(norm(oldName), desired));
  const menuSelector = 'a,button,[role="menuitem"],[data-view],[data-page],.nav-item,.menu-item,.sidebar-item';
  const rootsSelector = 'aside,nav,[id*="sidebar" i],[class*="sidebar" i]';
  const desiredRank = new Map(ORDER.map((name, index) => [name, index]));

  function itemName(node) {
    if (!node || !node.textContent) return '';
    return alias.get(norm(node.textContent)) || '';
  }
  function topMenuNode(node) {
    const outer = node.closest('.sidebar-item,.nav-item,.menu-item,li');
    return outer && itemName(outer) === itemName(node) ? outer : node;
  }
  function findItems(root) {
    const found = new Map();
    if (!root) return found;
    root.querySelectorAll(menuSelector).forEach(node => {
      const key = itemName(node);
      if (!key) return;
      const chosen = topMenuNode(node);
      if (!found.has(key) || chosen.querySelectorAll('*').length < found.get(key).querySelectorAll('*').length) {
        found.set(key, chosen);
      }
    });
    return found;
  }
  function findSidebar() {
    let winner = null, best = 0;
    document.querySelectorAll(rootsSelector).forEach(root => {
      const matches = findItems(root).size;
      const width = root.getBoundingClientRect ? root.getBoundingClientRect().width : 0;
      if (matches > best && (width < 540 || !width)) { winner = root; best = matches; }
    });
    return best >= 4 ? winner : null;
  }
  function relabel(node, name) {
    if (!node || node.textContent.trim() === name) return;
    const old = norm(node.textContent);
    if (!alias.has(old)) return;
    const iter = document.createTreeWalker(node, NodeFilter.SHOW_TEXT);
    const texts = [];
    while (iter.nextNode()) texts.push(iter.currentNode);
    const matching = texts.filter(textNode => alias.get(norm(textNode.nodeValue)) === name);
    if (matching.length === 1) matching[0].nodeValue = name;
  }
  function reorderMenu() {
    const root = findSidebar();
    if (!root) return;
    const found = findItems(root);
    // Cambiar exclusivamente texto visible, no data-view ni controles de acceso.
    ['Mis Créditos', 'Redes Sociales', 'Guías'].forEach(name => relabel(found.get(name), name));

    const groups = new Map();
    found.forEach((node, name) => {
      if (!node.parentElement) return;
      const parent = node.parentElement;
      if (!groups.has(parent)) groups.set(parent, []);
      groups.get(parent).push({ node, name });
    });
    let best = [];
    groups.forEach(group => { if (group.length > best.length) best = group; });
    if (best.length < 4) return; // No reorganizar si la estructura es inesperada.

    const parent = best[0].node.parentNode;
    const nodes = new Set(best.map(entry => entry.node));
    const actual = Array.from(parent.children).filter(node => nodes.has(node));
    const sorted = best.slice().sort((a, b) => desiredRank.get(a.name) - desiredRank.get(b.name));
    if (actual.length !== sorted.length || actual.every((node, index) => node === sorted[index].node)) return;
    const marker = document.createComment('wt-v273592-order-position');
    parent.insertBefore(marker, actual[0]);
    sorted.forEach(entry => parent.insertBefore(entry.node, marker));
    marker.remove();
  }

  const tabNames = new Set(['campanas','calendario','creativos','formularios',
    'landing pages','leads y atribucion','meta lead ads','meta ads']);
  function findTabs() {
    const all = Array.from(document.querySelectorAll('button,[role="tab"],a'));
    const potential = all.filter(node => tabNames.has(norm(node.textContent)));
    const parents = new Map();
    potential.forEach(node => {
      const parent = node.parentElement;
      if (!parent) return;
      if (!parents.has(parent)) parents.set(parent, new Set());
      parents.get(parent).add(norm(node.textContent));
    });
    let winner = null, count = 0;
    parents.forEach((names, parent) => {
      if (names.has('landing pages') && names.size > count) {
        winner = parent; count = names.size;
      }
    });
    return count >= 4 ? winner : null;
  }
  function updateLandingActionVisibility(tabs, row) {
    if (!tabs || !row) return;
    const siblings = Array.from(tabs.querySelectorAll('button,[role="tab"],a'));
    const active = node => node.matches('.active,.is-active,.selected,[aria-selected="true"],[data-active="true"],[aria-current="page"]') ||
      node.parentElement && node.parentElement.matches('.active,.is-active,.selected');
    const hasActive = siblings.some(active);
    const landingActive = siblings.some(node => norm(node.textContent) === 'landing pages' && active(node));
    row.style.display = hasActive && !landingActive ? 'none' : 'flex';
  }
  function moveLandingAction() {
    // Evitar ocultar acciones nuevas que otros modulos agreguen al encabezado.
    document.querySelectorAll('[data-wt-v273592-emptied]').forEach(parent => {
      if (parent.children.length || parent.textContent.trim()) {
        parent.style.display = parent.getAttribute('data-wt-v273592-prev-display') || '';
        parent.removeAttribute('data-wt-v273592-emptied');
        parent.removeAttribute('data-wt-v273592-prev-display');
      }
    });
    const tabs = findTabs();
    if (!tabs || !tabs.parentElement) return;
    const buttons = Array.from(document.querySelectorAll('button'));
    const action = buttons.find(node => norm(node.textContent) === 'nueva landing');
    if (!action || tabs.contains(action)) return;

    let row = document.getElementById('wt-v273592-landing-action-row');
    if (!row) {
      row = document.createElement('div');
      row.id = 'wt-v273592-landing-action-row';
      row.setAttribute('data-worktic-version', '27.35.92');
      row.style.cssText = 'display:flex;justify-content:flex-end;align-items:center;gap:12px;margin:14px 0 16px;flex-wrap:wrap';
      tabs.insertAdjacentElement('afterend', row);
    } else if (row.previousElementSibling !== tabs) {
      tabs.insertAdjacentElement('afterend', row);
    }
    if (action.parentElement === row) {
      updateLandingActionVisibility(tabs, row);
      return;
    }
    const oldParent = action.parentElement;
    row.appendChild(action); // conserva el DOM node y sus event listeners directos
    if (oldParent && oldParent !== document.body && oldParent !== document.querySelector('main') &&
        oldParent.children.length === 0 && !oldParent.textContent.trim()) {
      oldParent.setAttribute('data-wt-v273592-emptied', 'true');
      oldParent.setAttribute('data-wt-v273592-prev-display', oldParent.style.display || '');
      oldParent.style.display = 'none';
    }
    updateLandingActionVisibility(tabs, row);
  }
  function ensureStyles() {
    if (document.getElementById('wt-v273592-css')) return;
    const style = document.createElement('style');
    style.id = 'wt-v273592-css';
    style.textContent = '@media(max-width:640px){#wt-v273592-landing-action-row{justify-content:stretch!important}#wt-v273592-landing-action-row>button{width:100%!important}}';
    document.head.appendChild(style);
  }
  let scheduled = false;
  function schedule() {
    if (scheduled) return;
    scheduled = true;
    window.setTimeout(() => {
      scheduled = false;
      try { ensureStyles(); reorderMenu(); moveLandingAction(); }
      catch (err) { console.warn('[Worktic V27.35.92] No se pudo aplicar un ajuste UI:', err); }
    }, 70);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', schedule, { once: true });
  else schedule();
  const watcher = new MutationObserver(schedule);
  watcher.observe(document.documentElement, { subtree: true, childList: true, attributes: true, attributeFilter: ['class', 'aria-selected', 'aria-current', 'data-active'] });
  window.addEventListener('popstate', schedule);
  window.addEventListener('hashchange', schedule);
})();
