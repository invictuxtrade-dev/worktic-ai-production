/* Worktic AI - Agent Builder A1. Conversational assistant; NEVER saves/activates an agent. */
(() => {
  'use strict';
  const byId = id => document.getElementById(id);
  const allowedTypes = ['general','sales','support','appointments','campaigns','community','custom'];
  const allowedTools = ['contact','opportunity','appointment','catalog','handoff'];
  const allowedChannels = ['whatsapp','telegram','messenger','webchat','voice'];
  const state = { history: [], proposal: null, busy: false };
  let dialog = null;

  function text(tag, content, cls) {
    const el = document.createElement(tag);
    if (cls) el.className = cls;
    el.textContent = content;
    return el;
  }
  function addMessage(role, content) {
    const feed = byId('wtBuilderMessages');
    if (!feed) return;
    const item = text('div',content,'wt-builder-bubble ' + (role==='user'?'wt-builder-user':'wt-builder-ai'));
    feed.append(item);
    feed.scrollTop = feed.scrollHeight;
  }
  function renderProposal() {
    const summary = byId('wtBuilderSummary');
    const use = byId('wtBuilderUse');
    if (!summary || !use) return;
    summary.replaceChildren();
    const p=state.proposal;
    use.disabled = !p || !String(p.name||'').trim() || !String(p.objective||'').trim() || state.busy;
    if (!p) {summary.append(text('p','Conversemos sobre tu negocio. Aquí aparecerá una propuesta editable.')); return;}
    for (const [key,label] of [['name','Nombre'],['type','Especialidad'],['objective','Objetivo'],['tone','Tono'],['language','Idioma'],['instructions','Instrucciones'],['knowledge','Conocimiento'],['greeting','Bienvenida'],['handoff_rules','Transferencia']]) {
      const value = String(p[key]||'').trim();
      if (!value) continue;
      const row = document.createElement('div'); row.className='wt-builder-field';
      row.append(text('strong',label), text('span',value)); summary.append(row);
    }
    for (const [key,label] of [['tools','Herramientas'],['channels','Canales']]) {
      if (!Array.isArray(p[key]) || !p[key].length) continue;
      const row=document.createElement('div'); row.className='wt-builder-field';
      row.append(text('strong',label),text('span',p[key].join(', ')));summary.append(row);
    }
  }
  async function sendMessage(message) {
    const input=byId('wtBuilderInput');
    const send=byId('wtBuilderSend');
    const status=byId('wtBuilderStatus');
    if (state.busy) return;
    const value=String(message||'').trim();
    if (!value || value.length>1200) {status.textContent='Escribe un mensaje de hasta 1.200 caracteres.'; return;}
    state.busy=true; input.disabled=true; send.disabled=true;
    const pending={role:'user',content:value};
    addMessage('user',value); input.value='';status.textContent='Preparando una propuesta…';renderProposal();
    try {
      const history=state.history.concat(pending).slice(-14);
      const response=await fetch('/api/agents/builder-a1',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({messages:history,proposal:state.proposal})});
      const result=await response.json().catch(()=>({}));
      if(response.status===401) throw new Error('Tu sesión expiró; vuelve a iniciar sesión.');
      if(!response.ok) throw new Error(result.error||'No se pudo generar la propuesta.');
      if(typeof result.reply!=='string'||!result.agent||typeof result.agent!=='object')throw new Error('La respuesta del asistente no fue válida.');
      state.history=history.concat({role:'assistant',content:result.reply}).slice(-14);
      state.proposal=result.agent;
      addMessage('assistant',result.reply);
      status.textContent='Puedes seguir conversando o usar la propuesta en el formulario.';
    } catch(error){
      addMessage('assistant','No pude procesar ese mensaje. Puedes intentar nuevamente.');
      status.textContent=error.message||'Error temporal.';
      input.value=value;
    } finally {state.busy=false; input.disabled=false;send.disabled=false;renderProposal();input.focus();}
  }
  function assignValue(id, value) {
    if (typeof value !== 'string' || !value.trim()) return;
    const el=byId(id);
    if (!el) return;
    el.value=value;
    el.dispatchEvent(new Event('input',{bubbles:true}));
    el.dispatchEvent(new Event('change',{bubbles:true}));
  }
  function populateCheckboxes(group, values, allowed) {
    if (!Array.isArray(values) || !values.length) return;
    // Values not proposed remain in their default form state.
    for (const value of allowed) {
      const checkbox = document.querySelector(`[data-wua58-${group}="${value}"]`);
      if (checkbox) {
        checkbox.checked=values.includes(value);
        checkbox.dispatchEvent(new Event('change',{bubbles:true}));
      }
    }
  }
  function useInForm() {
    const p=state.proposal;
    if(!p || !String(p.name||'').trim() || !String(p.objective||'').trim()) return;
    const newAgent=byId('newAIAgent');
    if(!newAgent || newAgent.hidden || !newAgent.getClientRects().length) {
      byId('wtBuilderStatus').textContent='No tienes permiso para abrir el formulario de agentes.';
      return;
    }
    dialog.close();
    newAgent.click(); // Existing editor, existing Create Agent button and validation.
    if (!byId('maName') || !byId('saveAIAgent')) {
      dialog.showModal();
      byId('wtBuilderStatus').textContent='No se encontró el formulario actual. No se guardó ningún agente.';
      return;
    }
    assignValue('maName',p.name);
    if (allowedTypes.includes(p.type)) assignValue('maType',p.type);
    assignValue('maObjective',p.objective);
    assignValue('maTone',p.tone);
    assignValue('maLanguage',p.language);
    assignValue('maInstructions',p.instructions);
    assignValue('maKnowledge',p.knowledge);
    assignValue('maGreeting',p.greeting);
    assignValue('maHandoff',p.handoff_rules);
    populateCheckboxes('tool',p.tools,allowedTools);
    populateCheckboxes('channel',p.channels,allowedChannels);
    // IMPORTANT: never touch maStatus, maDefault, maBudget or saveAIAgent.
    const feedback=byId('wua58Feedback');
    if (feedback) { feedback.textContent='Propuesta transferida. Revisa y modifica cualquier campo; el agente todavía NO se ha creado. El estado y la activación los decides tú.';feedback.hidden=false; }
    byId('maName').focus();
  }
  function openWizard(){
    if(!dialog)return;
    if(!state.history.length) {
      const feed=byId('wtBuilderMessages');feed.replaceChildren();
      addMessage('assistant','¡Hola! Te ayudaré a diseñar tu agente IA. Cuéntame a qué se dedica tu negocio y qué quieres que haga el agente: vender, responder preguntas, agendar citas o atender clientes.');
    }
    renderProposal();dialog.showModal();byId('wtBuilderInput').focus();
  }
  function setup(){
    const anchor=byId('newAIAgent');
    if(!anchor || byId('wtBuilderOpen'))return;
    const button=document.createElement('button');button.type='button';button.id='wtBuilderOpen';button.className='wt-builder-launch';button.textContent='✦ Crear con ayuda de IA';button.title='Conversar con IA y completar el formulario sin crear el agente';
    anchor.insertAdjacentElement('beforebegin',button);
    const syncVisibility=()=>{button.hidden=!!anchor.hidden || anchor.disabled;};
    syncVisibility();
    new MutationObserver(syncVisibility).observe(anchor,{attributes:true,attributeFilter:['hidden','disabled','style']});
    dialog=document.createElement('dialog');dialog.id='wtAgentBuilderA1';dialog.className='wt-agent-builder';
    dialog.innerHTML=`<div class="wt-builder-shell"><header class="wt-builder-header"><div><small>WORKTIC AI · ASISTENTE</small><h2>Diseña tu agente conversando</h2><p>Descríbenos tu negocio. Te ayudaremos a definir sus instrucciones y capacidades.</p></div><button type="button" id="wtBuilderClose" aria-label="Cerrar asistente">×</button></header><div class="wt-builder-layout"><section class="wt-builder-chat"><div id="wtBuilderMessages" class="wt-builder-messages" aria-live="polite" aria-label="Conversación"></div><div class="wt-builder-compose"><label for="wtBuilderInput">Tu mensaje</label><textarea id="wtBuilderInput" rows="3" maxlength="1200" placeholder="Tengo una tienda de tecnología. Quiero un agente que atienda clientes y les ayude a elegir productos…"></textarea><div class="wt-builder-compose-bottom"><small id="wtBuilderStatus" role="status">El asistente solo preparará los campos del formulario.</small><button type="button" id="wtBuilderSend">Enviar ↗</button></div></div></section><aside class="wt-builder-preview"><h3>Configuración sugerida</h3><div id="wtBuilderSummary" class="wt-builder-summary"></div><div class="wt-builder-preview-footer"><p>La IA no crea ni activa agentes. Tú decides el estado y pulsas «Crear agente» en el formulario habitual.</p><button type="button" id="wtBuilderUse">Usar en el formulario</button></div></aside></div></div>`;
    document.body.append(dialog);
    button.addEventListener('click',openWizard);
    byId('wtBuilderClose').addEventListener('click',()=>dialog.close());
    byId('wtBuilderSend').addEventListener('click',()=>sendMessage(byId('wtBuilderInput').value));
    byId('wtBuilderInput').addEventListener('keydown',event=>{if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();sendMessage(event.target.value);}});
    byId('wtBuilderUse').addEventListener('click',useInForm);
    renderProposal();
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup,{once:true});else setup();
})();
