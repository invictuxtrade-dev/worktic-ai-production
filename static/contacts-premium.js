(() => {
  const sourceLabel = value => ({
    conversation: 'Conversación',
    form: 'Formulario',
    landing: 'Landing page',
    campaign: 'Campaña',
    manual: 'Manual',
    whatsapp: 'WhatsApp',
    telegram: 'Telegram',
    messenger: 'Messenger'
  })[value] || value || 'Manual';

  const channelLabel = value => ({
    whatsapp: 'WhatsApp',
    whatsapp_qr: 'WhatsApp',
    telegram: 'Telegram',
    messenger: 'Messenger',
    landing: 'Landing',
    form: 'Formulario',
    manual: 'Manual'
  })[value] || value || 'Manual';

  const contactDate = value => {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '—';
    return date.toLocaleString('es-CO', { dateStyle: 'medium', timeStyle: 'short' });
  };

  const dateOnly = value => {
    if (!value) return null;
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? null : date;
  };

  const currentFilters = () => ({
    search: ($('#crmContactSearch')?.value || '').trim().toLowerCase(),
    channel: $('#crmContactChannel')?.value || 'all',
    source: $('#crmContactSource')?.value || 'all',
    stage: $('#crmContactStage')?.value || 'all',
    from: $('#crmContactFrom')?.value || '',
    to: $('#crmContactTo')?.value || '',
    sort: $('#crmContactSort')?.value || 'recent'
  });

  function contactMatches(contact, filters) {
    const haystack = `${contact.name || ''} ${contact.phone || ''} ${contact.email || ''} ${contact.tags || ''} ${contact.notes || ''} ${contact.last_message || ''}`.toLowerCase();
    if (filters.search && !haystack.includes(filters.search)) return false;
    if (filters.channel !== 'all' && (contact.channel || 'manual') !== filters.channel) return false;
    if (filters.source !== 'all' && (contact.source || 'manual') !== filters.source) return false;
    if (filters.stage !== 'all' && (contact.stage || 'Nuevo') !== filters.stage) return false;

    const activity = dateOnly(contact.last_activity_at || contact.updated_at);
    if (filters.from) {
      const from = new Date(`${filters.from}T00:00:00`);
      if (activity && activity < from) return false;
    }
    if (filters.to) {
      const to = new Date(`${filters.to}T23:59:59`);
      if (activity && activity > to) return false;
    }
    return true;
  }

  function filteredContacts() {
    const filters = currentFilters();
    const contacts = crmContactsCache.filter(contact => contactMatches(contact, filters));
    contacts.sort((a, b) => {
      if (filters.sort === 'name') return (a.name || '').localeCompare(b.name || '', 'es');
      if (filters.sort === 'oldest') return (dateOnly(a.last_activity_at || a.updated_at)?.getTime() || 0) - (dateOnly(b.last_activity_at || b.updated_at)?.getTime() || 0);
      if (filters.sort === 'stage') return (a.stage || '').localeCompare(b.stage || '', 'es');
      return (dateOnly(b.last_activity_at || b.updated_at)?.getTime() || 0) - (dateOnly(a.last_activity_at || a.updated_at)?.getTime() || 0);
    });
    return contacts;
  }

  function renderStats() {
    const total = crmContactsCache.length;
    const newContacts = crmContactsCache.filter(x => (x.stage || 'Nuevo') === 'Nuevo').length;
    const conversations = crmContactsCache.filter(x => (x.conversation_count || 0) > 0 || x.source === 'conversation').length;
    const forms = crmContactsCache.filter(x => (x.lead_count || 0) > 0 || x.source === 'form').length;
    const node = $('#crmContactStats');
    if (!node) return;
    node.innerHTML = [
      ['Contactos totales', total],
      ['Nuevos', newContacts],
      ['Desde conversaciones', conversations],
      ['Desde formularios', forms]
    ].map(([label, value]) => `<div class="crm-contact-stat"><b>${Number(value).toLocaleString()}</b><small>${label}</small></div>`).join('');
  }

  function renderContacts() {
    const contacts = filteredContacts();
    const counter = $('#crmContactCount');
    if (counter) counter.textContent = `${contacts.length} contacto${contacts.length === 1 ? '' : 's'}`;

    const table = $('#contactsTable');
    if (!table) return;
    if (!contacts.length) {
      table.innerHTML = `<div class="crm-contact-empty"><div>◎</div><h3>No hay contactos para estos filtros</h3><p>Las personas que escriban por WhatsApp, Telegram, Messenger o completen un formulario aparecerán aquí automáticamente.</p></div>`;
      return;
    }

    table.innerHTML = `<div class="crm-contact-table-wrap"><table class="crm-contact-table">
      <thead><tr><th>Contacto</th><th>Origen</th><th>Etapa</th><th>Actividad</th><th>Relaciones</th><th>Acciones</th></tr></thead>
      <tbody>${contacts.map(contact => {
        const initial = esc((contact.name || contact.phone || contact.email || '?').trim().charAt(0).toUpperCase());
        const conversationCount = Number(contact.conversation_count || 0);
        const leadCount = Number(contact.lead_count || 0);
        const opportunityCount = Number(contact.opportunity_count || 0);
        return `<tr>
          <td><div class="crm-contact-person"><span class="crm-contact-avatar">${initial}</span><div><b>${esc(contact.name || 'Contacto sin nombre')}</b><small>${esc(contact.phone || 'Sin teléfono')}${contact.email ? ` · ${esc(contact.email)}` : ''}</small>${contact.last_message ? `<em>${esc(contact.last_message)}</em>` : ''}</div></div></td>
          <td><div class="crm-contact-origin"><span class="crm-channel crm-channel-${esc(contact.channel || 'manual')}">${esc(channelLabel(contact.channel))}</span><small>${esc(sourceLabel(contact.source))}</small></div></td>
          <td><span class="crm-stage crm-stage-${esc((contact.stage || 'Nuevo').toLowerCase().replace(/[^a-záéíóúñ]+/g, '-'))}">${esc(stageLabel(contact.stage || 'Nuevo'))}</span>${contact.tags ? `<small class="crm-contact-tags">${esc(contact.tags)}</small>` : ''}</td>
          <td><b class="crm-contact-date">${contactDate(contact.last_activity_at || contact.updated_at)}</b><small>Primera: ${contactDate(contact.first_seen_at || contact.created_at)}</small></td>
          <td><div class="crm-contact-relations"><span title="Mensajes">💬 ${conversationCount}</span><span title="Leads">◎ ${leadCount}</span><span title="Oportunidades">◫ ${opportunityCount}</span></div></td>
          <td><div class="crm-contact-actions">${contact.external_id ? `<button type="button" class="secondary" data-contact-chat="${esc(contact.external_id)}">Conversación</button>` : ''}<button type="button" class="secondary" data-contact-opportunity="${contact.id}">Oportunidad</button><button type="button" class="secondary" data-contact-edit="${contact.id}">Editar</button><button type="button" class="danger ghost-danger" data-contact-delete="${contact.id}">Eliminar</button></div></td>
        </tr>`;
      }).join('')}</tbody></table></div>`;

    table.querySelectorAll('[data-contact-edit]').forEach(button => button.onclick = () => contactFormById(Number(button.dataset.contactEdit)));
    table.querySelectorAll('[data-contact-delete]').forEach(button => button.onclick = () => deleteCRMContact(Number(button.dataset.contactDelete)));
    table.querySelectorAll('[data-contact-chat]').forEach(button => button.onclick = () => openContactConversation(button.dataset.contactChat));
    table.querySelectorAll('[data-contact-opportunity]').forEach(button => button.onclick = () => {
      const contact = crmContactsCache.find(item => item.id === Number(button.dataset.contactOpportunity));
      if (!contact) return;
      openOpportunityForm({ contact_id: contact.id, title: `Oportunidad · ${contact.name || contact.phone || 'Contacto'}`, stage: 'Nuevo', value: 0, owner: '' });
    });
  }

  function ensureContactLayout() {
    const view = $('#contacts');
    if (!view || view.dataset.crmPremiumReady === '1') return;
    view.dataset.crmPremiumReady = '1';
    const toolbar = view.querySelector('.toolbar');
    if (toolbar) {
      toolbar.className = 'crm-contact-heading';
      toolbar.innerHTML = `<div><span class="eyebrow">CRM UNIFICADO</span><h2>Contactos</h2><p>Cada conversación y cada lead alimenta automáticamente una ficha comercial única.</p></div><div class="crm-contact-heading-actions"><button type="button" id="syncCRMContacts" class="secondary">Actualizar</button><button type="button" id="newContact">Nuevo contacto</button></div>`;
    }
    const table = $('#contactsTable');
    const stats = document.createElement('div');
    stats.id = 'crmContactStats';
    stats.className = 'crm-contact-stats';
    const filters = document.createElement('div');
    filters.className = 'crm-contact-filters';
    filters.innerHTML = `<label class="crm-contact-search"><span>Buscar</span><input id="crmContactSearch" placeholder="Nombre, teléfono, correo, etiquetas o mensaje"></label>
      <label><span>Canal</span><select id="crmContactChannel"><option value="all">Todos</option><option value="whatsapp">WhatsApp</option><option value="telegram">Telegram</option><option value="messenger">Messenger</option><option value="landing">Landing</option><option value="manual">Manual</option></select></label>
      <label><span>Origen</span><select id="crmContactSource"><option value="all">Todos</option><option value="conversation">Conversaciones</option><option value="form">Formularios</option><option value="manual">Manual</option></select></label>
      <label><span>Etapa</span><select id="crmContactStage"><option value="all">Todas</option>${['Nuevo','Contactado','Interesado','Calificado','Cotización','Negociación','Ganado','Perdido'].map(stage => `<option value="${stage}">${stageLabel(stage)}</option>`).join('')}</select></label>
      <label><span>Desde</span><input id="crmContactFrom" type="date"></label>
      <label><span>Hasta</span><input id="crmContactTo" type="date"></label>
      <label><span>Ordenar</span><select id="crmContactSort"><option value="recent">Más recientes</option><option value="oldest">Más antiguos</option><option value="name">Nombre</option><option value="stage">Etapa</option></select></label>
      <button type="button" id="clearCRMContactFilters" class="secondary">Limpiar</button>`;
    const summary = document.createElement('div');
    summary.className = 'crm-contact-summary';
    summary.innerHTML = `<b id="crmContactCount">0 contactos</b><span>Sincronización automática activa</span>`;
    view.insertBefore(stats, table);
    view.insertBefore(filters, table);
    view.insertBefore(summary, table);

    ['crmContactChannel','crmContactSource','crmContactStage','crmContactFrom','crmContactTo','crmContactSort'].forEach(id => $(`#${id}`).addEventListener('change', renderContacts));
    $('#crmContactSearch').addEventListener('input', renderContacts);
    $('#clearCRMContactFilters').onclick = () => {
      $('#crmContactSearch').value = '';
      $('#crmContactChannel').value = 'all';
      $('#crmContactSource').value = 'all';
      $('#crmContactStage').value = 'all';
      $('#crmContactFrom').value = '';
      $('#crmContactTo').value = '';
      $('#crmContactSort').value = 'recent';
      renderContacts();
    };
    $('#syncCRMContacts').onclick = () => loadContacts(true);
    $('#newContact').onclick = () => openContactForm({ channel: 'manual', source: 'manual', stage: 'Nuevo' });
  }

  loadContacts = async function(showConfirmation = false) {
    ensureContactLayout();
    const button = $('#syncCRMContacts');
    if (button) {
      button.disabled = true;
      button.textContent = 'Actualizando…';
    }
    try {
      crmContactsCache = await api('/api/crm/contacts');
      renderStats();
      renderContacts();
      if (showConfirmation) toast('Contactos sincronizados');
    } catch (error) {
      const table = $('#contactsTable');
      if (table) table.innerHTML = `<div class="crm-contact-empty error"><h3>No fue posible cargar los contactos</h3><p>${esc(error.message)}</p></div>`;
    } finally {
      if (button) {
        button.disabled = false;
        button.textContent = 'Actualizar';
      }
    }
  };

  openContactConversation = chat => {
    const navButton = [...document.querySelectorAll('nav button')].find(button => button.dataset.view === 'inbox');
    if (!navButton) return;
    navButton.click();
    setTimeout(() => openChat(chat), 180);
  };

  contactFormById = id => {
    const contact = crmContactsCache.find(item => item.id === id);
    if (contact) openContactForm(contact);
  };

  openContactForm = contact => {
    const item = contact || {};
    modal(`<div class="modal-heading"><span class="eyebrow">CRM UNIFICADO</span><h2>${item.id ? 'Editar' : 'Nuevo'} contacto</h2><p>Completa la ficha comercial. Los datos automáticos de conversaciones y formularios se conservarán.</p></div>
      <div class="form crm-contact-form"><div class="grid2"><label>Nombre<input id="crmName" value="${esc(item.name || '')}" placeholder="Nombre del contacto"></label><label>Teléfono<input id="crmPhone" value="${esc(item.phone || '')}" placeholder="573001234567"></label></div>
      <label>Correo<input id="crmEmail" type="email" value="${esc(item.email || '')}" placeholder="cliente@empresa.com"></label>
      <div class="grid2"><label>Canal principal<select id="crmChannel">${['manual','whatsapp','telegram','messenger','landing','form'].map(value => `<option value="${value}" ${item.channel === value ? 'selected' : ''}>${channelLabel(value)}</option>`).join('')}</select></label><label>Etapa comercial<select id="crmStage">${['Nuevo','Contactado','Interesado','Calificado','Cotización','Negociación','Ganado','Perdido'].map(value => `<option value="${value}" ${(item.stage || 'Nuevo') === value ? 'selected' : ''}>${stageLabel(value)}</option>`).join('')}</select></label></div>
      <label>Etiquetas<input id="crmTags" value="${esc(item.tags || '')}" placeholder="vip, interesado, medellín"></label>
      <label>Notas<textarea id="crmNotes" rows="6" placeholder="Contexto comercial, preferencias y próximos pasos">${esc(item.notes || '')}</textarea></label>
      ${item.source ? `<div class="notice"><b>Origen:</b> ${esc(sourceLabel(item.source))}${item.first_seen_at ? ` · <b>Primera interacción:</b> ${contactDate(item.first_seen_at)}` : ''}</div>` : ''}
      <div class="actions"><button type="button" id="saveCRMContact">Guardar contacto</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);

    $('#saveCRMContact').onclick = async () => {
      const button = $('#saveCRMContact');
      const payload = {
        ...item,
        name: $('#crmName').value.trim(),
        phone: $('#crmPhone').value.trim(),
        email: $('#crmEmail').value.trim(),
        channel: $('#crmChannel').value,
        source: item.source || 'manual',
        stage: $('#crmStage').value,
        tags: $('#crmTags').value.trim(),
        notes: $('#crmNotes').value.trim()
      };
      if (!payload.name && !payload.phone && !payload.email) return formError('Escribe al menos el nombre, teléfono o correo.');
      button.disabled = true;
      button.textContent = 'Guardando…';
      try {
        const response = await api('/api/crm/contacts', { method: item.id ? 'PUT' : 'POST', body: JSON.stringify(payload) });
        $('#modal').close();
        toast(response.merged ? 'Contacto existente actualizado' : 'Contacto guardado');
        await loadContacts();
      } catch (error) {
        formError(error.message);
        button.disabled = false;
        button.textContent = 'Guardar contacto';
      }
    };
  };

  deleteCRMContact = async id => {
    const contact = crmContactsCache.find(item => item.id === id);
    const confirmed = await professionalConfirm({
      title: 'Eliminar contacto',
      message: `Se ocultará la ficha de ${contact?.name || 'este contacto'}. Las conversaciones y leads originales se conservarán. Si la persona vuelve a escribir, el contacto se activará nuevamente.`,
      confirmText: 'Eliminar',
      danger: true
    });
    if (!confirmed) return;
    try {
      await api(`/api/crm/contacts?id=${id}`, { method: 'DELETE' });
      toast('Contacto eliminado');
      await loadContacts();
    } catch (error) {
      toast(error.message);
    }
  };

  ensureContactLayout();
})();
