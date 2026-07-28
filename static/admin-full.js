/* Worktic Superadmin Full — módulo aislado */
(() => {
  const root = document.querySelector('#admin');
  if (!root) return;

  const state = {
    tab: 'users',
    plansAll: [],
    users: { page: 1, per_page: 20, search: '', status: 'all', role: 'all', plan: 'all' },
    plans: { page: 1, per_page: 20, search: '', status: 'all' },
    payments: { page: 1, per_page: 20, search: '', status: 'all', network: 'all', plan: 'all', from: '', to: '' },
    licenses: { page: 1, per_page: 20, search: '', status: 'all', source: 'all', plan: 'all' },
    timers: {}
  };

  const q = (s, p = document) => p.querySelector(s);
  const qa = (s, p = document) => [...p.querySelectorAll(s)];
  const safe = v => typeof esc === 'function' ? esc(v ?? '') : String(v ?? '').replace(/[&<>"']/g, m => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[m]));
  const fmtDate = v => v ? new Date(v).toLocaleDateString('es-ES') : '—';
  const fmtDateTime = v => v ? new Date(v).toLocaleString('es-ES') : '—';
  const money = v => `${Number(v || 0).toLocaleString('es-ES', {maximumFractionDigits: 2})} USDT`;
  const initials = v => String(v || '?').trim().split(/\s+/).slice(0,2).map(x => x[0]).join('').toUpperCase();
  const query = obj => new URLSearchParams(Object.entries(obj).filter(([,v]) => v !== '' && v != null)).toString();
  const notify = m => typeof toast === 'function' ? toast(m) : alert(m);
  const errMsg = e => e?.message || 'No fue posible completar la operación';
  const ask = async (title, message, confirmText = 'Confirmar', danger = false) => {
    if (typeof professionalConfirm === 'function') return professionalConfirm({title, message, confirmText, danger});
    return confirm(`${title}\n\n${message}`);
  };

  function debounce(key, fn, ms = 350) {
    clearTimeout(state.timers[key]);
    state.timers[key] = setTimeout(fn, ms);
  }

  function statusBadge(item) {
    if (item.deleted_at) return '<span class="wt-admin-badge deleted">Eliminado</span>';
    if (!item.active) return '<span class="wt-admin-badge blocked">Bloqueado</span>';
    return '<span class="wt-admin-badge active">Activo</span>';
  }

  function sourceLabel(v) {
    return ({courtesy:'Cortesía', payment:'Pago', checkout:'Pago', admin:'Administración', registration:'Registro', system:'Sistema'})[v] || v || 'Sistema';
  }

  function paginationHTML(meta, key) {
    const p = meta || {page:1,pages:0,total:0,per_page:20};
    const start = p.total ? (p.page - 1) * p.per_page + 1 : 0;
    const end = Math.min(p.total, p.page * p.per_page);
    return `<div class="wt-admin-pagination"><small>Mostrando ${start}–${end} de ${p.total}</small><div class="wt-admin-pages"><button class="secondary" data-page-key="${key}" data-page="${Math.max(1,p.page-1)}" ${p.page<=1?'disabled':''}>‹</button><span class="wt-admin-badge">Página ${p.page} de ${Math.max(1,p.pages||1)}</span><button class="secondary" data-page-key="${key}" data-page="${Math.min(Math.max(1,p.pages),p.page+1)}" ${p.page>=p.pages?'disabled':''}>›</button></div></div>`;
  }

  function bindPagination(container, loader) {
    qa('[data-page-key]', container).forEach(b => b.onclick = () => {
      const key = b.dataset.pageKey;
      state[key].page = Number(b.dataset.page);
      loader();
    });
  }

  async function loadPlansAll() {
    const data = await api('/api/admin/full/plans?per_page=100&status=all');
    state.plansAll = data.items || [];
    const options = `<option value="all">Todos los planes</option>${state.plansAll.map(p => `<option value="${safe(p.code)}">${safe(p.name)}</option>`).join('')}`;
    ['#wtAdminUserPlan','#wtAdminPaymentPlan','#wtAdminLicensePlan'].forEach(sel => {
      const el = q(sel); if (!el) return;
      const current = el.value;
      el.innerHTML = options;
      if ([...el.options].some(o => o.value === current)) el.value = current;
    });
  }

  async function loadOverview() {
    const o = await api('/api/admin/full/overview');
    const stats = [
      ['Usuarios',o.users],['Activos',o.active_users],['Bloqueados',o.blocked_users],['Eliminados',o.deleted_users],
      ['Licencias activas',o.active_subscriptions],['Cortesías',o.courtesy_licenses],['Pagos pendientes',o.pending_payments],['Ventas',money(o.revenue_usdt)]
    ];
    q('#wtAdminStats').innerHTML = stats.map(([label,value]) => `<div class="stat"><b>${safe(value)}</b><small>${safe(label)}</small></div>`).join('');
  }

  function activateTab(tab) {
    state.tab = tab;
    qa('.wt-admin-tab', root).forEach(x => x.classList.toggle('active', x.dataset.adminTab === tab));
    qa('.wt-admin-panel', root).forEach(x => x.classList.toggle('active', x.dataset.adminPanel === tab));
    ({users:loadUsers,plans:loadPlans,payments:loadPayments,licenses:loadLicenses})[tab]?.();
  }

  function bindBase() {
    qa('.wt-admin-tab', root).forEach(b => b.onclick = () => activateTab(b.dataset.adminTab));
    q('#wtAdminNewUser').onclick = () => userForm(null);
    q('#wtAdminNewPlan').onclick = () => planForm(null);
    q('#wtAdminNewCourtesy').onclick = () => courtesyForm(null);
    q('#newCommercialPlan').onclick = () => { activateTab('plans'); planForm(null); };

    const textFilter = (id, key, field, loader) => {
      q(id).oninput = e => { state[key][field] = e.target.value; state[key].page = 1; debounce(`${key}-${field}`, loader); };
    };
    const selectFilter = (id, key, field, loader) => {
      q(id).onchange = e => { state[key][field] = e.target.value; state[key].page = 1; loader(); };
    };
    textFilter('#wtAdminUserSearch','users','search',loadUsers);
    selectFilter('#wtAdminUserStatus','users','status',loadUsers);
    selectFilter('#wtAdminUserRole','users','role',loadUsers);
    selectFilter('#wtAdminUserPlan','users','plan',loadUsers);
    textFilter('#wtAdminPlanSearch','plans','search',loadPlans);
    selectFilter('#wtAdminPlanStatus','plans','status',loadPlans);
    textFilter('#wtAdminPaymentSearch','payments','search',loadPayments);
    selectFilter('#wtAdminPaymentStatus','payments','status',loadPayments);
    selectFilter('#wtAdminPaymentNetwork','payments','network',loadPayments);
    selectFilter('#wtAdminPaymentPlan','payments','plan',loadPayments);
    selectFilter('#wtAdminPaymentFrom','payments','from',loadPayments);
    selectFilter('#wtAdminPaymentTo','payments','to',loadPayments);
    textFilter('#wtAdminLicenseSearch','licenses','search',loadLicenses);
    selectFilter('#wtAdminLicenseStatus','licenses','status',loadLicenses);
    selectFilter('#wtAdminLicenseSource','licenses','source',loadLicenses);
    selectFilter('#wtAdminLicensePlan','licenses','plan',loadLicenses);
    q('#wtAdminClearUsers').onclick = () => resetFilters('users', {search:'',status:'all',role:'all',plan:'all'}, ['#wtAdminUserSearch','#wtAdminUserStatus','#wtAdminUserRole','#wtAdminUserPlan'], loadUsers);
    q('#wtAdminClearPayments').onclick = () => resetFilters('payments', {search:'',status:'all',network:'all',plan:'all',from:'',to:''}, ['#wtAdminPaymentSearch','#wtAdminPaymentStatus','#wtAdminPaymentNetwork','#wtAdminPaymentPlan','#wtAdminPaymentFrom','#wtAdminPaymentTo'], loadPayments);
  }

  function resetFilters(key, values, ids, loader) {
    Object.assign(state[key], values, {page:1});
    ids.forEach(id => { const el=q(id); if(el) el.value = state[key][({
      '#wtAdminUserSearch':'search','#wtAdminUserStatus':'status','#wtAdminUserRole':'role','#wtAdminUserPlan':'plan',
      '#wtAdminPaymentSearch':'search','#wtAdminPaymentStatus':'status','#wtAdminPaymentNetwork':'network','#wtAdminPaymentPlan':'plan','#wtAdminPaymentFrom':'from','#wtAdminPaymentTo':'to'
    })[id]] ?? ''; });
    loader();
  }

  async function loadUsers() {
    const el = q('#wtAdminUsersTable');
    el.innerHTML = '<div class="wt-admin-empty">Cargando usuarios…</div>';
    try {
      const data = await api('/api/admin/full/users?' + query(state.users));
      const items = data.items || [];
      el.innerHTML = items.length ? `<div class="wt-admin-table-wrap"><table class="wt-admin-table"><thead><tr><th>Usuario</th><th>Empresa</th><th>Rol</th><th>Licencia</th><th>Estado</th><th>Registro</th><th>Acciones</th></tr></thead><tbody>${items.map(u => `<tr>
        <td><div class="wt-admin-user-cell"><span class="wt-admin-avatar">${safe(initials(u.name))}</span><span><b>${safe(u.name)}</b><small>${safe(u.email)}</small></span></div></td>
        <td><b>${safe(u.tenant_name || u.company || '—')}</b><small style="display:block;color:#82788e">${safe(u.account_type || '')}${u.is_owner?' · Propietario':''}</small></td>
        <td><span class="wt-admin-badge">${safe(({owner:'Propietario',admin:'Administrador',supervisor:'Supervisor',agent:'Asesor',superadmin:'Superadmin'})[u.role]||u.role)}</span></td>
        <td><b>${safe(u.plan_name)}</b><small style="display:block;color:#82788e">${u.plan_ends_at ? 'Hasta '+fmtDate(u.plan_ends_at) : 'Sin fecha'}</small></td>
        <td>${statusBadge(u)}${u.blocked_reason?`<small style="display:block;margin-top:4px;color:#857a90">${safe(u.blocked_reason)}</small>`:''}</td>
        <td>${fmtDate(u.created_at)}</td>
        <td><div class="wt-admin-actions"><button class="secondary" data-user-edit="${u.id}">Editar</button><button class="secondary" data-user-license="${u.id}">Cortesía</button>${u.deleted_at?`<button data-user-restore="${u.id}">Restaurar</button>`:u.active?`<button class="wt-warn" data-user-block="${u.id}">Bloquear</button><button class="wt-danger" data-user-delete="${u.id}">Eliminar</button>`:`<button data-user-unblock="${u.id}">Desbloquear</button><button class="wt-danger" data-user-delete="${u.id}">Eliminar</button>`}</div></td>
      </tr>`).join('')}</tbody></table></div>${paginationHTML(data.pagination,'users')}` : '<div class="wt-admin-empty"><b>No encontramos usuarios</b><span>Ajusta los filtros o crea un nuevo usuario.</span></div>';
      bindPagination(el, loadUsers);
      qa('[data-user-edit]',el).forEach(b=>b.onclick=()=>userForm(items.find(x=>x.id===Number(b.dataset.userEdit))));
      qa('[data-user-license]',el).forEach(b=>b.onclick=()=>courtesyForm(items.find(x=>x.id===Number(b.dataset.userLicense))));
      qa('[data-user-block]',el).forEach(b=>b.onclick=()=>userAction(Number(b.dataset.userBlock),'block'));
      qa('[data-user-unblock]',el).forEach(b=>b.onclick=()=>userAction(Number(b.dataset.userUnblock),'unblock'));
      qa('[data-user-delete]',el).forEach(b=>b.onclick=()=>userAction(Number(b.dataset.userDelete),'delete'));
      qa('[data-user-restore]',el).forEach(b=>b.onclick=()=>userAction(Number(b.dataset.userRestore),'restore'));
    } catch(e) { el.innerHTML = `<div class="wt-admin-empty"><b>Error al cargar</b><span>${safe(errMsg(e))}</span></div>`; }
  }

  function userForm(u) {
    const editing = !!u;
    modal(`<div class="modal-heading"><span class="eyebrow">SUPERADMIN</span><h2>${editing?'Editar usuario':'Crear usuario'}</h2><p>Administra identidad, empresa, rol y acceso. Los cambios de contraseña cierran las sesiones activas.</p></div><div class="form">
      <div class="form-section"><div class="grid2"><label>Nombre completo<input id="wtAUName" value="${safe(u?.name||'')}"></label><label>Correo electrónico<input id="wtAUEmail" type="email" value="${safe(u?.email||'')}"></label></div><div class="grid2"><label>Empresa<input id="wtAUCompany" value="${safe(u?.company||u?.tenant_name||'')}"></label><label>Rol<select id="wtAURole">${(editing?['owner','admin','supervisor','agent','superadmin']:['owner','superadmin']).map(r=>`<option value="${r}" ${u?.role===r?'selected':''}>${safe(({owner:'Propietario',admin:'Administrador',supervisor:'Supervisor',agent:'Asesor',superadmin:'Superadministrador'})[r])}</option>`).join('')}</select></label></div><div class="grid2"><label>Tipo de cuenta<select id="wtAUAccount"><option value="business" ${u?.account_type!=='personal'?'selected':''}>Empresa</option><option value="personal" ${u?.account_type==='personal'?'selected':''}>Personal</option></select></label><label>${editing?'Nueva contraseña (opcional)':'Contraseña temporal'}<input id="wtAUPassword" type="password" autocomplete="new-password" placeholder="Mínimo 8 caracteres"></label></div></div>
      <div class="wt-admin-form-note">${editing?'Deja la contraseña vacía para conservar la actual.':'El usuario podrá iniciar sesión inmediatamente con esta contraseña.'}</div>
      <div class="actions"><button type="button" id="wtAUSave">${editing?'Guardar cambios':'Crear usuario'}</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);
    q('#wtAUSave').onclick = async () => {
      const data = {id:u?.id||0,action:'update',name:q('#wtAUName').value.trim(),email:q('#wtAUEmail').value.trim(),company:q('#wtAUCompany').value.trim(),role:q('#wtAURole').value,account_type:q('#wtAUAccount').value,password:q('#wtAUPassword').value};
      if (!data.name || !data.email || (!editing && data.password.length < 8)) return formError('Completa nombre, correo y una contraseña de mínimo 8 caracteres.');
      try { await api('/api/admin/full/users',{method:editing?'PUT':'POST',body:JSON.stringify(data)}); q('#modal').close(); notify(editing?'Usuario actualizado':'Usuario creado'); await Promise.all([loadUsers(),loadOverview()]); } catch(e){ formError(errMsg(e)); }
    };
  }

  async function userAction(id, action) {
    const cfg = {
      block:['Bloquear usuario','El usuario perderá el acceso inmediatamente y sus sesiones serán cerradas.','Bloquear',true],
      unblock:['Desbloquear usuario','El usuario podrá iniciar sesión nuevamente.','Desbloquear',false],
      delete:['Eliminar usuario','Se realizará una eliminación segura: el usuario desaparecerá de la operación, pero se conservarán sus datos para auditoría.','Eliminar',true],
      restore:['Restaurar usuario','El usuario y su acceso serán restaurados.','Restaurar',false]
    }[action];
    if (!await ask(...cfg)) return;
    let reason = '';
    if (action === 'block' || action === 'delete') reason = prompt('Motivo administrativo (opcional):','') || '';
    try { await api('/api/admin/full/users',{method:'PUT',body:JSON.stringify({id,action,reason})}); notify('Acción aplicada'); await Promise.all([loadUsers(),loadOverview()]); } catch(e){ notify(errMsg(e)); }
  }

  async function loadPlans() {
    const el=q('#wtAdminPlansTable'); el.innerHTML='<div class="wt-admin-empty">Cargando planes…</div>';
    try {
      const data=await api('/api/admin/full/plans?'+query(state.plans)); const items=data.items||[];
      el.innerHTML=items.length?`<div class="wt-admin-table-wrap"><table class="wt-admin-table"><thead><tr><th>Plan</th><th>Precio</th><th>Vigencia</th><th>Capacidades</th><th>Suscripciones</th><th>Estado</th><th>Acciones</th></tr></thead><tbody>${items.map(p=>`<tr><td><div class="wt-admin-plan-name"><b>${safe(p.name)}</b><small>${safe(p.code)} · ${safe(p.description)}</small></div></td><td><b>${money(p.price_usdt)}</b></td><td>${p.billing_days} días</td><td>${p.max_users} usuarios · ${p.max_channels} canales<br><small>${p.max_contacts} contactos · ${p.max_ai_responses} respuestas IA</small></td><td><b>${p.active_subscriptions}</b> activas<br><small>${p.subscriptions} históricas</small></td><td>${p.deleted_at?'<span class="wt-admin-badge deleted">Eliminado</span>':p.active?'<span class="wt-admin-badge active">Activo</span>':'<span class="wt-admin-badge blocked">Inactivo</span>'}</td><td><div class="wt-admin-actions">${p.deleted_at?`<button data-plan-restore="${p.id}">Restaurar</button>`:`<button class="secondary" data-plan-edit="${p.id}">Editar</button>${p.code!=='free'?`<button class="wt-danger" data-plan-delete="${p.id}">Eliminar</button>`:''}`}</div></td></tr>`).join('')}</tbody></table></div>${paginationHTML(data.pagination,'plans')}`:'<div class="wt-admin-empty"><b>No hay planes</b><span>Crea un plan comercial para comenzar.</span></div>';
      bindPagination(el,loadPlans);
      qa('[data-plan-edit]',el).forEach(b=>b.onclick=()=>planForm(items.find(x=>x.id===Number(b.dataset.planEdit))));
      qa('[data-plan-delete]',el).forEach(b=>b.onclick=()=>planDelete(Number(b.dataset.planDelete)));
      qa('[data-plan-restore]',el).forEach(b=>b.onclick=()=>planRestore(Number(b.dataset.planRestore)));
    } catch(e){el.innerHTML=`<div class="wt-admin-empty"><b>Error al cargar</b><span>${safe(errMsg(e))}</span></div>`;}
  }

  function planForm(p) {
    p=p||{billing_days:30,max_users:1,max_channels:1,max_whatsapp:1,max_telegram:1,max_messenger:0,max_agents:1,max_contacts:100,max_ai_responses:100,max_products:10,max_rules:5,features_json:'{"crm":true,"agenda":true}',active:true};
    modal(`<div class="modal-heading"><span class="eyebrow">PLANES WORKTIC</span><h2>${p.id?'Editar':'Nuevo'} plan comercial</h2><p>Define precio, duración, límites y capacidades disponibles.</p></div><div class="form"><div class="form-section"><div class="grid2"><label>Nombre<input id="wtAPName" value="${safe(p.name||'')}"></label><label>Código<input id="wtAPCode" value="${safe(p.code||'')}"></label></div><label>Descripción<textarea id="wtAPDesc">${safe(p.description||'')}</textarea></label><div class="grid2"><label>Precio USDT<input id="wtAPPrice" type="number" min="0" step="0.01" value="${p.price_usdt||0}"></label><label>Vigencia en días<input id="wtAPDays" type="number" min="1" value="${p.billing_days||30}"></label></div></div><div class="form-section"><h3 class="form-section-title">Capacidades</h3><div class="grid2"><label>Usuarios<input id="wtAPUsers" type="number" min="1" value="${p.max_users||1}"></label><label>Canales totales<input id="wtAPChannels" type="number" min="0" value="${p.max_channels||1}"></label><label>WhatsApp<input id="wtAPWA" type="number" min="0" value="${p.max_whatsapp||0}"></label><label>Telegram<input id="wtAPTG" type="number" min="0" value="${p.max_telegram||0}"></label><label>Messenger<input id="wtAPMSG" type="number" min="0" value="${p.max_messenger||0}"></label><label>Agentes IA<input id="wtAPAgents" type="number" min="0" value="${p.max_agents||1}"></label><label>Contactos<input id="wtAPContacts" type="number" min="0" value="${p.max_contacts||100}"></label><label>Respuestas IA/mes<input id="wtAPAI" type="number" min="0" value="${p.max_ai_responses||100}"></label><label>Productos<input id="wtAPProducts" type="number" min="0" value="${p.max_products||10}"></label><label>Automatizaciones<input id="wtAPRules" type="number" min="0" value="${p.max_rules||5}"></label></div><label>Características JSON<textarea id="wtAPFeatures" rows="5">${safe(p.features_json||'{}')}</textarea></label><label class="check"><input id="wtAPActive" type="checkbox" ${p.active!==false?'checked':''}> Plan activo y visible</label></div><div class="actions"><button type="button" id="wtAPSave">Guardar plan</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);
    q('#wtAPSave').onclick=async()=>{let features;try{features=JSON.parse(q('#wtAPFeatures').value||'{}')}catch{return formError('El JSON de características no es válido.')}const d={...p,name:q('#wtAPName').value.trim(),code:q('#wtAPCode').value.trim().toLowerCase(),description:q('#wtAPDesc').value.trim(),price_usdt:Number(q('#wtAPPrice').value),billing_days:Number(q('#wtAPDays').value),max_users:Number(q('#wtAPUsers').value),max_channels:Number(q('#wtAPChannels').value),max_whatsapp:Number(q('#wtAPWA').value),max_telegram:Number(q('#wtAPTG').value),max_messenger:Number(q('#wtAPMSG').value),max_agents:Number(q('#wtAPAgents').value),max_contacts:Number(q('#wtAPContacts').value),max_ai_responses:Number(q('#wtAPAI').value),max_products:Number(q('#wtAPProducts').value),max_rules:Number(q('#wtAPRules').value),features_json:JSON.stringify(features),active:q('#wtAPActive').checked};if(!d.name||!d.code)return formError('Nombre y código son obligatorios.');try{await api('/api/admin/full/plans',{method:p.id?'PUT':'POST',body:JSON.stringify(d)});q('#modal').close();notify('Plan guardado');await Promise.all([loadPlans(),loadPlansAll(),loadOverview()]);}catch(e){formError(errMsg(e))}};
  }

  async function planDelete(id){if(!await ask('Eliminar plan','El plan dejará de estar disponible para nuevas compras. Las licencias existentes conservarán su vigencia.','Eliminar',true))return;try{await api('/api/admin/full/plans?id='+id,{method:'DELETE'});notify('Plan eliminado');await Promise.all([loadPlans(),loadPlansAll(),loadOverview()]);}catch(e){notify(errMsg(e))}}
  async function planRestore(id){try{await api(`/api/admin/full/plans?id=${id}&action=restore`,{method:'PUT',body:'{}'});notify('Plan restaurado');await Promise.all([loadPlans(),loadPlansAll(),loadOverview()]);}catch(e){notify(errMsg(e))}}

  async function loadPayments(){
    const el=q('#wtAdminPaymentsTable');el.innerHTML='<div class="wt-admin-empty">Cargando pagos…</div>';
    try{const data=await api('/api/admin/full/payments?'+query(state.payments)),items=data.items||[],s=data.summary||{};q('#wtAdminPaymentSummary').innerHTML=[['Pendientes',s.pending],['Aprobados',s.approved],['Rechazados',s.rejected],['Ventas',money(s.revenue_usdt)]].map(([a,b])=>`<div><b>${safe(b)}</b><small>${safe(a)}</small></div>`).join('');el.innerHTML=items.length?`<div class="wt-admin-table-wrap"><table class="wt-admin-table"><thead><tr><th>Usuario</th><th>Plan</th><th>Red / Monto</th><th>Transacción</th><th>Fecha</th><th>Estado</th><th>Acciones</th></tr></thead><tbody>${items.map(x=>`<tr><td><b>${safe(x.user_name)}</b><small style="display:block;color:#82788e">${safe(x.user_email)}</small></td><td>${safe(x.plan_name)}</td><td><b>${safe(x.network)}</b><br>${money(x.amount_usdt)}</td><td><span class="wt-admin-hash" title="${safe(x.tx_hash)}">${safe(x.tx_hash)}</span></td><td>${fmtDateTime(x.created_at)}</td><td><span class="wt-admin-badge ${x.status==='approved'?'active':x.status==='rejected'?'deleted':'blocked'}">${safe(({pending:'Pendiente',approved:'Aprobado',rejected:'Rechazado'})[x.status]||x.status)}</span></td><td><div class="wt-admin-actions">${x.status==='pending'?`<button data-pay-approve="${x.id}">Aprobar</button><button class="wt-danger" data-pay-reject="${x.id}">Rechazar</button>`:`<button class="secondary" data-pay-detail="${x.id}">Detalle</button>`}</div></td></tr>`).join('')}</tbody></table></div>${paginationHTML(data.pagination,'payments')}`:'<div class="wt-admin-empty"><b>No hay pagos</b><span>La sección ya está lista para recibir y filtrar registros.</span></div>';bindPagination(el,loadPayments);qa('[data-pay-approve]',el).forEach(b=>b.onclick=()=>reviewPayment(Number(b.dataset.payApprove),'approve'));qa('[data-pay-reject]',el).forEach(b=>b.onclick=()=>reviewPayment(Number(b.dataset.payReject),'reject'));qa('[data-pay-detail]',el).forEach(b=>b.onclick=()=>paymentDetail(items.find(x=>x.id===Number(b.dataset.payDetail))));}catch(e){el.innerHTML=`<div class="wt-admin-empty"><b>Error al cargar</b><span>${safe(errMsg(e))}</span></div>`;}
  }

  function paymentDetail(x){modal(`<div class="modal-heading"><span class="eyebrow">PAGO #${x.id}</span><h2>${safe(x.plan_name)}</h2><p>${safe(x.user_name)} · ${safe(x.user_email)}</p></div><div class="form"><div class="form-section"><div class="grid2"><div><small>Red</small><b style="display:block">${safe(x.network)}</b></div><div><small>Monto</small><b style="display:block">${money(x.amount_usdt)}</b></div></div><label>Dirección<input readonly value="${safe(x.wallet)}"></label><label>Hash<input readonly value="${safe(x.tx_hash)}"></label><label>Nota administrativa<textarea readonly>${safe(x.admin_note||'Sin nota')}</textarea></label></div><div class="actions"><button value="cancel" class="secondary">Cerrar</button></div></div>`)}

  async function loadLicenses(){
    const el=q('#wtAdminLicensesTable');el.innerHTML='<div class="wt-admin-empty">Cargando licencias…</div>';
    try{const data=await api('/api/admin/full/subscriptions?'+query(state.licenses)),items=data.items||[];el.innerHTML=items.length?`<div class="wt-admin-table-wrap"><table class="wt-admin-table"><thead><tr><th>Usuario / Empresa</th><th>Plan</th><th>Origen</th><th>Inicio</th><th>Vencimiento</th><th>Estado</th><th>Acciones</th></tr></thead><tbody>${items.map(x=>`<tr><td><b>${safe(x.user_name)}</b><small style="display:block;color:#82788e">${safe(x.user_email)} · ${safe(x.company||'')}</small></td><td><b>${safe(x.plan_name)}</b></td><td><span class="wt-admin-badge ${x.source==='courtesy'?'courtesy':''}">${safe(sourceLabel(x.source))}</span>${x.granted_by_name?`<small style="display:block;margin-top:4px">por ${safe(x.granted_by_name)}</small>`:''}</td><td>${fmtDate(x.starts_at)}</td><td><b>${fmtDate(x.ends_at)}</b></td><td><span class="wt-admin-badge ${x.status==='active'?'active':x.status==='cancelled'?'deleted':'blocked'}">${safe(({active:'Activa',cancelled:'Cancelada',replaced:'Reemplazada',expired:'Vencida'})[x.status]||x.status)}</span></td><td><div class="wt-admin-actions"><button class="secondary" data-license-extend="${x.id}">Extender</button>${x.status==='active'?`<button class="wt-danger" data-license-cancel="${x.id}">Cancelar</button>`:''}</div></td></tr>`).join('')}</tbody></table></div>${paginationHTML(data.pagination,'licenses')}`:'<div class="wt-admin-empty"><b>No hay licencias</b><span>Crea una licencia de cortesía para cualquier usuario.</span></div>';bindPagination(el,loadLicenses);qa('[data-license-extend]',el).forEach(b=>b.onclick=()=>licenseAction(Number(b.dataset.licenseExtend),'extend'));qa('[data-license-cancel]',el).forEach(b=>b.onclick=()=>licenseAction(Number(b.dataset.licenseCancel),'cancel'));}catch(e){el.innerHTML=`<div class="wt-admin-empty"><b>Error al cargar</b><span>${safe(errMsg(e))}</span></div>`;}
  }

  async function courtesyForm(preselected){
    if(!state.plansAll.length) await loadPlansAll();
    let selected=preselected||null;
    modal(`<div class="modal-heading"><span class="eyebrow">LICENCIA DE CORTESÍA</span><h2>Otorgar licencia</h2><p>Activa cualquier plan sin registrar un pago. Si eliges un miembro de un equipo, la licencia se aplicará a la cuenta propietaria de esa empresa.</p></div><div class="form"><div class="form-section"><label>Buscar usuario<input id="wtACUserSearch" placeholder="Nombre, correo o empresa" value="${safe(preselected?.email||'')}"></label><div id="wtACUserResults" class="wt-admin-courtesy-results"></div><input id="wtACUserID" type="hidden" value="${preselected?.id||''}"></div><div class="form-section"><div class="grid2"><label>Plan<select id="wtACPlan">${state.plansAll.filter(p=>!p.deleted_at).map(p=>`<option value="${safe(p.code)}">${safe(p.name)} · ${money(p.price_usdt)}</option>`).join('')}</select></label><label>Días de cortesía<input id="wtACDays" type="number" min="1" max="3650" value="30"></label></div><label>O fecha exacta de vencimiento<input id="wtACEnd" type="date"><small>Si defines una fecha, tendrá prioridad sobre los días.</small></label><label>Nota administrativa<textarea id="wtACNote" rows="3" placeholder="Motivo de la cortesía, campaña, acuerdo comercial…"></textarea></label></div><div class="actions"><button type="button" id="wtACSave">Activar licencia</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);
    const results=q('#wtACUserResults');
    const drawSelected=()=>{if(selected)results.innerHTML=`<button type="button" class="wt-admin-courtesy-user selected"><span><b>${safe(selected.name)}</b><small>${safe(selected.email)} · ${safe(selected.tenant_name||selected.company||'')}</small></span><span class="wt-admin-badge active">Seleccionado</span></button>`};
    const searchUsers=async()=>{const s=q('#wtACUserSearch').value.trim();if(s.length<2&&!selected){results.innerHTML='<small>Escribe al menos 2 caracteres para buscar.</small>';return}try{const d=await api('/api/admin/full/users?per_page=25&status=all&search='+encodeURIComponent(s));const xs=d.items||[];results.innerHTML=xs.map(u=>`<button type="button" class="wt-admin-courtesy-user ${selected?.id===u.id?'selected':''}" data-courtesy-user="${u.id}"><span><b>${safe(u.name)}</b><small>${safe(u.email)} · ${safe(u.tenant_name||u.company||'')}</small></span><span class="wt-admin-badge">${safe(u.plan_name)}</span></button>`).join('')||'<small>No encontramos usuarios.</small>';qa('[data-courtesy-user]',results).forEach(b=>b.onclick=()=>{selected=xs.find(x=>x.id===Number(b.dataset.courtesyUser));q('#wtACUserID').value=selected.id;searchUsers()})}catch(e){results.innerHTML=`<small>${safe(errMsg(e))}</small>`}};
    q('#wtACUserSearch').oninput=()=>{selected=null;q('#wtACUserID').value='';debounce('courtesy-users',searchUsers,250)};
    preselected?drawSelected():searchUsers();
    q('#wtACSave').onclick=async()=>{const user_id=Number(q('#wtACUserID').value),plan_code=q('#wtACPlan').value,days=Number(q('#wtACDays').value),ends_at=q('#wtACEnd').value,note=q('#wtACNote').value.trim();if(!user_id)return formError('Selecciona un usuario.');try{await api('/api/admin/full/subscriptions',{method:'POST',body:JSON.stringify({user_id,plan_code,days,ends_at,note})});q('#modal').close();notify('Licencia de cortesía activada');await Promise.all([loadLicenses(),loadUsers(),loadOverview()]);}catch(e){formError(errMsg(e))}};
  }

  async function licenseAction(id,action){if(action==='cancel'){if(!await ask('Cancelar licencia','La licencia dejará de estar activa inmediatamente.','Cancelar licencia',true))return;try{await api('/api/admin/full/subscriptions',{method:'PUT',body:JSON.stringify({id,action:'cancel'})});notify('Licencia cancelada');await Promise.all([loadLicenses(),loadUsers(),loadOverview()]);}catch(e){notify(errMsg(e))};return}modal(`<div class="modal-heading"><span class="eyebrow">EXTENDER LICENCIA</span><h2>Agregar días</h2><p>La extensión se suma a la fecha actual de vencimiento. Si ya venció, se calcula desde hoy.</p></div><div class="form"><label>Días adicionales<input id="wtALExtendDays" type="number" min="1" value="30"></label><label>Nota<textarea id="wtALExtendNote"></textarea></label><div class="actions"><button type="button" id="wtALExtendSave">Extender</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);q('#wtALExtendSave').onclick=async()=>{try{await api('/api/admin/full/subscriptions',{method:'PUT',body:JSON.stringify({id,action:'extend',days:Number(q('#wtALExtendDays').value),note:q('#wtALExtendNote').value})});q('#modal').close();notify('Licencia extendida');await Promise.all([loadLicenses(),loadUsers(),loadOverview()]);}catch(e){formError(errMsg(e))}}}

  async function load() {
    bindBaseOnce();
    try {
      await Promise.all([loadOverview(),loadPlansAll()]);
      activateTab(state.tab);
    } catch(e) { notify(errMsg(e)); }
  }

  let bound=false;
  function bindBaseOnce(){if(bound)return;bound=true;bindBase()}

  // Sustituye solamente el cargador de la vista de superadministración.
  window.loadAdmin = load;
  try { loadAdmin = load; } catch (_) {}
  window.wtAdminFullLoad = load;
})();
