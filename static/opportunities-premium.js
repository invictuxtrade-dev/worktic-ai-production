(() => {
  'use strict';

  const opportunityStages = ['Nuevo','Interesado','Calificado','Cotización','Negociación','Ganado','Perdido'];
  let opportunityItems = [];
  let opportunityContacts = [];

  const qs = selector => document.querySelector(selector);
  const safe = value => typeof esc === 'function' ? esc(String(value ?? '')) : String(value ?? '').replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
  const stageText = value => typeof stageLabel === 'function' ? stageLabel(value) : value;
  const money = value => `$${Number(value || 0).toLocaleString('es-CO', {maximumFractionDigits: 0})}`;
  const dateText = value => {
    if (!value) return 'Sin actividad';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString('es-CO', {day:'2-digit', month:'short', hour:'2-digit', minute:'2-digit'});
  };
  const sourceText = source => ({conversation:'Conversación', form:'Formulario', landing:'Landing', crm:'CRM', manual:'Manual', automation:'Automatización', worktic_form:'Formulario'})[source] || source || 'Automático';
  const channelText = channel => ({whatsapp:'WhatsApp', whatsapp_qr:'WhatsApp', telegram:'Telegram', messenger:'Messenger', landing:'Landing', form:'Formulario', manual:'Manual'})[channel] || channel || 'Sin canal';
  const activeOpportunity = item => !['Ganado','Perdido'].includes(item.stage);

  function filters() {
    return {
      search: (qs('#oppSearch')?.value || '').trim().toLowerCase(),
      stage: qs('#oppStageFilter')?.value || 'all',
      source: qs('#oppSourceFilter')?.value || 'all',
      channel: qs('#oppChannelFilter')?.value || 'all',
      sort: qs('#oppSort')?.value || 'activity'
    };
  }

  function filteredItems() {
    const f = filters();
    const list = opportunityItems.filter(item => {
      const haystack = [item.title,item.contact_name,item.contact_phone,item.contact_email,item.owner,item.interest,item.last_message,sourceText(item.source),channelText(item.channel || item.contact_channel)].join(' ').toLowerCase();
      if (f.search && !haystack.includes(f.search)) return false;
      if (f.stage !== 'all' && item.stage !== f.stage) return false;
      if (f.source !== 'all' && (item.source || 'manual') !== f.source) return false;
      const channel = item.channel || item.contact_channel || 'manual';
      if (f.channel !== 'all' && channel !== f.channel) return false;
      return true;
    });
    list.sort((a,b) => {
      if (f.sort === 'value') return Number(b.value || 0) - Number(a.value || 0);
      if (f.sort === 'score') return Number(b.score || 0) - Number(a.score || 0);
      if (f.sort === 'oldest') return new Date(a.last_activity_at || a.updated_at || 0) - new Date(b.last_activity_at || b.updated_at || 0);
      return new Date(b.last_activity_at || b.updated_at || 0) - new Date(a.last_activity_at || a.updated_at || 0);
    });
    return list;
  }

  function ensureLayout() {
    const view = qs('#pipeline');
    if (!view || view.dataset.opportunityPremiumReady === '1') return;
    view.dataset.opportunityPremiumReady = '1';
    view.innerHTML = `
      <div class="opp-premium-head">
        <div><span class="eyebrow">PIPELINE INTELIGENTE</span><h2>Oportunidades</h2><p>Worktic crea y actualiza oportunidades automáticamente cuando detecta intención comercial, recibe un formulario o el contacto avanza de etapa.</p></div>
        <div class="opp-head-actions"><button type="button" id="refreshOpps" class="secondary">Actualizar</button><button type="button" id="newOpp">Nueva oportunidad manual</button></div>
      </div>
      <div id="oppStats" class="opp-stats"></div>
      <div class="opp-automation-note"><div class="opp-automation-icon">✦</div><div><strong>Flujo automático activo</strong><span>Precio, cotización, compra, agenda, formularios y cambios de etapa alimentan este pipeline sin duplicar negocios abiertos.</span></div></div>
      <div class="opp-filters">
        <label class="opp-search"><span>Buscar</span><input id="oppSearch" placeholder="Oportunidad, contacto, responsable o mensaje"></label>
        <label><span>Etapa</span><select id="oppStageFilter"><option value="all">Todas</option>${opportunityStages.map(stage => `<option value="${stage}">${stageText(stage)}</option>`).join('')}</select></label>
        <label><span>Origen</span><select id="oppSourceFilter"><option value="all">Todos</option><option value="conversation">Conversación</option><option value="form">Formulario</option><option value="crm">CRM</option><option value="manual">Manual</option></select></label>
        <label><span>Canal</span><select id="oppChannelFilter"><option value="all">Todos</option><option value="whatsapp">WhatsApp</option><option value="telegram">Telegram</option><option value="messenger">Messenger</option><option value="landing">Landing</option><option value="manual">Manual</option></select></label>
        <label><span>Ordenar</span><select id="oppSort"><option value="activity">Actividad reciente</option><option value="score">Mayor puntuación</option><option value="value">Mayor valor</option><option value="oldest">Más antiguas</option></select></label>
        <button type="button" id="clearOppFilters" class="secondary">Limpiar</button>
      </div>
      <div class="opp-filter-summary"><b id="oppResultCount">0 oportunidades</b><span id="oppResultSummary">Todos los flujos</span></div>
      <div id="pipelineBoard" class="opp-board"></div>`;

    ['oppStageFilter','oppSourceFilter','oppChannelFilter','oppSort'].forEach(id => qs(`#${id}`)?.addEventListener('change', renderOpportunities));
    qs('#oppSearch')?.addEventListener('input', renderOpportunities);
    qs('#clearOppFilters').onclick = () => {
      qs('#oppSearch').value = '';
      qs('#oppStageFilter').value = 'all';
      qs('#oppSourceFilter').value = 'all';
      qs('#oppChannelFilter').value = 'all';
      qs('#oppSort').value = 'activity';
      renderOpportunities();
    };
    qs('#refreshOpps').onclick = () => loadOpps(true);
    qs('#newOpp').onclick = () => openOpportunityForm({stage:'Nuevo',value:0,score:50,source:'manual',automation_state:'manual'});
  }

  function renderStats() {
    const open = opportunityItems.filter(activeOpportunity);
    const qualified = open.filter(item => ['Calificado','Cotización','Negociación'].includes(item.stage));
    const automatic = opportunityItems.filter(item => item.automation_state === 'automatic' || item.source !== 'manual');
    const value = open.reduce((sum,item) => sum + Number(item.value || 0), 0);
    const stats = qs('#oppStats');
    if (!stats) return;
    stats.innerHTML = [
      [open.length,'Oportunidades abiertas'],
      [qualified.length,'Listas para seguimiento'],
      [money(value),'Valor del pipeline'],
      [automatic.length,'Generadas automáticamente']
    ].map(([number,label]) => `<div class="opp-stat"><b>${number}</b><small>${label}</small></div>`).join('');
  }

  function nextStage(stage) {
    const index = opportunityStages.indexOf(stage);
    if (index < 0 || index >= opportunityStages.length - 2) return '';
    return opportunityStages[index + 1];
  }

  function renderCard(item) {
    const channel = item.channel || item.contact_channel || 'manual';
    const automated = item.automation_state === 'automatic' || item.source !== 'manual';
    const contact = item.contact_name || item.contact_phone || item.contact_email || 'Sin contacto relacionado';
    const contactDetail = [item.contact_phone,item.contact_email].filter(Boolean).join(' · ');
    const next = nextStage(item.stage);
    return `<article class="opp-card" draggable="true" data-opp-id="${item.id}">
      <div class="opp-card-top"><h4>${safe(item.title)}</h4><span class="opp-auto-badge ${automated ? '' : 'manual'}">${automated ? 'Automática' : 'Manual'}</span></div>
      <div class="opp-contact"><b>${safe(contact)}</b><small>${safe(contactDetail || item.interest || 'Sin datos adicionales')}</small></div>
      <div class="opp-card-meta"><span class="opp-chip ${safe(channel)}">${safe(channelText(channel))}</span><span class="opp-chip">${safe(sourceText(item.source))}</span><span class="opp-chip">Score ${Number(item.score || 0)}</span></div>
      <div class="opp-value-row"><strong>${money(item.value)}</strong><small>${Number(item.probability || 0)}% prob.</small></div>
      <div class="opp-score" title="Puntuación comercial ${Number(item.score || 0)}"><span style="--opp-score:${Math.max(0,Math.min(100,Number(item.score || 0)))}%"></span></div>
      ${item.last_message ? `<p class="opp-card-message">${safe(item.last_message)}</p>` : ''}
      <div class="opp-card-meta"><span class="opp-chip">${safe(item.owner || 'Sin asignar')}</span><span class="opp-chip">${safe(dateText(item.last_activity_at || item.updated_at))}</span></div>
      <div class="opp-card-actions">${next ? `<button type="button" class="secondary" data-opp-next="${item.id}" data-next-stage="${safe(next)}">Avanzar</button>` : ''}<button type="button" class="secondary" data-opp-edit="${item.id}">Editar</button><button type="button" class="danger ghost-danger" data-opp-delete="${item.id}">Eliminar</button></div>
    </article>`;
  }

  function renderOpportunities() {
    const board = qs('#pipelineBoard');
    if (!board) return;
    const list = filteredItems();
    const selectedStage = filters().stage;
    const visibleStages = selectedStage === 'all' ? opportunityStages : [selectedStage];
    board.innerHTML = visibleStages.map(stage => {
      const items = list.filter(item => item.stage === stage);
      const total = items.reduce((sum,item) => sum + Number(item.value || 0), 0);
      return `<section class="opp-column" data-stage="${safe(stage)}"><div class="opp-column-head"><div><b>${safe(stageText(stage))}</b><small class="opp-column-value">${money(total)}</small></div><span>${items.length}</span></div>${items.length ? items.map(renderCard).join('') : '<div class="opp-empty-column">Sin oportunidades en esta etapa</div>'}</section>`;
    }).join('');

    qs('#oppResultCount').textContent = `${list.length} oportunidad${list.length === 1 ? '' : 'es'}`;
    const f = filters();
    const summary = [];
    if (f.stage !== 'all') summary.push(stageText(f.stage));
    if (f.source !== 'all') summary.push(sourceText(f.source));
    if (f.channel !== 'all') summary.push(channelText(f.channel));
    qs('#oppResultSummary').textContent = summary.length ? summary.join(' · ') : 'Todos los flujos';

    board.querySelectorAll('[data-opp-edit]').forEach(button => button.onclick = () => opportunityFormById(Number(button.dataset.oppEdit)));
    board.querySelectorAll('[data-opp-delete]').forEach(button => button.onclick = () => deleteOpportunity(Number(button.dataset.oppDelete)));
    board.querySelectorAll('[data-opp-next]').forEach(button => button.onclick = () => moveOpportunity(Number(button.dataset.oppNext), button.dataset.nextStage));
    wireDragAndDrop(board);
  }

  function wireDragAndDrop(board) {
    board.querySelectorAll('.opp-card').forEach(card => {
      card.addEventListener('dragstart', event => {
        card.classList.add('dragging');
        event.dataTransfer.setData('text/plain', card.dataset.oppId);
        event.dataTransfer.effectAllowed = 'move';
      });
      card.addEventListener('dragend', () => card.classList.remove('dragging'));
    });
    board.querySelectorAll('.opp-column').forEach(column => {
      column.addEventListener('dragover', event => { event.preventDefault(); column.classList.add('drag-over'); });
      column.addEventListener('dragleave', () => column.classList.remove('drag-over'));
      column.addEventListener('drop', async event => {
        event.preventDefault();
        column.classList.remove('drag-over');
        const id = Number(event.dataTransfer.getData('text/plain'));
        if (id) await moveOpportunity(id, column.dataset.stage);
      });
    });
  }

  async function moveOpportunity(id, stage) {
    const item = opportunityItems.find(entry => Number(entry.id) === Number(id));
    if (!item || !stage || item.stage === stage) return;
    try {
      await api('/api/crm/opportunities', {method:'PUT', body:JSON.stringify({...item, stage})});
      item.stage = stage;
      item.probability = ({Nuevo:10,Interesado:25,Calificado:45,'Cotización':65,'Negociación':80,Ganado:100,Perdido:0})[stage] ?? item.probability;
      item.updated_at = new Date().toISOString();
      renderStats();
      renderOpportunities();
      toast(`Oportunidad movida a ${stageText(stage)}`);
      if (typeof loadContacts === 'function' && qs('#contacts')?.classList.contains('active')) loadContacts();
    } catch (error) {
      toast(error.message);
    }
  }

  async function fetchContacts() {
    try {
      opportunityContacts = await api('/api/crm/contacts');
      if (Array.isArray(window.crmContactsCache)) window.crmContactsCache = opportunityContacts;
    } catch (_) {
      opportunityContacts = Array.isArray(window.crmContactsCache) ? window.crmContactsCache : [];
    }
  }

  window.loadOpps = async function(showConfirmation = false) {
    ensureLayout();
    const refresh = qs('#refreshOpps');
    if (refresh) { refresh.disabled = true; refresh.textContent = 'Actualizando…'; }
    try {
      const [items] = await Promise.all([api('/api/crm/opportunities'), fetchContacts()]);
      opportunityItems = Array.isArray(items) ? items : [];
      window.opportunitiesCache = opportunityItems;
      renderStats();
      renderOpportunities();
      if (showConfirmation) toast('Pipeline actualizado');
    } catch (error) {
      const board = qs('#pipelineBoard');
      if (board) board.innerHTML = `<div class="opp-empty"><h3>No fue posible cargar oportunidades</h3><p>${safe(error.message)}</p></div>`;
    } finally {
      if (refresh) { refresh.disabled = false; refresh.textContent = 'Actualizar'; }
    }
  };

  window.opportunityFormById = id => {
    const item = opportunityItems.find(entry => Number(entry.id) === Number(id));
    if (item) openOpportunityForm(item);
  };

  window.openOpportunityForm = async function(item = {}) {
    if (!opportunityContacts.length) await fetchContacts();
    const selectedContact = opportunityContacts.find(contact => Number(contact.id) === Number(item.contact_id));
    const defaultTitle = item.title || (selectedContact ? `Oportunidad · ${selectedContact.name || selectedContact.phone || 'Contacto'}` : '');
    modal(`<div class="modal-heading"><span class="eyebrow">PIPELINE INTELIGENTE</span><h2>${item.id ? 'Editar' : 'Nueva'} oportunidad</h2><p>${item.id && item.automation_state === 'automatic' ? 'Esta oportunidad nació automáticamente. Puedes completar valor, responsable y seguimiento sin perder su origen.' : 'La creación manual sigue disponible; Worktic continuará actualizando el flujo automáticamente.'}</p></div>
      <div class="form opp-premium-form">
        <div class="opp-form-note">Las oportunidades automáticas se generan desde conversaciones con intención de compra, formularios y avances del contacto. Worktic evita crear duplicados abiertos para la misma persona.</div>
        <label>Contacto relacionado<select id="ofContact"><option value="0">Sin contacto relacionado</option>${opportunityContacts.map(contact => `<option value="${contact.id}" ${Number(item.contact_id) === Number(contact.id) ? 'selected' : ''}>${safe(contact.name || contact.phone || contact.email || `Contacto #${contact.id}`)} · ${safe(channelText(contact.channel))}</option>`).join('')}</select></label>
        <label>Nombre de la oportunidad<input id="ofTitle" value="${safe(defaultTitle)}" placeholder="Ej. Plan empresarial · Cliente"></label>
        <div class="grid2"><label>Etapa<select id="ofStage">${opportunityStages.map(stage => `<option value="${stage}" ${item.stage === stage ? 'selected' : ''}>${safe(stageText(stage))}</option>`).join('')}</select></label><label>Valor estimado<input id="ofValue" type="number" min="0" step="1" value="${Number(item.value || 0)}"></label></div>
        <div class="grid2"><label>Responsable<input id="ofOwner" value="${safe(item.owner || '')}" placeholder="Nombre del asesor"></label><label>Canal<select id="ofChannel">${['manual','whatsapp','telegram','messenger','landing'].map(channel => `<option value="${channel}" ${(item.channel || item.contact_channel || 'manual') === channel ? 'selected' : ''}>${safe(channelText(channel))}</option>`).join('')}</select></label></div>
        <label>Interés o necesidad<input id="ofInterest" value="${safe(item.interest || '')}" placeholder="Producto, servicio, cotización o necesidad principal"></label>
        <div class="grid2"><label>Puntuación comercial<input id="ofScore" type="number" min="0" max="100" value="${Number(item.score ?? 50)}"></label><div class="opp-probability"><span>Probabilidad según etapa</span><b id="ofProbability">${Number(item.probability || 10)}%</b></div></div>
        <div class="actions"><button type="button" id="saveOpportunity">Guardar oportunidad</button><button value="cancel" class="secondary">Cancelar</button></div>
      </div>`);

    const probabilities = {Nuevo:10,Interesado:25,Calificado:45,'Cotización':65,'Negociación':80,Ganado:100,Perdido:0};
    qs('#ofStage').onchange = event => { qs('#ofProbability').textContent = `${probabilities[event.target.value] ?? 10}%`; };
    qs('#ofContact').onchange = event => {
      if (qs('#ofTitle').value.trim()) return;
      const contact = opportunityContacts.find(entry => Number(entry.id) === Number(event.target.value));
      if (contact) qs('#ofTitle').value = `Oportunidad · ${contact.name || contact.phone || 'Contacto'}`;
    };
    qs('#saveOpportunity').onclick = async () => {
      const data = {
        ...item,
        contact_id:Number(qs('#ofContact').value || 0),
        title:qs('#ofTitle').value.trim(),
        stage:qs('#ofStage').value,
        value:Number(qs('#ofValue').value || 0),
        owner:qs('#ofOwner').value.trim(),
        channel:qs('#ofChannel').value,
        interest:qs('#ofInterest').value.trim(),
        score:Number(qs('#ofScore').value || 0),
        source:item.source || 'manual',
        automation_state:item.automation_state || 'manual'
      };
      if (!data.title) return formError('Escribe el nombre de la oportunidad.');
      const button = qs('#saveOpportunity');
      button.disabled = true;
      button.textContent = 'Guardando…';
      try {
        const result = await api('/api/crm/opportunities', {method:item.id ? 'PUT' : 'POST', body:JSON.stringify(data)});
        qs('#modal').close();
        toast(result?.merged ? 'Oportunidad existente actualizada' : 'Oportunidad guardada');
        await loadOpps();
        if (typeof loadContacts === 'function' && qs('#contacts')?.classList.contains('active')) loadContacts();
      } catch (error) {
        formError(error.message);
        button.disabled = false;
        button.textContent = 'Guardar oportunidad';
      }
    };
  };

  window.deleteOpportunity = async function(id) {
    if (!await professionalConfirm({title:'Eliminar oportunidad',message:'La oportunidad se ocultará del pipeline. El contacto y sus conversaciones se conservarán.',confirmText:'Eliminar',danger:true})) return;
    try {
      await api(`/api/crm/opportunities?id=${id}`, {method:'DELETE'});
      toast('Oportunidad eliminada');
      await loadOpps();
    } catch (error) {
      toast(error.message);
    }
  };

  const existingButton = qs('#newOpp');
  if (existingButton) existingButton.onclick = () => openOpportunityForm({stage:'Nuevo',value:0,score:50,source:'manual'});
})();
