/* WorkticAI V27.35.13 — Navegacion organizada. Solo UI: mueve botones originales, no los clona. */
(function () {
  'use strict';
  const sidebar = document.getElementById('appSidebar');
  const nav = sidebar && sidebar.querySelector('nav');
  if (!nav || nav.dataset.wtOrganized === '1') return;
  const buttons = Array.from(nav.querySelectorAll('button[data-view]'));
  const byView = new Map(buttons.map(b => [b.dataset.view, b]));
  const originalViews = buttons.map(b => b.dataset.view);
  const icons = {
    primary: '<rect x="3" y="3" width="8" height="8" rx="2"/><rect x="13" y="3" width="8" height="8" rx="2"/><rect x="3" y="13" width="8" height="8" rx="2"/><rect x="13" y="13" width="8" height="8" rx="2"/>',
    crm: '<circle cx="8" cy="8" r="3"/><path d="M2.5 20v-2a6 6 0 0 1 11 0v2M17 5h5M17 9h5M16 14h6M16 18h6"/>',
    channels: '<circle cx="5" cy="12" r="2.5"/><circle cx="19" cy="5" r="2.5"/><circle cx="19" cy="19" r="2.5"/><path d="m7.4 11 9-5M7.4 13l9 5"/>',
    marketing: '<path d="M3 10v4h4l10 5V5L7 10H3zM7 14l2 6h3l-2-5M20 9c1 2 1 4 0 6"/>',
    metrics: '<rect x="3" y="12" width="4" height="9" rx="1"/><rect x="10" y="7" width="4" height="14" rx="1"/><rect x="17" y="3" width="4" height="18" rx="1"/>',
    company: '<path d="M3 21V7l9-4 9 4v14H3zM9 21v-7h6v7M7 9h2M15 9h2M7 12h2M15 12h2"/>'
  };
  const groups = [
    {id:'crm',es:'CRM y Ventas',en:'CRM & Sales',icon:'crm',views:['contacts','pipeline','appointments','products']},
    {id:'channels',es:'Canales y Automatización',en:'Channels & Automation',icon:'channels',views:['channels','whatsappbusiness','rules','webchat','voice-ai']},
    {id:'marketing',es:'Marketing y Crecimiento',en:'Marketing & Growth',icon:'marketing',views:['socialhub','whatsappmarketing','adscenter','marketing','landings','groups']},
    {id:'finance',es:'Analítica y Finanzas',en:'Analytics & Finance',icon:'metrics',views:['analytics','studio-credits','billing']},
    {id:'company',es:'Empresa y Recursos',en:'Company & Resources',icon:'company',views:['users','partnerscenter','profile','setup']}
  ];
  const lang = () => (typeof window.wtLang === 'function' ? window.wtLang() : document.documentElement.lang) === 'en' ? 'en' : 'es';
  const esc = str => String(str).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const svg = (shape, cls='') => `<svg class="wt-menu-icon ${cls}" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">${shape}</svg>`;
  const root = document.createDocumentFragment();
  const lead = document.createElement('div');
  lead.className = 'wt-menu-lead';
  const leadTitle = document.createElement('p'); leadTitle.className = 'wt-menu-heading wt-menu-main-label';
  lead.appendChild(leadTitle);
  const mainViews = ['dashboard','inbox','agents','studio'];
  mainViews.forEach(id => { const button = byView.get(id); if (button) { button.classList.add('wt-menu-main-link'); lead.appendChild(button); } });
  root.appendChild(lead);
  const groupLabel = document.createElement('p');
  groupLabel.className = 'wt-menu-heading wt-menu-tools-label';
  root.appendChild(groupLabel);
  const allGroups = new Map();
  groups.forEach(g => {
    const details = document.createElement('details');
    details.className = 'wt-menu-group'; details.dataset.group = g.id;
    const summary = document.createElement('summary');
    summary.className = 'wt-menu-summary';
    summary.innerHTML = `${svg(icons[g.icon])}<span class="wt-menu-summary-text"></span><svg class="wt-menu-chevron" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="2"><path d="m7 10 5 5 5-5"/></svg>`;
    const content = document.createElement('div'); content.className = 'wt-menu-group-items';
    g.views.forEach(id => {const button = byView.get(id); if (button) {button.classList.add('wt-menu-sub-link'); content.appendChild(button);}});
    details.appendChild(summary); details.appendChild(content);
    allGroups.set(g.id, details);
    root.appendChild(details);
    details.addEventListener('toggle', () => {
      if (details.open) allGroups.forEach((other, id) => {if (other !== details && other.open) other.open = false;});
      if (details.open) try {sessionStorage.setItem('wt-menu-open-group', g.id);} catch (_) {}
      updateGroupStates();
    });
  });
  const adminWrap = document.createElement('div'); adminWrap.className = 'wt-menu-admin';
  const adminBtn = byView.get('admin'); if(adminBtn) adminWrap.appendChild(adminBtn);
  root.appendChild(adminWrap);
  // Botones originales que no deben perderse al incorporar vistas nuevas.
  const used = new Set(mainViews.concat(...groups.map(g => g.views), ['admin']));
  const extras = originalViews.filter(id => !used.has(id));
  if(extras.length) {
    const other = allGroups.get('company')?.querySelector('.wt-menu-group-items');
    extras.forEach(id => {if(other) other.appendChild(byView.get(id));});
  }
  nav.appendChild(root);
  nav.dataset.wtOrganized = '1';
  // Conserva data-view, listeners y reglas de permisos originales, sin duplicarlos.
  const labels = {
    channels:{es:'Canales conectados',en:'Connected channels'},
    whatsappbusiness:{es:'WhatsApp Business',en:'WhatsApp Business'},
    landings:{es:'Landings y Funnels',en:'Landings & Funnels'},
    groups:{es:'Grupos y Comunidades',en:'Groups & Communities'},
    setup:{es:'Guías y ayuda',en:'Guides & Help'},
    'studio-credits':{es:'Billetera de Créditos',en:'Credits Wallet'}
  };
  function applyLabels(){
    const l = lang();
    leadTitle.textContent = l==='es' ? 'ACCESOS PRINCIPALES' : 'QUICK ACCESS';
    groupLabel.textContent = l==='es' ? 'HERRAMIENTAS DE TU NEGOCIO' : 'BUSINESS TOOLS';
    groups.forEach(g => {
      const details=allGroups.get(g.id); if(!details)return;
      details.querySelector('.wt-menu-summary-text').textContent = g[l];
      details.querySelector('summary').title = g[l];
      details.querySelector('summary').setAttribute('aria-label', g[l]);
    });
    Object.entries(labels).forEach(([id, t]) => {
      const b=byView.get(id), span=b?.querySelector('span:last-child');
      if (!span) return;
      // No cambia IDs ni claves internas, solo el texto que ve el usuario.
      span.textContent=t[l]; b.setAttribute('title',t[l]);
    });
    buttons.forEach(b => { const label=b.querySelector("span:last-child")?.textContent?.trim(); if(label) b.setAttribute("title",label); });
  }
  function updateGroupStates(){
    allGroups.forEach(details => {
      const selected = !!details.querySelector('button[data-view].active');
      details.classList.toggle('wt-menu-has-active', selected);
    });
    nav.querySelectorAll('button[data-view]').forEach(b => {
      if(b.classList.contains('active'))b.setAttribute('aria-current','page');
      else b.removeAttribute('aria-current');
    });
  }
  nav.addEventListener('click', e => {
    const button=e.target.closest('button[data-view]');
    if(!button)return;
    const parent=button.closest('.wt-menu-group');
    if (parent && !parent.open) parent.open=true;
    window.requestAnimationFrame(updateGroupStates);
  });
  function selectByView(id) {
    const button=byView.get(id), group=button?.closest('.wt-menu-group');
    if (group && !group.open) group.open=true;
    updateGroupStates();
  }
  // Primer grupo abierto al entrar; recuerda la sección utilizada en la sesión.
  let stored='';try{stored=sessionStorage.getItem('wt-menu-open-group')||'';}catch(_){}
  const active=nav.querySelector('button[data-view].active')?.dataset.view||'dashboard';
  const selectedGroup=byView.get(active)?.closest('.wt-menu-group')?.dataset.group;
  const initial=selectedGroup || (allGroups.has(stored) ? stored : 'crm');
  if (allGroups.has(initial)) allGroups.get(initial).open=true;
  applyLabels();updateGroupStates();
  // El motor i18n traduce en DOMContentLoaded; aplicar etiquetas propias después.
  document.addEventListener("DOMContentLoaded", applyLabels);
  document.addEventListener('worktic:langchange',applyLabels);
  window.WorkticMenuV273513 = {refresh:applyLabels, selectView:selectByView, views:originalViews.slice()};
})();
