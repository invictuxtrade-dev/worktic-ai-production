/* Worktic AI - Agent Builder A2. Normal chat + on-demand form extraction. No save/publish. */
(() => {
  'use strict';
  const byId = id => document.getElementById(id);
  const TYPES = ['general','sales','support','appointments','campaigns','community','custom'];
  const TOOLS = ['contact','opportunity','appointment','catalog','handoff'];
  const CHANNELS = ['whatsapp','telegram','messenger','webchat','voice'];
  const FIELDS = [
    ['name','Nombre'],['type','Especialidad'],['objective','Objetivo'],['tone','Tono'],
    ['language','Idioma'],['instructions','Instrucciones'],['knowledge','Conocimiento'],
    ['greeting','Bienvenida'],['handoff_rules','Transferencia']
  ];
  const state = { messages: [], proposal: null, proposalAt: -1, busy: false, initialised: false };
  let dialog = null;

  function el(tag, content, className) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (content !== undefined && content !== null) node.textContent = String(content);
    return node;
  }
  function status(message) { const node = byId('wtBuilderStatus'); if (node) node.textContent = message; }
  function scrollDown() { const feed = byId('wtBuilderMessages'); if (feed) feed.scrollTop = feed.scrollHeight; }
  function messageBubble(role, content, extra = '') {
    const feed = byId('wtBuilderMessages'); if (!feed) return null;
    const wrap = el('div', null, 'wt-builder-turn ' + (role === 'user' ? 'wt-builder-turn-user' : 'wt-builder-turn-ai') + ' ' + extra);
    if (role !== 'user') wrap.append(el('span','✦','wt-builder-avatar'));
    wrap.append(el('div', content, 'wt-builder-bubble ' + (role === 'user' ? 'wt-builder-user' : 'wt-builder-ai')));
    feed.append(wrap); scrollDown(); return wrap;
  }
  function typingBubble() {
    const node = messageBubble('assistant','', 'wt-builder-typing');
    if (node) {
      const b = node.querySelector('.wt-builder-bubble');
      if (b) { b.textContent = ''; const dots = el('span',null,'wt-builder-dots'); dots.setAttribute('aria-label','El asistente está respondiendo'); dots.innerHTML = '<i></i><i></i><i></i>'; b.append(dots); }
    }
    return node;
  }
  function updateComposer() {
    const send = byId('wtBuilderSend'); const use = byId('wtBuilderUse'); const prep = byId('wtBuilderPrepare');
    const input = byId('wtBuilderInput'); const bottom = byId('wtBuilderBottomUse');
    if (send) send.disabled = state.busy || !(input && input.value.trim());
    if (use) use.disabled = state.busy || !state.messages.some(m => m.role === 'user');
    if (bottom) bottom.disabled = state.busy || !state.messages.some(m => m.role === 'user');
    if (prep) prep.disabled = state.busy || !state.messages.some(m => m.role === 'user');
    if (input) input.setAttribute('aria-busy',state.busy ? 'true':'false');
  }
  function renderProposal() {
    const summary = byId('wtBuilderSummary'); if (!summary) return;
    summary.replaceChildren();
    const p=state.proposal;
    if (!p) {
      const empty = el('div',null,'wt-builder-summary-empty');
      empty.append(el('div','✦','wt-builder-empty-icon'));
      empty.append(el('h4','Tu agente, a tu manera'));
      empty.append(el('p','Conversa sin llenar cuestionarios. Cuando quieras, prepararé una configuración a partir de lo que hayamos hablado.'));
      summary.append(empty);
      return;
    }
    if (state.proposalAt !== state.messages.length) {
      summary.append(el('p','Tu conversación tiene novedades. Actualiza la propuesta antes de transferirla.','wt-builder-stale'));
    }
    for (const [key,label] of FIELDS) {
      const value = String(p[key] || '').trim(); if (!value) continue;
      const row = el('section',null,'wt-builder-field');
      row.append(el('small',label),el('div',value)); summary.append(row);
    }
    for (const [key,label] of [['tools','Herramientas'],['channels','Canales']]) {
      if (!Array.isArray(p[key]) || !p[key].length) continue;
      const row = el('section',null,'wt-builder-field'); row.append(el('small',label),el('div',p[key].join(' · ')));summary.append(row);
    }
    if (!summary.children.length) summary.append(el('p','Todavía no hay detalles suficientes. Puedes seguir conversando y preparar la propuesta después.'));
  }
  function togglePreview(open) {
    if (!dialog) return;
    const enabled = typeof open === 'boolean' ? open : !dialog.classList.contains('wt-builder-show-preview');
    dialog.classList.toggle('wt-builder-show-preview',enabled);
    const toggle = byId('wtBuilderPreviewToggle');
    if (toggle) {toggle.textContent = enabled ? 'Volver al chat' : 'Ver configuración'; toggle.setAttribute('aria-expanded',String(enabled));}
  }
  function historyForRequest() {
    const selected = state.messages.slice(-16);
    while (selected.length > 1 && JSON.stringify(selected).length > 8200) selected.shift();
    return selected;
  }
  async function requestAI(mode) {
    const response = await fetch('/api/agents/builder-a1', {
      method:'POST', credentials:'same-origin', headers:{'Content-Type':'application/json'},
      body:JSON.stringify({mode, messages:historyForRequest(), proposal:mode==='proposal' ? state.proposal : undefined})
    });
    const json = await response.json().catch(() => ({}));
    if (response.status === 401) throw new Error('Tu sesión expiró. Vuelve a iniciar sesión.');
    if (response.status === 403) throw new Error('No tienes permiso para configurar agentes.');
    if (!response.ok) throw new Error(json.error || json.message || 'No se pudo conectar con el asistente.');
    return json;
  }
  async function sendMessage(value) {
    if (state.busy) return;
    value = String(value || '').trim();
    if (!value || value.length > 1200) {status('Escribe un mensaje de hasta 1.200 caracteres.');return;}
    const input=byId('wtBuilderInput');
    // Leave the textarea unlocked so the user can compose while waiting.
    state.messages.push({role:'user',content:value});
    messageBubble('user',value);
    if (input) {input.value='';input.style.height='auto';}
    state.busy=true;updateComposer(); status('El asistente está respondiendo…');
    const typing = typingBubble();
    try {
      const result = await requestAI('chat');
      if (!result || typeof result.reply !== 'string' || !result.reply.trim()) throw new Error('El asistente no devolvió una respuesta.');
      typing?.remove();
      const reply=result.reply.trim();
      state.messages.push({role:'assistant',content:reply});
      messageBubble('assistant',reply);
      status('Puedes seguir conversando libremente.');
    } catch (error) {
      typing?.remove();
      state.messages.pop(); // Failed turn was not accepted into the conversation history.
      messageBubble('assistant',error.message || 'No se pudo responder. Inténtalo de nuevo.','wt-builder-error');
      if (input && !input.value.trim()) input.value=value;
      status('No se ha perdido tu mensaje. Puedes reintentar.');
    } finally {
      state.busy=false;updateComposer();renderProposal();
      if (dialog?.open) input?.focus();
    }
  }
  async function prepareProposal() {
    if (state.busy || !state.messages.some(m=>m.role==='user')) return false;
    if (state.proposal && state.proposalAt===state.messages.length) return true;
    state.busy=true;updateComposer();status('Preparando los campos del formulario…');
    const spinner=typingBubble();
    try {
      const result = await requestAI('proposal');
      if (!result || !result.agent || typeof result.agent !== 'object') throw new Error('No llegó la configuración del agente.');
      state.proposal=result.agent;
      state.proposalAt=state.messages.length;
      renderProposal();status('Propuesta lista. Puedes revisarla o pasarla al formulario.');
      return true;
    } catch(error) {
      status(error.message || 'No se pudo preparar el formulario.');
      return false;
    } finally {spinner?.remove();state.busy=false;updateComposer();}
  }
  function assignValue(id, value) {
    if (typeof value !== 'string' || !value.trim()) return;
    const field=byId(id); if (!field) return;
    field.value=value;
    field.dispatchEvent(new Event('input',{bubbles:true}));
    field.dispatchEvent(new Event('change',{bubbles:true}));
  }
  function populateCheckboxes(group, values, allowed) {
    if (!Array.isArray(values) || !values.length) return;
    for (const value of allowed) {
      const checkbox = document.querySelector(`[data-wua58-${group}="${value}"]`);
      if (checkbox) {checkbox.checked=values.includes(value);checkbox.dispatchEvent(new Event('change',{bubbles:true}));}
    }
  }
  async function useInForm() {
    if (state.busy || !state.messages.some(m=>m.role==='user')) return;
    if (!(await prepareProposal())) return;
    const p=state.proposal; if (!p) return;
    const btn=byId('newAIAgent');
    if (!btn || btn.hidden || btn.disabled || !btn.getClientRects().length) {status('El formulario no está disponible para este usuario.');return;}
    dialog.close();
    btn.click(); // Reuse the existing new-agent form; do not create/save/activate.
    if (!byId('maName') || !byId('saveAIAgent')) {
      dialog.showModal();status('No se encontró el editor original. No se guardó ningún agente.');return;
    }
    assignValue('maName',p.name);
    if (TYPES.includes(p.type)) assignValue('maType',p.type);
    assignValue('maObjective',p.objective);
    assignValue('maTone',p.tone);
    assignValue('maLanguage',p.language);
    assignValue('maInstructions',p.instructions);
    assignValue('maKnowledge',p.knowledge);
    assignValue('maGreeting',p.greeting);
    assignValue('maHandoff',p.handoff_rules);
    populateCheckboxes('tool',p.tools,TOOLS);
    populateCheckboxes('channel',p.channels,CHANNELS);
    // DO NOT change maStatus, maDefault, maBudget or click saveAIAgent.
    const feedback=byId('wua58Feedback');
    if (feedback) {feedback.textContent='Configuración cargada. Revisa cada campo y decide cuándo pulsar «Crear agente». No se ha guardado nada.';feedback.hidden=false;}
    byId('maName').focus();
  }
  function startNewChat() {
    if (state.busy) return;
    state.messages=[];state.proposal=null;state.proposalAt=-1;
    byId('wtBuilderMessages')?.replaceChildren();
    messageBubble('assistant','¡Hola! 👋 Soy tu asistente para diseñar agentes IA. Puedes preguntarme lo que quieras y contarme tus ideas. Cuando te guste el resultado, lo pasamos al formulario sin crear nada automáticamente.');
    renderProposal();updateComposer();status('Conversa con naturalidad. No tienes que llenar un cuestionario.');
    togglePreview(false);
  }
  function openChat() {
    if (!dialog) return;
    if (!state.initialised) {startNewChat();state.initialised=true;}
    dialog.showModal();byId('wtBuilderInput')?.focus();scrollDown();
  }
  function setup() {
    const anchor=byId('newAIAgent');if(!anchor || byId('wtBuilderOpen'))return;
    const button=el('button','✦ Crear con ayuda de IA','wt-builder-launch');
    button.type='button';button.id='wtBuilderOpen';button.title='Conversar con IA y completar el formulario existente';
    anchor.insertAdjacentElement('beforebegin',button);
    const syncVisibility=()=>{button.hidden=!!anchor.hidden||anchor.disabled;};syncVisibility();
    new MutationObserver(syncVisibility).observe(anchor,{attributes:true,attributeFilter:['hidden','disabled','style']});
    dialog=document.createElement('dialog');dialog.id='wtAgentBuilderA1';dialog.className='wt-agent-builder';
    dialog.innerHTML=`
      <div class="wt-builder-shell">
        <header class="wt-builder-header">
          <div class="wt-builder-brand"><span class="wt-builder-mark">✦</span><div><small>WORKTIC AI</small><h2>Asistente para crear agentes</h2></div></div>
          <div class="wt-builder-header-actions"><button type="button" id="wtBuilderNew" class="wt-builder-header-soft">+ Nuevo chat</button><button type="button" id="wtBuilderPreviewToggle" class="wt-builder-header-soft" aria-expanded="false">Ver configuración</button><button type="button" id="wtBuilderClose" class="wt-builder-close" aria-label="Cerrar">×</button></div>
        </header>
        <div class="wt-builder-layout">
          <section class="wt-builder-chat" aria-label="Chat de asistente">
            <div class="wt-builder-conversation-top"><span class="wt-builder-online"></span> Asistente de Worktic AI <span class="wt-builder-mode">· Conversación libre</span></div>
            <div id="wtBuilderMessages" class="wt-builder-messages" aria-live="polite" aria-label="Conversación"></div>
            <div class="wt-builder-starters" id="wtBuilderStarters"><button type="button" data-starter="Quiero crear un agente de ventas para mi negocio. ¿Cómo empezamos?">Agente de ventas</button><button type="button" data-starter="¿Qué puede hacer un agente de atención al cliente en Worktic AI?">Atención al cliente</button><button type="button" data-starter="Ayúdame a diseñar un agente para agendar citas.">Agenda de citas</button></div>
            <div class="wt-builder-compose"><label class="wt-builder-sr-only" for="wtBuilderInput">Escribe un mensaje</label><textarea id="wtBuilderInput" rows="2" maxlength="1200" placeholder="Pregúntame lo que quieras sobre tu agente…"></textarea><div class="wt-builder-compose-bottom"><span id="wtBuilderStatus" role="status">Conversa libremente.</span><button type="button" id="wtBuilderSend" aria-label="Enviar mensaje" title="Enviar mensaje">➤</button></div></div>
            <div class="wt-builder-chat-note">La IA te ayuda a diseñarlo. Nada se crea ni publica hasta que tú uses el formulario.</div>
          </section>
          <aside class="wt-builder-preview" aria-label="Configuración del agente">
            <div class="wt-builder-preview-head"><div><small>TU AGENTE</small><h3>Configuración sugerida</h3></div><button id="wtBuilderHidePreview" type="button" aria-label="Volver al chat">×</button></div>
            <div id="wtBuilderSummary" class="wt-builder-summary"></div>
            <div class="wt-builder-preview-footer"><button id="wtBuilderPrepare" type="button" class="wt-builder-secondary">Actualizar propuesta</button><button id="wtBuilderUse" type="button" class="wt-builder-primary">Usar en el formulario →</button><p>Podrás editarla antes de pulsar «Crear agente».</p></div>
          </aside>
        </div>
        <footer class="wt-builder-footer"><span>✦ Tu conversación se utiliza para preparar el formulario; no se guarda ningún agente.</span><button type="button" id="wtBuilderBottomUse">Usar en el formulario →</button></footer>
      </div>`;
    document.body.append(dialog);
    button.addEventListener('click',openChat);
    byId('wtBuilderClose').addEventListener('click',()=>dialog.close());
    byId('wtBuilderNew').addEventListener('click',()=>{if(state.messages.length>0 && !window.confirm('¿Comenzar una conversación nueva? Se perderá la conversación actual.'))return;startNewChat();});
    byId('wtBuilderPreviewToggle').addEventListener('click',async()=>{
      if (dialog.classList.contains('wt-builder-show-preview')) {togglePreview(false);return;}
      if (state.messages.some(m=>m.role==='user') && (!state.proposal || state.proposalAt!==state.messages.length)) {
        if (!(await prepareProposal())) return;
      }
      togglePreview(true);
    });
    byId('wtBuilderHidePreview').addEventListener('click',()=>togglePreview(false));
    byId('wtBuilderPrepare').addEventListener('click',async()=>{if(await prepareProposal())togglePreview(true);});
    byId('wtBuilderUse').addEventListener('click',useInForm);
    byId('wtBuilderBottomUse').addEventListener('click',useInForm);
    const input=byId('wtBuilderInput');
    input.addEventListener('input',()=>{updateComposer();input.style.height='auto';input.style.height=Math.min(155,input.scrollHeight)+'px';});
    input.addEventListener('keydown',event=>{if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();sendMessage(input.value);}});
    byId('wtBuilderSend').addEventListener('click',()=>sendMessage(input.value));
    byId('wtBuilderStarters').addEventListener('click',event=>{const starter=event.target.closest('[data-starter]');if(starter)sendMessage(starter.dataset.starter);});
    dialog.addEventListener('close',()=>{togglePreview(false);});
    renderProposal();updateComposer();
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup,{once:true}); else setup();
})();
