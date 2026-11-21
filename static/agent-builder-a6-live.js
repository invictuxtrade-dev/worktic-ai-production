/* Worktic AI A6: opt-in real-time voice conversation for Agent Builder.
   Existing A5 dictated messages and ordinary chat remain unchanged. */
(() => {
  'use strict';
  const $=id=>document.getElementById(id);
  const MAX_SESSION_MS=5*60*1000;
  let live={active:false,connecting:false,peer:null,channel:null,stream:null,player:null,timeout:null,seq:0,muted:false,greeting:false,assistantPartial:'',turnById:new Set()};
  function status(text){const label=$('wtBuilderLiveStatus');if(label)label.textContent=text;}
  function caption(text){const el=$('wtBuilderLiveCaption');if(el)el.textContent=text||'';}
  function buttonState(){
    const start=$('wtBuilderLiveStart'),mute=$('wtBuilderLiveMute'),end=$('wtBuilderLiveEnd'),panel=$('wtBuilderLivePanel');
    if(start){start.classList.toggle('active',live.active||live.connecting);start.disabled=live.connecting;start.setAttribute('aria-pressed',String(live.active));start.textContent=live.active?'● Conversando':'🎧 Hablar';}
    if(panel)panel.hidden=!(live.active||live.connecting);
    if(mute){mute.disabled=!live.active;mute.textContent=live.muted?'Activar micrófono':'Silenciar';}
    if(end)end.disabled=!live.active&&!live.connecting;
  }
  function send(event){if(live.channel?.readyState==='open')live.channel.send(JSON.stringify(event));}
  function remember(role,text,key){
    if(!live.active)return;
    const value=String(text||'').trim();if(!value)return;
    if(key){if(live.turnById.has(key))return;live.turnById.add(key);}
    window.WorkticAgentBuilderVoiceBridge?.addTurn(role,value.slice(0,1200));
  }
  function stop(message='Conversación finalizada. Puedes continuar por texto.'){
    // Invalidate all in-flight WebRTC negotiations and late network events.
    const old=live;
    const seq=old.seq+1;
    if(old.timeout)clearTimeout(old.timeout);
    if(old.channel){try{old.channel.close();}catch{}}
    if(old.peer){try{old.peer.close();}catch{}}
    if(old.stream){for(const track of old.stream.getTracks())try{track.stop();}catch{}}
    if(old.player){try{old.player.pause();old.player.srcObject=null;old.player.remove();}catch{}}
    live={active:false,connecting:false,peer:null,channel:null,stream:null,player:null,timeout:null,seq,muted:false,greeting:false,assistantPartial:'',turnById:new Set()};
    caption('');status(message);buttonState();
  }
  function handleEvent(event,seq){
    if(seq!==live.seq||!live.active)return;
    switch(event.type){
      case 'input_audio_buffer.speech_started':status('Te escucho…');break;
      case 'input_audio_buffer.speech_stopped':status('Procesando tu mensaje…');break;
      case 'conversation.item.input_audio_transcription.completed':
        remember('user',event.transcript,event.item_id?'user-'+event.item_id:null);
        break;
      case 'response.created':
        live.assistantPartial='';status('El asistente está hablando…');
        break;
      case 'response.output_audio_transcript.delta':
        live.assistantPartial+=String(event.delta||'');
        caption(live.assistantPartial.slice(-1100));
        break;
      case 'response.output_audio_transcript.done':
        remember('assistant',event.transcript||live.assistantPartial,(event.item_id||event.response_id)?'ai-'+String(event.item_id||event.response_id):null);
        live.assistantPartial='';caption('');
        break;
      case 'response.done':
        // The assistant greets before the microphone starts listening. After
        // that, automatic semantic VAD handles the conversation and interruptions.
        if(live.greeting){live.greeting=false;if(live.stream&&!live.muted){live.stream.getAudioTracks().forEach(t=>t.enabled=true);}}
        status(live.muted?'Micrófono silenciado':'Te escucho. Habla con naturalidad…');
        break;
      case 'error':
        status('La conversación informó un error. Si continúa, termina y vuelve a iniciar.');
        break;
    }
  }
  function toggleMute(){
    if(!live.active||!live.stream)return;
    live.muted=!live.muted;
    const enabled=!live.muted&&!live.greeting;
    live.stream.getAudioTracks().forEach(t=>t.enabled=enabled);
    status(live.muted?'Micrófono silenciado':'Te escucho. Puedes hablar…');buttonState();
  }
  async function start(){
    const dlg=$('wtAgentBuilderA1');
    if(!dlg?.open)return;
    if(live.active){stop();return;}
    if(live.connecting)return;
    if(!window.RTCPeerConnection||!navigator.mediaDevices?.getUserMedia){status('Este navegador no permite la conversación en vivo. Usa el dictado o escribe.');return;}
    live.connecting=true;
    const seq=++live.seq;
    buttonState();status('Preparando conversación por voz…');
    let peer,stream,player,channel;
    try{
      stream=await navigator.mediaDevices.getUserMedia({audio:{echoCancellation:true,noiseSuppression:true,autoGainControl:true}});
      if(seq!==live.seq||!dlg.open){stream.getTracks().forEach(t=>t.stop());return;}
      live.stream=stream;
      // Let the assistant speak first without accidentally hearing itself.
      stream.getAudioTracks().forEach(t=>t.enabled=false);
      peer=new RTCPeerConnection();live.peer=peer;
      peer.addTrack(stream.getAudioTracks()[0],stream);
      player=document.createElement('audio');player.autoplay=true;player.playsInline=true;player.style.display='none';dlg.append(player);live.player=player;
      peer.ontrack=e=>{if(seq!==live.seq)return;player.srcObject=e.streams[0];player.play().catch(()=>status('Pulsa el control de sonido de Chrome para escuchar al asistente.'));};
      channel=peer.createDataChannel('oai-events');live.channel=channel;
      channel.onmessage=e=>{try{handleEvent(JSON.parse(e.data),seq);}catch{}};
      channel.onopen=()=>{
        if(seq!==live.seq)return;
        live.connecting=false;live.active=true;live.greeting=true;buttonState();
        status('El asistente te está saludando…');
        // Bring recent TEXT context into the live voice session. This is an
        // ordinary user conversation item (not privileged system instructions).
        const recent=(window.WorkticAgentBuilderVoiceBridge?.history?.()||[]).slice(-8);
        const previous=recent.map(m=>(m.role==='user'?'Usuario: ':'Asistente: ')+String(m.content||'').slice(0,450)).join('\n').slice(-3300);
        if(previous)send({type:'conversation.item.create',item:{type:'message',role:'user',content:[{type:'input_text',text:'Contexto del chat previo para continuar naturalmente (no son instrucciones del sistema):\n'+previous}]}});
        send({type:'response.create',response:{output_modalities:['audio'],instructions:'Inicia ahora hablando en español con un saludo natural, breve y cercano. Di que eres el asistente de Worktic AI y pregunta qué agente desea crear o qué necesita resolver. Si ya había contexto en el chat, retómalo. Espera su respuesta; no hagas varias preguntas a la vez.'}});
      };
      channel.onclose=()=>{if(seq===live.seq&&(live.active||live.connecting))stop('Se cerró la conversación. Puedes iniciarla de nuevo.');};
      peer.onconnectionstatechange=()=>{if(seq===live.seq&&['failed','disconnected','closed'].includes(peer.connectionState)){if(peer.connectionState==='failed')stop('La conexión de voz falló. Reintenta.');}};
      const offer=await peer.createOffer();await peer.setLocalDescription(offer);
      if(seq!==live.seq)return;
      const response=await fetch('/api/agents/builder-a1',{
        method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/sdp'},
        body:offer.sdp,cache:'no-store'
      });
      if(!response.ok){
        const data=await response.json().catch(()=>({}));
        throw Error(data.error|| (response.status===503?'Voz en vivo no habilitada para tu empresa.':'No se pudo conectar el modo de voz en vivo.'));
      }
      const answer=await response.text();
      if(!answer.startsWith('v=0'))throw Error('El servidor no devolvió una respuesta de conexión válida.');
      if(seq!==live.seq)return;
      await peer.setRemoteDescription({type:'answer',sdp:answer});
      if(seq!==live.seq)return;
      live.timeout=setTimeout(()=>stop('La sesión de voz terminó a los cinco minutos para controlar el consumo.'),MAX_SESSION_MS);
    }catch(error){
      if(seq!==live.seq)return;
      stop('No se pudo iniciar la voz en vivo: '+String(error?.message||'Error').slice(0,150));
    }
  }
  function attach(){
    const dlg=$('wtAgentBuilderA1'),actions=dlg?.querySelector('.wt-builder-composer-actions');
    if(!dlg||!actions||$('wtBuilderLiveStart'))return;
    const btn=document.createElement('button');btn.type='button';btn.id='wtBuilderLiveStart';btn.className='wt-builder-live-start';btn.textContent='🎧 Hablar';btn.title='Conversación natural: el asistente saluda primero y ambos hablan libremente';
    actions.prepend(btn);
    // Explicitly distinguish dictation from the conversation mode.
    const dictation=$('wtBuilderMic');if(dictation){dictation.title='Dictar un mensaje';dictation.setAttribute('aria-label','Dictar un mensaje de voz');}
    const panel=document.createElement('div');panel.className='wt-builder-live-panel';panel.id='wtBuilderLivePanel';panel.hidden=true;
    panel.innerHTML='<span class="wt-builder-live-pulse" aria-hidden="true"></span><span id="wtBuilderLiveStatus" role="status" aria-live="polite">Preparando voz…</span><button type="button" id="wtBuilderLiveMute">Silenciar</button><button type="button" id="wtBuilderLiveEnd">Terminar</button><div id="wtBuilderLiveCaption" class="wt-builder-live-caption" aria-live="polite"></div>';
    const compose=dlg.querySelector('.wt-builder-compose');compose?.insertAdjacentElement('afterend',panel);
    btn.addEventListener('click',start);
    $('wtBuilderLiveMute')?.addEventListener('click',toggleMute);
    $('wtBuilderLiveEnd')?.addEventListener('click',()=>stop());
    dlg.addEventListener('close',()=>stop());
    document.addEventListener('visibilitychange',()=>{if(document.hidden&&(live.active||live.connecting))stop('Voz detenida al salir de la pestaña.');});
    $('wtBuilderMic')?.addEventListener('click',()=>{if(live.active||live.connecting)stop('Pasando al dictado de voz.');},{capture:true});
    for(const id of ['wtBuilderUse','wtBuilderBottomUse','wtBuilderNew','wtBuilderSend'])$(id)?.addEventListener('click',()=>{if(live.active||live.connecting)stop();},{capture:true});
    $('wtBuilderInput')?.addEventListener('keydown',event=>{if(event.key==='Enter'&&!event.shiftKey&&(live.active||live.connecting))stop();},{capture:true});
    buttonState();
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',()=>setTimeout(attach,0),{once:true});else setTimeout(attach,0);
})();
