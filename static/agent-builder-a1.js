/* Worktic AI - Agent Builder A4. Streaming + browser microphone permission + voice + existing form. No save/publish. */
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
  const voice = { active:false, recognition:null, listening:false, speaking:false, requesting:false, generation:0 };
  const SpeechRecognizer = window.SpeechRecognition || window.webkitSpeechRecognition;
  const canListen = !!SpeechRecognizer;
  const canSpeak = !!(window.speechSynthesis && window.SpeechSynthesisUtterance);
  const icons = {
    mic: '<svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="2" width="6" height="12" rx="3"/><path d="M5 10a7 7 0 0 0 14 0M12 17v5M8 22h8"/></svg>',
    send: '<svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m5 12 14-8-4 16-3-7-7-1Z"/><path d="m12 13 7-9"/></svg>'
  };

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
    const mic=byId('wtBuilderMic'); if (mic) mic.disabled = !canListen || voice.requesting;
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
  // Decode incremental NDJSON from the existing authenticated POST endpoint.
  async function requestStream(onDelta) {
    const response = await fetch('/api/agents/builder-a1', {
      method:'POST', credentials:'same-origin', headers:{'Content-Type':'application/json'},
      body:JSON.stringify({mode:'stream',messages:historyForRequest()})
    });
    if (!response.ok) {
      const json=await response.json().catch(()=>({}));
      if (response.status===401) throw new Error('Tu sesión expiró. Vuelve a iniciar sesión.');
      if (response.status===403) throw new Error('No tienes permiso para configurar agentes.');
      throw new Error(json.error||json.message||'No se pudo iniciar la respuesta.');
    }
    if (!response.body || !response.body.getReader) throw new Error('Tu navegador no permite recibir respuestas progresivas.');
    const reader=response.body.getReader(), decoder=new TextDecoder();
    let buffer='', text='', completed=false;
    const parseLine=line=>{
      if (!line.trim()) return;
      let item;
      try { item=JSON.parse(line); } catch { throw new Error('La respuesta del servidor no tiene un formato válido.'); }
      if (item.type==='delta') {
        text += String(item.value||'');
        if (text.length>12000) throw new Error('La respuesta superó el límite permitido.');
        onDelta(text);
      } else if (item.type==='done') completed=true;
      else if (item.type==='error') throw new Error(String(item.value||'No se pudo completar la respuesta.'));
    };
    try {
      while(true){
        const {done,value}=await reader.read();
        buffer+=done?decoder.decode():decoder.decode(value,{stream:true});
        let lineEnd;
        while((lineEnd=buffer.indexOf('\n'))!==-1){parseLine(buffer.slice(0,lineEnd));buffer=buffer.slice(lineEnd+1);}
        if(done) break;
      }
      if (buffer.trim()) parseLine(buffer);
      if (!completed || !text.trim()) throw new Error('La respuesta se interrumpió. Intenta nuevamente.');
      return text.trim();
    } finally {try {reader.releaseLock();} catch { /* closed stream */ }}
  }

  function showMicNotice(kind) {
    const notice=byId('wtBuilderMicNotice');
    const message=byId('wtBuilderMicNoticeText');
    if (!notice || !message) return;
    const help={
      denied:'Chrome bloqueó el micrófono. Para habilitarlo: pulsa el icono de controles del sitio junto a la dirección web → Configuración del sitio → Micrófono → Permitir. Después actualiza la página e inténtalo de nuevo. Si bloqueaste el permiso antes, Chrome no mostrará otra ventana hasta que cambies ese ajuste.',
      missing:'No se detectó un micrófono. Comprueba que esté conectado y seleccionado en la configuración de sonido del dispositivo y vuelve a intentarlo.',
      unsupported:'No está disponible el acceso al micrófono en este navegador. Utiliza Chrome actualizado con HTTPS o continúa escribiendo.',
      service:'El acceso al dispositivo está habilitado, pero el servicio de reconocimiento de voz no respondió. Comprueba la conexión y las restricciones de Chrome; puedes continuar por texto.'
    };
    message.textContent=help[kind] || help.service;
    notice.hidden=false;
  }
  function hideMicNotice(){const notice=byId('wtBuilderMicNotice');if(notice)notice.hidden=true;}
  function updateVoiceUI() {
    const mic=byId('wtBuilderMic'); if (!mic) return;
    mic.classList.toggle('wt-builder-mic-active',voice.active);
    mic.classList.toggle('wt-builder-mic-listening',voice.listening);
    mic.classList.toggle('wt-builder-mic-requesting',voice.requesting);
    mic.setAttribute('aria-pressed',String(voice.active));
    mic.setAttribute('aria-label',voice.active?'Detener conversación por voz':voice.requesting?'Solicitando acceso al micrófono':'Activar micrófono y conversación por voz');
    mic.title=canListen?(voice.active?'Detener conversación por voz':voice.requesting?'Solicitando permiso de Chrome':'Activar voz y solicitar permiso de micrófono'):'Tu navegador no admite reconocimiento de voz';
    mic.disabled=!canListen || voice.requesting;
  }
  function stopVoice(message) {
    voice.generation++;
    voice.active=false;
    voice.requesting=false;
    voice.listening=false;
    voice.speaking=false;
    const recognizer=voice.recognition;
    voice.recognition=null;
    if(recognizer){try{recognizer.abort();}catch{/* already stopped */}}
    if(canSpeak){try{window.speechSynthesis.cancel();}catch{/* unavailable */}}
    updateVoiceUI();
    if(message) status(message);
  }
  function voiceParts(text){
    const sentences=String(text).replace(/\s+/g,' ').match(/[^.!?]+[.!?]*|[^.!?]+$/g)||[];
    const parts=[];
    for(const sentence of sentences){
      const words=sentence.trim().split(/\s+/);let part='';
      for(const word of words){
        if(part.length+word.length+1>180&&part){parts.push(part);part='';}
        part+=(part?' ':'')+word;
      }
      if(part)parts.push(part);
    }
    return parts;
  }
  function speakReply(text) {
    if(!voice.active)return;
    if(!canSpeak){stopVoice('Tu navegador no permite respuesta hablada; puedes seguir escribiendo.');return;}
    const parts=voiceParts(text), generation=voice.generation;
    if(!parts.length){beginListening();return;}
    voice.speaking=true;
    status('El asistente está hablando…');
    const synth=window.speechSynthesis;
    const available=synth.getVoices?synth.getVoices():[];
    const selected=available.find(v=>v.lang.toLowerCase()==='es-co')||available.find(v=>v.lang.toLowerCase().startsWith('es-'));
    let i=0;
    const sayNext=()=>{
      if(!voice.active||generation!==voice.generation||!dialog?.open)return;
      if(i>=parts.length){voice.speaking=false;beginListening();return;}
      const utterance=new SpeechSynthesisUtterance(parts[i++]);
      utterance.lang=selected?.lang||'es-CO';
      if(selected)utterance.voice=selected;
      utterance.rate=1.03;
      utterance.onend=sayNext;
      utterance.onerror=()=>{voice.speaking=false;stopVoice('No se pudo reproducir el audio. Puedes continuar por texto.');};
      synth.speak(utterance);
    };
    try{synth.cancel();sayNext();}catch{stopVoice('El audio no está disponible. Puedes continuar por texto.');}
  }
  function beginListening() {
    if(!voice.active||!canListen||state.busy||!dialog?.open||document.hidden)return;
    const generation=voice.generation;
    const recognizer=new SpeechRecognizer();
    voice.recognition=recognizer;
    recognizer.lang='es-CO';
    recognizer.interimResults=true;
    recognizer.continuous=false;
    recognizer.maxAlternatives=1;
    const input=byId('wtBuilderInput');
    const base=(input?.value||'').trim();
    let final='', errorReceived=false;
    recognizer.onstart=()=>{if(generation!==voice.generation)return;voice.listening=true;hideMicNotice();updateVoiceUI();status('Escuchando… Habla naturalmente.');};
    recognizer.onresult=event=>{
      if(generation!==voice.generation)return;
      let interim='';
      for(let i=event.resultIndex;i<event.results.length;i++){
        const result=event.results[i];const words=String(result[0]?.transcript||'').trim();
        if(result.isFinal) final+=(final?' ':'')+words;
        else interim+=(interim?' ':'')+words;
      }
      if(input){input.value=[base,final,interim].filter(Boolean).join(' ').slice(0,1200);updateComposer();}
    };
    recognizer.onerror=event=>{
      errorReceived=true;
      const messages={'not-allowed':'Reconocimiento bloqueado. Revisa el permiso del micrófono en Chrome.','service-not-allowed':'El servicio de reconocimiento está bloqueado.','audio-capture':'No se encontró un micrófono disponible.','network':'El reconocimiento de voz no está disponible en este momento.','no-speech':'No se detectó voz. Pulsa el micrófono para intentarlo de nuevo.'};
      if(event.error!=='aborted'&&generation===voice.generation){
        stopVoice(messages[event.error]||'No se pudo reconocer la voz. Puedes escribir tu mensaje.');
        if(event.error==='not-allowed')showMicNotice('denied');
        else if(event.error==='service-not-allowed'||event.error==='network')showMicNotice('service');
        else if(event.error==='audio-capture')showMicNotice('missing');
      }
    };
    recognizer.onend=()=>{
      if(generation!==voice.generation)return;
      voice.listening=false;voice.recognition=null;updateVoiceUI();
      if(!voice.active||errorReceived)return;
      const content=(input?.value||'').trim();
      if(final.trim()&&content){sendMessage(content,{voiceTurn:true});return;}
      stopVoice('No detecté una frase completa. Pulsa el micrófono cuando quieras hablar.');
    };
    try{recognizer.start();}catch{stopVoice('No fue posible activar el micrófono.');}
  }
  async function toggleVoice() {
    if(voice.active){stopVoice('Voz desactivada. Puedes seguir por texto.');return;}
    if(voice.requesting)return;
    if(!canListen){showMicNotice('unsupported');status('El reconocimiento de voz no está disponible en este navegador.');return;}
    if(!canSpeak){status('Tu navegador no permite reproducir la respuesta por voz.');return;}
    if(state.busy){status('Espera a que termine la respuesta actual para activar el micrófono.');return;}
    // A genuine browser permission request, invoked directly from the mic click.
    // SpeechRecognition.start() alone does not reliably prompt on every Chrome setup.
    if(!navigator.mediaDevices || typeof navigator.mediaDevices.getUserMedia!=='function'){
      showMicNotice('unsupported');
      status('No se puede solicitar el micrófono. Comprueba HTTPS y la compatibilidad del navegador.');
      return;
    }
    hideMicNotice();
    const generation=++voice.generation;
    voice.requesting=true;
    updateVoiceUI();
    status('Chrome está solicitando acceso al micrófono…');
    let stream;
    try{
      stream=await navigator.mediaDevices.getUserMedia({audio:{echoCancellation:true,noiseSuppression:true}});
      // SpeechRecognition will acquire its own input. Do not leave a second capture open.
      if(stream && typeof stream.getTracks==='function'){
        for(const track of stream.getTracks()){try{track.stop();}catch{/* already closed */}}
      }
      stream=null;
      if(generation!==voice.generation || !dialog?.open)return;
      voice.requesting=false;
      voice.active=true;
      updateVoiceUI();
      status('Micrófono autorizado. Escuchando…');
      beginListening();
    }catch(error){
      if(stream && typeof stream.getTracks==='function'){
        for(const track of stream.getTracks()){try{track.stop();}catch{/* cleanup */}}
      }
      if(generation!==voice.generation)return;
      voice.requesting=false;
      voice.active=false;
      updateVoiceUI();
      const name=String(error?.name||'');
      if(['NotAllowedError','PermissionDeniedError','SecurityError'].includes(name)){
        showMicNotice('denied');
        status('Micrófono bloqueado. Consulta los pasos para permitirlo en Chrome.');
      }else if(['NotFoundError','DevicesNotFoundError','NotReadableError','TrackStartError'].includes(name)){
        showMicNotice('missing');
        status('No se pudo acceder al dispositivo de audio.');
      }else{
        showMicNotice('service');
        status('No se pudo activar el micrófono. Puedes continuar escribiendo.');
      }
    }
  }

  async function sendMessage(value) {
    if(state.busy)return;
    value=String(value||'').trim();
    if(!value||value.length>1200){status('Escribe un mensaje de hasta 1.200 caracteres.');return;}
    const input=byId('wtBuilderInput');
    if(voice.recognition){try{voice.recognition.abort();}catch{/* stopped */}voice.recognition=null;voice.listening=false;}
    state.messages.push({role:'user',content:value});
    messageBubble('user',value);
    if(input){input.value='';input.style.height='auto';}
    state.busy=true;updateComposer();updateVoiceUI();status('El asistente está respondiendo…');
    const typing=typingBubble();const bubble=typing?.querySelector('.wt-builder-bubble');
    let received=false;
    try{
      const reply=await requestStream(text=>{
        if(!received){received=true;typing?.classList.remove('wt-builder-typing');}
        if(bubble)bubble.textContent=text;
        scrollDown();
      });
      state.messages.push({role:'assistant',content:reply});
      status('Puedes seguir conversando libremente.');
      if(voice.active)speakReply(reply);
    }catch(error){
      typing?.remove();
      state.messages.pop();
      messageBubble('assistant',error.message||'No pude responder. Intenta otra vez.','wt-builder-error');
      if(input&&!input.value.trim())input.value=value;
      if(voice.active)stopVoice('Ocurrió un error. Puedes reintentar por texto o voz.');
      else status('No se ha perdido tu mensaje. Puedes reintentar.');
    }finally{
      state.busy=false;updateComposer();renderProposal();
      if(dialog?.open&&!voice.active)input?.focus();
    }
  }
  async function prepareProposal() {
    if(voice.active)stopVoice('Preparando la configuración del formulario.');
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
    if(voice.active)stopVoice();
    if (state.busy || !state.messages.some(m=>m.role==='user')) return;
    if (!(await prepareProposal())) return;
    const p=state.proposal; if (!p) return;
    const btn=byId('newAIAgent');
    if (!btn || btn.hidden || btn.disabled || !btn.getClientRects().length) {status('El formulario no está disponible para este usuario.');return;}
    dialog.close();
    // When opened from Nuevo agente, fill THAT form instead of reopening and resetting it.
    if(!byId('maName') || !byId('saveAIAgent') || !byId('modal')?.open){btn.click();}
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
    stopVoice();hideMicNotice();
    state.messages=[];state.proposal=null;state.proposalAt=-1;
    byId('wtBuilderMessages')?.replaceChildren();
    messageBubble('assistant','¡Hola! 👋 Soy tu asistente para diseñar agentes IA. Puedes preguntarme lo que quieras y contarme tus ideas. Cuando te guste el resultado, lo pasamos al formulario sin crear nada automáticamente.');
    renderProposal();updateComposer();updateVoiceUI();status('Conversa con naturalidad. No tienes que llenar un cuestionario.');
    togglePreview(false);
  }
  function openChat() {
    if (!dialog) return;
    if (!state.initialised) {startNewChat();state.initialised=true;}
    dialog.showModal();byId('wtBuilderInput')?.focus();scrollDown();
  }
  function injectIntoNewAgentForm() {
    const origin=byId('modal'),form=origin?.querySelector('.wua58-agent');
    const submit=form?.querySelector('#saveAIAgent');
    if(!origin?.open||!form||!submit||submit.textContent.trim()!=='Crear agente'||byId('wtBuilderOpen'))return;
    const heading=form.querySelector('.modal-heading');if(!heading)return;
    const entry=el('div',null,'wt-builder-form-entry');entry.id='wtBuilderFormEntry';
    entry.innerHTML=`<span class="wt-builder-form-sparkle" aria-hidden="true">✦</span><div class="wt-builder-form-copy"><strong>¿Prefieres diseñar tu agente conversando?</strong><span>La IA te guía y completa este formulario. Tú decides cuándo crear el agente.</span></div><button type="button" id="wtBuilderOpen" class="wt-builder-launch">Crear con IA <span aria-hidden="true">→</span></button>`;
    heading.insertAdjacentElement('afterend',entry);
    byId('wtBuilderOpen')?.addEventListener('click',openChat);
  }
  function setup() {
    const anchor=byId('newAIAgent');if(!anchor||byId('wtAgentBuilderA1'))return;
    const legacy=byId('wtBuilderOpen');if(legacy)legacy.remove();
    const modalContent=byId('modalContent');
    if(modalContent){new MutationObserver(injectIntoNewAgentForm).observe(modalContent,{childList:true,subtree:true});}
    anchor.addEventListener('click',()=>queueMicrotask(injectIntoNewAgentForm));
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
            <div class="wt-builder-starters-area"><span>IDEAS PARA EMPEZAR · Puedes crear otros agentes libremente</span><div class="wt-builder-starters" id="wtBuilderStarters">
              <button type="button" data-starter="Quiero diseñar un agente de ventas para mi negocio. Ayúdame a definirlo.">Ventas</button>
              <button type="button" data-starter="Quiero diseñar un agente de atención y soporte al cliente.">Atención al cliente</button>
              <button type="button" data-starter="Quiero un agente que ayude a gestionar y confirmar citas.">Agenda de citas</button>
              <button type="button" data-starter="Ayúdame a diseñar un agente de campañas de marketing.">Marketing</button>
              <button type="button" data-starter="Quiero un agente para moderar comunidades y responder preguntas frecuentes.">Comunidades</button>
              <button type="button" data-starter="Quiero un asistente para ayudar a calificar leads y registrar información en CRM, dentro de las funciones que tenga disponibles.">CRM y leads</button>
              <button type="button" data-starter="Necesito un agente de soporte técnico que atienda consultas y sepa cuándo escalar a un humano.">Soporte técnico</button>
              <button type="button" data-starter="Quiero un agente personalizado con tareas particulares. Ayúdame a analizar lo que puede hacer.">Personalizado</button>
            </div></div>
            <div class="wt-builder-compose"><label class="wt-builder-sr-only" for="wtBuilderInput">Escribe un mensaje</label><textarea id="wtBuilderInput" rows="2" maxlength="1200" placeholder="Pregúntame lo que quieras sobre tu agente…"></textarea><div class="wt-builder-compose-bottom"><span id="wtBuilderStatus" role="status">Conversa libremente.</span><div class="wt-builder-composer-actions"><button type="button" id="wtBuilderMic" aria-label="Conversar por voz" aria-pressed="false" title="Conversar por voz">${icons.mic}</button><button type="button" id="wtBuilderSend" aria-label="Enviar mensaje" title="Enviar mensaje">${icons.send}</button></div></div></div>
            <div class="wt-builder-mic-notice" id="wtBuilderMicNotice" role="alert" hidden><span aria-hidden="true">🎙️</span><p id="wtBuilderMicNoticeText"></p><button type="button" id="wtBuilderMicNoticeClose" aria-label="Cerrar indicaciones del micrófono">×</button></div>
            <div class="wt-builder-chat-note">El micrófono requiere tu permiso. El reconocimiento puede utilizar servicios del navegador. Nada se crea ni publica desde el chat.</div>
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
    byId('wtBuilderClose').addEventListener('click',()=>dialog.close());
    byId('wtBuilderMic').addEventListener('click',toggleVoice);
    byId('wtBuilderMicNoticeClose').addEventListener('click',hideMicNotice);
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
    dialog.addEventListener('close',()=>{stopVoice();togglePreview(false);});
    document.addEventListener('visibilitychange',()=>{if(document.hidden&&voice.active)stopVoice('Voz detenida porque la ventana no está visible.');});
    renderProposal();updateComposer();updateVoiceUI();
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup,{once:true}); else setup();
})();
