/* Worktic AI - Agent Builder A5. Authenticated audio transcription, progressive text and existing form. No save/publish. */
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
  const MAX_MESSAGE_CHARS = 120000;
  const MAX_HISTORY_CHARS = 120000;
  const isSuperAdmin = () => String(window.currentUser?.role||'').toLowerCase()==='superadmin';
  const state = { messages: [], proposal: null, proposalAt: -1, busy: false, initialised: false };
  let dialog = null;
  const voice = { active:false, recorder:null, stream:null, recording:false, speaking:false, requesting:false, timeout:null, generation:0 };
  const canListen = !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia && window.MediaRecorder);
  const canSpeak = !!(window.speechSynthesis && window.SpeechSynthesisUtterance);
  const MAX_AUDIO_BYTES = 5 * 1024 * 1024;
  const MAX_RECORDING_MS = 30000;
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
    const counter=byId('wtBuilderCount'); if(counter&&input)counter.textContent=isSuperAdmin()?input.value.length.toLocaleString('es-CO')+' · sin límite':input.value.length.toLocaleString('es-CO')+' / 20.000';
    const mic=byId('wtBuilderMic'); if (mic) mic.disabled = typeof window.WorkticAgentBuilderLiveStart === 'function' ? false : (!canListen || voice.requesting);
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
    // Keep a long business brief as an anchor when the chat grows.
    // Preserve the latest question and a bounded amount of recent dialogue.
    const selected = state.messages.slice(-16);
    const brief = state.messages.find(m => m.role==='user' && String(m.content||'').length>4000);
    if (brief && !selected.includes(brief)) selected.unshift(brief);
    while (selected.length>16) selected.splice(selected[0]===brief ? 1 : 0,1);
    while (selected.length>1 && JSON.stringify(selected).length>MAX_HISTORY_CHARS) {
      const dropIndex=selected[0]===brief ? 1 : 0;
      if (dropIndex>=selected.length-1) break;
      selected.splice(dropIndex,1);
    }
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

  function showMicNotice(kind, detail='') {
    const notice=byId('wtBuilderMicNotice'), message=byId('wtBuilderMicNoticeText');
    if(!notice||!message)return;
    const help={
      denied:'Chrome o Windows denegó el acceso al dispositivo de audio. Si Chrome ya está en Permitir, comprueba en Windows → Privacidad y seguridad → Micrófono que el acceso esté habilitado para las aplicaciones de escritorio.',
      missing:'No se detectó un micrófono conectado o Windows no pudo abrirlo. Revisa el dispositivo de entrada en Configuración de sonido.',
      unsupported:'Este navegador no permite grabar audio. Utiliza Chrome actualizado en una conexión HTTPS o continúa escribiendo.',
      service:'El micrófono pudo grabar, pero no se completó la transcripción. Comprueba la conexión y la disponibilidad del servicio de IA; puedes continuar por texto.',
      short:'La grabación fue demasiado breve o no contiene audio suficiente. Habla unos segundos y vuelve a intentarlo.'
    };
    message.textContent=(help[kind]||help.service)+(detail?' '+String(detail).slice(0,120):'');
    notice.hidden=false;
  }
  function hideMicNotice(){const n=byId('wtBuilderMicNotice');if(n)n.hidden=true;}
  function releaseMicStream() {
    if(voice.stream){for(const t of voice.stream.getTracks()){try{t.stop();}catch{/* already closed */}}voice.stream=null;}
    if(voice.timeout){clearTimeout(voice.timeout);voice.timeout=null;}
  }
  function updateVoiceUI() {
    const mic=byId('wtBuilderMic'); if(!mic)return;
    const dictation=byId('wtBuilderDictate');
    if(dictation){dictation.textContent=voice.recording?'■ Enviar dictado':voice.requesting?'Procesando…':'Dictar';dictation.disabled=!canListen||voice.requesting;}
    // The A6 live microphone owns its own visual state; A5 remains a separate dictation action.
    if(typeof window.WorkticAgentBuilderLiveStart === 'function')return;
    mic.classList.toggle('wt-builder-mic-active',voice.active);
    mic.classList.toggle('wt-builder-mic-listening',voice.recording);
    mic.classList.toggle('wt-builder-mic-requesting',voice.requesting);
    mic.setAttribute('aria-pressed',String(voice.recording));
    mic.setAttribute('aria-label',voice.recording?'Detener y enviar grabación':voice.speaking?'Detener voz del asistente':voice.requesting?'Procesando audio':'Iniciar grabación de voz');
    mic.title=voice.recording?'Pulsa para terminar y enviar':'Grabar un mensaje por voz';
    mic.disabled=!canListen || voice.requesting;
  }
  function stopVoice(message) {
    voice.generation++;
    voice.active=false;voice.recording=false;voice.requesting=false;voice.speaking=false;
    const recorder=voice.recorder;voice.recorder=null;
    if(recorder){recorder.onstop=null;recorder.ondataavailable=null;recorder.onerror=null;try{if(recorder.state!=='inactive')recorder.stop();}catch{/* already stopped */}}
    releaseMicStream();
    if(canSpeak){try{window.speechSynthesis.cancel();}catch{/* unavailable */}}
    updateVoiceUI();
    if(message)status(message);
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
    if(!canSpeak){stopVoice('Respuesta lista en texto. La voz no está disponible en este navegador.');return;}
    const parts=voiceParts(text), generation=voice.generation;
    if(!parts.length){stopVoice('Pulsa el micrófono para hablar de nuevo.');return;}
    voice.speaking=true;updateVoiceUI();status('El asistente está hablando…');
    const synth=window.speechSynthesis;
    const available=synth.getVoices?synth.getVoices():[];
    // A5 dictation fallback uses the OS voices, not OpenAI Realtime.
    // Never silently choose a male default when a feminine voice is requested.
    const femaleName=/\b(female|woman|mujer|elena|lucia|lucía|helena|sofia|sofía|monica|mónica|laura|paulina|sabina|paloma|isabela|isabella|maria|maría|carmen)\b/i;
    const spanish=available.filter(v=>/^es([_-]|$)/i.test(v.lang||''));
    const selected=spanish.find(v=>/^es[-_]AR$/i.test(v.lang)&&femaleName.test(v.name||''))
      ||spanish.find(v=>femaleName.test(v.name||''));
    if(!selected){stopVoice('La voz femenina de dictado no está instalada en Chrome. Usa «Hablar» (voz de OpenAI) cuando la conexión Realtime funcione; puedes continuar por texto.');return;}
    let i=0;
    const sayNext=()=>{
      if(!voice.active||generation!==voice.generation||!dialog?.open)return;
      if(i>=parts.length){voice.speaking=false;voice.active=false;updateVoiceUI();status('Pulsa el micrófono para hablar de nuevo.');return;}
      const u=new SpeechSynthesisUtterance(parts[i++]);
      u.lang=selected.lang;u.voice=selected;u.rate=0.98;
      u.onend=sayNext;
      u.onerror=()=>stopVoice('No se pudo reproducir la voz. La respuesta está disponible en texto.');
      synth.speak(u);
    };
    try{synth.cancel();sayNext();}catch{stopVoice('El audio de respuesta no está disponible. Puedes leer el mensaje.');}
  }
  function preferredRecordingMime() {
    for(const mime of ['audio/webm;codecs=opus','audio/webm','audio/mp4','audio/ogg;codecs=opus']){
      if(!MediaRecorder.isTypeSupported||MediaRecorder.isTypeSupported(mime))return mime;
    }
    return '';
  }
  async function uploadVoiceAudio(blob,filename) {
    const data=new FormData();data.append('mode','transcribe');data.append('audio',blob,filename);
    const response=await fetch('/api/agents/builder-a1',{method:'POST',credentials:'same-origin',body:data});
    const result=await response.json().catch(()=>({}));
    if(response.status===401)throw new Error('Tu sesión expiró. Inicia sesión nuevamente.');
    if(response.status===403)throw new Error('La sesión no tiene permiso para transcribir audio.');
    if(!response.ok)throw new Error(String(result.error||result.message||'La transcripción no está disponible.').slice(0,140));
    const text=String(result.text||'').trim().slice(0,1200);
    if(!text)throw new Error('No se detectaron palabras en la grabación.');
    return text;
  }
  function finishRecording() {
    if(!voice.recording||!voice.recorder)return;
    voice.recording=false;
    voice.requesting=true;updateVoiceUI();status('Procesando tu grabación…');
    try{if(voice.recorder.state==='recording')voice.recorder.stop();}catch{stopVoice('No se pudo detener la grabación.');}
  }
  async function toggleVoice() {
    if(voice.recording){finishRecording();return;}
    if(voice.requesting)return;
    if(voice.speaking){stopVoice('Voz detenida. Puedes seguir por texto.');return;}
    if(!canListen){showMicNotice('unsupported');status('Este navegador no puede grabar audio.');return;}
    if(state.busy){status('Espera a que termine la respuesta actual.');return;}
    hideMicNotice();
    const generation=++voice.generation;
    voice.requesting=true;updateVoiceUI();status('Solicitando acceso al micrófono…');
    let stream;
    try{
      stream=await navigator.mediaDevices.getUserMedia({audio:{echoCancellation:true,noiseSuppression:true}});
      if(generation!==voice.generation||!dialog?.open){stream.getTracks().forEach(t=>t.stop());return;}
      voice.stream=stream;
      const mime=preferredRecordingMime();
      const recorder=mime?new MediaRecorder(stream,{mimeType:mime}):new MediaRecorder(stream);
      const chunks=[];
      voice.recorder=recorder;voice.active=true;
      recorder.ondataavailable=e=>{if(e.data&&e.data.size)chunks.push(e.data);};
      recorder.onerror=()=>{if(generation!==voice.generation)return;stopVoice('No se pudo grabar el audio.');showMicNotice('service');};
      recorder.onstop=async()=>{
        releaseMicStream();voice.recorder=null;
        if(generation!==voice.generation||!dialog?.open)return;
        voice.requesting=true;updateVoiceUI();status('Transcribiendo tu mensaje…');
        const recordingType=recorder.mimeType||mime||'audio/webm';
        const blob=new Blob(chunks,{type:recordingType});
        if(blob.size<800){stopVoice('La grabación fue demasiado corta.');showMicNotice('short');return;}
        if(blob.size>MAX_AUDIO_BYTES){stopVoice('La grabación superó el límite de tamaño.');return;}
        const filename=recordingType.includes('mp4')?'mensaje.mp4':recordingType.includes('ogg')?'mensaje.ogg':'mensaje.webm';
        try{
          const text=await uploadVoiceAudio(blob,filename);
          if(generation!==voice.generation||!dialog?.open)return;
          voice.requesting=false;updateVoiceUI();
          status('Texto reconocido. Enviando tu mensaje…');
          await sendMessage(text);
        }catch(error){
          if(generation!==voice.generation)return;
          stopVoice('No se pudo transcribir el audio. Puedes escribir tu mensaje.');
          showMicNotice('service',error.message);
        }
      };
      voice.requesting=false;voice.recording=true;updateVoiceUI();
      status('Grabando… Habla y pulsa otra vez el micrófono para enviar (máximo 30 s).');
      recorder.start(1000);
      voice.timeout=setTimeout(()=>{if(generation===voice.generation&&voice.recording)finishRecording();},MAX_RECORDING_MS);
    }catch(error){
      if(stream){for(const t of stream.getTracks()){try{t.stop();}catch{/* closed */}}}
      if(generation!==voice.generation)return;
      stopVoice();
      const name=String(error?.name||'');
      if(['NotAllowedError','PermissionDeniedError','SecurityError'].includes(name)){
        showMicNotice('denied');status('Chrome o Windows no concedió acceso al dispositivo de audio.');
      }else if(['NotFoundError','DevicesNotFoundError','NotReadableError','TrackStartError','OverconstrainedError'].includes(name)){
        showMicNotice('missing');status('No fue posible abrir el micrófono.');
      }else{
        showMicNotice('unsupported');status('No se pudo iniciar la grabación.');
      }
    }
  }

  async function sendMessage(value) {
    if(state.busy)return;
    value=String(value||'').trim();
    if(!value||value.length>MAX_MESSAGE_CHARS){status(isSuperAdmin()?'El contenido es demasiado grande para una sola petición.':'Escribe un mensaje de hasta 20.000 caracteres.');return;}
    const input=byId('wtBuilderInput');
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
  // A6: only adds spoken transcript turns to the draft conversation.
  // It neither saves nor activates an agent and cannot edit state/budget.
  window.WorkticAgentBuilderVoiceBridge = Object.freeze({
    history() { return historyForRequest().map(m => ({role:m.role, content:m.content})); },
    addTurn(role, rawText) {
      if (!dialog?.open || (role!=='user' && role!=='assistant')) return;
      const content=String(rawText||'').trim().slice(0,MAX_MESSAGE_CHARS);
      if (!content) return;
      state.messages.push({role,content});
      if (state.messages.length>80) state.messages.splice(0,state.messages.length-80);
      state.proposalAt=-1;
      messageBubble(role,content);
      renderProposal();updateComposer();
    }
  });

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
            <div class="wt-builder-compose"><label class="wt-builder-sr-only" for="wtBuilderInput">Escribe un mensaje</label><textarea id="wtBuilderInput" rows="2" placeholder="Cuéntame sobre tu empresa, pega tu presentación o describe el agente que deseas crear…"></textarea><div class="wt-builder-compose-bottom"><span id="wtBuilderStatus" role="status">Conversa libremente.</span><span id="wtBuilderCount" class="wt-builder-count" aria-label="Caracteres utilizados">0 / 20.000</span><div class="wt-builder-composer-actions"><button type="button" id="wtBuilderMic" aria-label="Conversar por voz" aria-pressed="false" title="Conversar por voz">${icons.mic}</button><button type="button" id="wtBuilderSend" aria-label="Enviar mensaje" title="Enviar mensaje">${icons.send}</button></div></div></div>
            <div class="wt-builder-mic-notice" id="wtBuilderMicNotice" role="alert" hidden><span aria-hidden="true">🎙️</span><p id="wtBuilderMicNoticeText"></p><button type="button" id="wtBuilderMicNoticeClose" aria-label="Cerrar indicaciones del micrófono">×</button></div>
            <div class="wt-builder-chat-note">🎙️ El micrófono inicia una conversación de voz en vivo. La asistente te saluda primero y después puedes hablar con ella de forma natural, sin volver a pulsar el micrófono. También puedes escribir en el mismo chat. Nada se crea ni publica automáticamente.</div>
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
    byId('wtBuilderMic').addEventListener('click',()=>{
      if(typeof window.WorkticAgentBuilderLiveStart === 'function')window.WorkticAgentBuilderLiveStart();
      else {
        status('La conversación de voz en vivo no está disponible en este momento. Actualiza la página e inténtalo nuevamente.');
        showMicNotice('service','El módulo de voz en vivo no se cargó.');
      }
    });
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
    document.addEventListener('visibilitychange',()=>{if(document.hidden&&(voice.active||voice.requesting))stopVoice('Voz detenida porque la ventana no está visible.');});
    renderProposal();updateComposer();updateVoiceUI();
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup,{once:true}); else setup();
})();
