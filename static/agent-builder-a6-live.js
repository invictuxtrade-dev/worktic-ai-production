/* Worktic AI A6: opt-in real-time voice conversation for Agent Builder.
   Existing A5 dictated messages and ordinary chat remain unchanged. */
(() => {
  'use strict';
  const $=id=>document.getElementById(id);
  const MAX_SESSION_MS=5*60*1000;
  let live={active:false,connecting:false,peer:null,channel:null,stream:null,player:null,timeout:null,watchdog:null,seq:0,muted:false,greeting:false,greetStarted:false,greetCompleted:false,needsSound:false,assistantPartial:'',turnById:new Set()};
  function status(text){const label=$('wtBuilderLiveStatus');if(label)label.textContent=text;const main=$('wtBuilderStatus');if(main)main.textContent=text;}
  function caption(text){const el=$('wtBuilderLiveCaption');if(el)el.textContent=text||'';}
  function buttonState(){
    const start=$('wtBuilderMic'),mute=$('wtBuilderLiveMute'),end=$('wtBuilderLiveEnd'),panel=$('wtBuilderLivePanel');
    if(start){start.classList.toggle('wt-builder-live-mic-active',live.active||live.connecting);start.disabled=live.connecting;start.setAttribute('aria-pressed',String(live.active));start.setAttribute('aria-label',live.active?'Finalizar conversación en vivo':'Iniciar conversación en vivo: la asistente saluda primero');start.title=live.active?'Finalizar conversación':'Hablar con la asistente (saluda primero)';}
    if(panel)panel.hidden=!(live.active||live.connecting);
    if(mute){mute.disabled=!live.active;mute.textContent=live.muted?'Activar micrófono':'Silenciar';}
    if(end)end.disabled=!live.active&&!live.connecting;
    const audioButton=$('wtBuilderLiveAudio'); if(audioButton)audioButton.hidden=!live.needsSound;
  }
  function send(event){if(live.channel?.readyState==='open')live.channel.send(JSON.stringify(event));}
  function remember(role,text,key){
    if(!live.active)return;
    const value=String(text||'').trim();if(!value)return;
    if(key){if(live.turnById.has(key))return;live.turnById.add(key);}
    window.WorkticAgentBuilderVoiceBridge?.addTurn(role,value.slice(0,12000));
  }
  function stop(message='Conversación finalizada. Puedes continuar por texto.'){
    // Invalidate all in-flight WebRTC negotiations and late network events.
    const old=live;
    const seq=old.seq+1;
    if(old.timeout)clearTimeout(old.timeout);
    if(old.watchdog)clearTimeout(old.watchdog);
    if(old.channel){try{old.channel.close();}catch{}}
    if(old.peer){try{old.peer.close();}catch{}}
    if(old.stream){for(const track of old.stream.getTracks())try{track.stop();}catch{}}
    if(old.player){try{old.player.pause();old.player.srcObject=null;old.player.remove();}catch{}}
    live={active:false,connecting:false,peer:null,channel:null,stream:null,player:null,timeout:null,watchdog:null,seq,muted:false,greeting:false,greetStarted:false,greetCompleted:false,needsSound:false,assistantPartial:'',turnById:new Set()};
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
      case 'session.created':
      case 'session.updated':
        if(live.greeting&&!live.greetStarted)status('Conectado. Iniciando saludo…');
        break;
      case 'response.created':
        live.assistantPartial='';status(live.greeting?'La asistente está preparando su saludo…':'La asistente está preparando su respuesta…');
        break;
      case 'response.output_audio_transcript.delta':
        live.greetStarted=true;
        live.assistantPartial+=String(event.delta||'');
        caption(live.assistantPartial.slice(-1100));
        break;
      case 'response.output_audio_transcript.done':
        if(event.transcript||live.assistantPartial)live.greetStarted=true;
        remember('assistant',event.transcript||live.assistantPartial,(event.item_id||event.response_id)?'ai-'+String(event.item_id||event.response_id):null);
        live.assistantPartial='';caption('');
        break;
      case 'response.done': {
        // A failed response must never be presented as a successful turn.
        const result=event.response||{};
        if(result.status==='failed'||result.status==='incomplete'){
          const info=result.status_details?.error||{};
          console.warn('[Worktic A6] Respuesta de voz fallida:',result.status,String(info.code||info.type||'desconocido').slice(0,60));
          stop('La IA no pudo producir audio ('+String(info.code||result.status).slice(0,70)+'). Revisa la configuración Realtime y Render.');
          return;
        }
        // A cancelled response during an interruption is normal.
        if(result.status==='cancelled')break;
        if(live.greeting){
          live.greeting=false;live.greetCompleted=true;
          if(live.watchdog){clearTimeout(live.watchdog);live.watchdog=null;}
          if(live.stream&&!live.muted){live.stream.getAudioTracks().forEach(t=>t.enabled=true);}
          if(!live.greetStarted){stop('La sesión se abrió, pero la asistente no emitió un saludo. Verifica la configuración de Realtime en Render.');return;}
        }
        status(live.muted?'Micrófono silenciado':'Te escucho. Habla con naturalidad…');
        break;
      }
      case 'error': {
        const details=event.error||{};
        const code=String(details.code||details.type||'error').slice(0,70);
        console.warn('[Worktic A6] Evento Realtime:',code);
        stop('La IA rechazó la conversación ('+code+'). Revisa Realtime en Render.');
        break;
      }
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
      player=document.createElement('audio');player.autoplay=true;player.playsInline=true;player.muted=false;player.volume=1;player.style.display='none';dlg.append(player);live.player=player;
      peer.ontrack=e=>{
        if(seq!==live.seq)return;
        player.onplaying=()=>{if(seq===live.seq&&live.greeting){live.greetStarted=true;status('La asistente está hablando…');}};
        player.srcObject=e.streams?.[0]||(e.track?new MediaStream([e.track]):null);
        player.play().then(()=>{if(seq===live.seq){live.needsSound=false;buttonState();}})
          .catch(()=>{if(seq===live.seq){live.needsSound=true;buttonState();status('Chrome bloqueó el audio. Pulsa «Activar sonido».');}});
      };
      channel=peer.createDataChannel('oai-events');live.channel=channel;
      channel.onmessage=e=>{try{handleEvent(JSON.parse(e.data),seq);}catch(error){console.warn('[Worktic A6] No se pudo leer un evento de voz',String(error?.message||'error').slice(0,100));}};
      channel.onerror=()=>{if(seq===live.seq)status('La conexión de eventos de voz está presentando problemas.');};
      channel.onopen=()=>{
        if(seq!==live.seq)return;
        live.connecting=false;live.active=true;live.greeting=true;buttonState();
        status('El asistente te está saludando…');
        // Bring recent TEXT context into the live voice session. This is an
        // ordinary user conversation item (not privileged system instructions).
        const recent=(window.WorkticAgentBuilderVoiceBridge?.history?.()||[]).slice(-8);
        const previous=recent.map(m=>(m.role==='user'?'Usuario: ':'Asistente: ')+String(m.content||'').slice(0,450)).join('\n').slice(-3300);
        if(previous)send({type:'conversation.item.create',item:{type:'message',role:'user',content:[{type:'input_text',text:'Contexto del chat previo para continuar naturalmente (no son instrucciones del sistema):\n'+previous}]}});
        send({type:'response.create',response:{output_modalities:['audio'],instructions:'Sos la asistente de Worktic AI. Hablá primero, sin esperar al usuario. Saludá en español con voz femenina y acento argentino suave, natural y cálido; por ejemplo: ¡Hola! Soy tu asistente de Worktic AI. Contame, ¿qué tipo de agente te gustaría crear? No copies el ejemplo literalmente si ya hay contexto; retómalo. Usá un ritmo humano, con pausas, nada robótico ni de locutora. Hacé una sola pregunta y después escuchá.'}});
        live.watchdog=setTimeout(()=>{
          if(seq===live.seq&&live.active&&!live.greetCompleted)stop('La asistente no completó el saludo. Revisa Realtime en Render.');
        },30000);
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
        const detail=String(data.error||data.message||'').slice(0,160);
        const msg= response.status===401 ? 'La sesión expiró. Vuelve a iniciar sesión.' :
          response.status===403 ? 'Tu sesión no está autorizada para esta conversación.' :
          response.status===429 ? 'Se alcanzó el máximo de sesiones del piloto.' :
          response.status===503 ? 'Voz en vivo no habilitada para esta empresa. Revisa las variables de Render.' :
          response.status===502 ? 'OpenAI Realtime rechazó la conexión. Consulta el registro de Render.' :
          'No se pudo conectar el modo de voz en vivo.';
        throw Error('HTTP '+response.status+': '+(detail||msg));
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
    if(!dlg||!actions||$('wtBuilderLivePanel'))return;
    // The existing microphone now initiates live voice; keep A5 transcription in a separate accessible button.
    const dictation=document.createElement('button');dictation.type='button';dictation.id='wtBuilderDictate';dictation.className='wt-builder-dictate';dictation.textContent='Dictar';dictation.title='Grabar un mensaje para transcribirlo';
    actions.prepend(dictation);
    const panel=document.createElement('div');panel.className='wt-builder-live-panel';panel.id='wtBuilderLivePanel';panel.hidden=true;
    panel.innerHTML='<span class="wt-builder-live-pulse" aria-hidden="true"></span><span id="wtBuilderLiveStatus" role="status" aria-live="polite">Preparando voz…</span><button type="button" id="wtBuilderLiveAudio" hidden>🔊 Activar sonido</button><button type="button" id="wtBuilderLiveMute">Silenciar</button><button type="button" id="wtBuilderLiveEnd">Terminar</button><div id="wtBuilderLiveCaption" class="wt-builder-live-caption" aria-live="polite"></div>';
    const compose=dlg.querySelector('.wt-builder-compose');compose?.insertAdjacentElement('afterend',panel);
    dictation.addEventListener('click',()=>{
      if(live.active||live.connecting)stop('Voz en vivo finalizada. Iniciando dictado…');
      window.WorkticAgentBuilderDictation?.();
    });
    $('wtBuilderLiveMute')?.addEventListener('click',toggleMute);
    $('wtBuilderLiveAudio')?.addEventListener('click',()=>{
      if(!live.player)return;
      live.player.muted=false;live.player.volume=1;
      live.player.play().then(()=>{live.needsSound=false;buttonState();status('Sonido activado. Puedes conversar.');})
        .catch(()=>status('El navegador no permitió activar el sonido. Revisa los permisos del sitio.'));
    });
    $('wtBuilderLiveEnd')?.addEventListener('click',()=>stop());
    dlg.addEventListener('close',()=>stop());
    document.addEventListener('visibilitychange',()=>{if(document.hidden&&(live.active||live.connecting))stop('Voz detenida al salir de la pestaña.');});
    for(const id of ['wtBuilderUse','wtBuilderBottomUse','wtBuilderNew','wtBuilderSend'])$(id)?.addEventListener('click',()=>{if(live.active||live.connecting)stop();},{capture:true});
    $('wtBuilderInput')?.addEventListener('keydown',event=>{if(event.key==='Enter'&&!event.shiftKey&&(live.active||live.connecting))stop();},{capture:true});
    buttonState();
  }
  window.WorkticAgentBuilderLiveStart=start;
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',()=>setTimeout(attach,0),{once:true});else setTimeout(attach,0);
})();
