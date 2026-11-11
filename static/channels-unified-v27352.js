/* WorkticAI V27.35.3 — Corrección de formulario Chat Web sobre Canales unificados.
 * Una sola experiencia de creación/listado. Las API de telefonía y chat web
 * siguen siendo sus fuentes de verdad: no se duplican registros ni saldos.
 */
(()=>{'use strict';
const $=s=>document.querySelector(s);
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const nativeTypes={whatsapp_cloud:'WhatsApp Business Cloud (Oficial)',whatsapp_qr:'WhatsApp QR',telegram:'Telegram Bot',messenger:'Facebook Messenger',webchat:'Chat Web',voice_ai:'Llamadas IA',instagram:'Instagram (oficial de Meta)',email_ai:'Email IA (Gmail / Microsoft 365)'};
const category={whatsapp_cloud:'whatsapp_cloud',whatsapp_qr:'whatsapp_qr',telegram:'telegram',messenger:'messenger',webchat:'web',voice_ai:'voice'};
let allChannels=new Map(),webAvailable=false,voiceAvailable=false,webCountFallback=0;
const notice=e=>{if(window.toast)window.toast(e);else alert(e)};
const api=(url,options)=>window.api(url,options);
const closeModal=()=>{const d=$('#modal');if(d?.open)d.close()};
function openVoiceAdvanced(tab='channels'){
 const nav=$('aside nav button[data-view="voice-ai"]');if(!nav){notice('Módulo de voz no disponible');return}
 nav.click();const b=$(`#voice-ai [data-voice-tab="${tab}"]`);if(b)b.click();
}
async function voiceForm(ch={}){
 let d;try{d=await api('/api/voice/v2735/overview')}catch(e){notice('Llamadas IA no disponible: '+e.message);return}
 const agents=(d.agents||[]).filter(a=>a.status==='active'||String(a.id)===String(ch.agent_id));
 if(!agents.length){notice('Primero crea y activa un agente IA en Agentes IA');return}
 const vs=d.voices||[];
 if(!vs.length){notice('El proveedor de voz no devolvió voces disponibles');return}
 closeModal();
 window.modal(`<h2>${ch.id?'Configurar':'Nueva conexión'} · Llamadas IA</h2><div class="form wt-channel-form">
 <label>Nombre de la conexión<input id="ucVoiceName" maxlength="80" value="${esc(ch.name||'Atención telefónica IA')}" required></label>
 <label>Agente asignado<select id="ucVoiceAgent">${agents.map(a=>`<option value="${esc(a.id)}" ${String(a.id)===String(ch.agent_id)?'selected':''}>${esc(a.name)}</option>`).join('')}</select></label>
 <label>Voz del agente<select id="ucVoiceVoice">${vs.map(v=>`<option value="${esc(v.id)}" ${v.id===(ch.voice||'marin')?'selected':''}>${esc(v.name)} · ${esc(v.style||'')}</option>`).join('')}</select></label>
 <label>Número telefónico (opcional)<input id="ucVoicePhone" type="tel" placeholder="+573001234567" value="${esc(ch.phone||'')}"></label>
 <label>Duración máxima por llamada (minutos)<input id="ucVoiceDuration" type="number" min="1" max="15" value="${Number(ch.max_minutes||5)}"></label>
 <label class="check"><input id="ucVoiceActive" type="checkbox" ${ch.enabled?'checked':''}> Activar conexión</label>
 <div class="notice">${d.active?'Proveedor configurado.':'El proveedor de voz aún no está activo.'} ${d.sales_enabled?'': 'La creación de canales requiere habilitación comercial de voz en administración.'} No se ejecutarán llamadas hasta habilitar la telefonía.</div>
 <div class="actions"><button type="button" id="ucVoiceSave">${ch.id?'Guardar cambios':'Crear conexión'}</button><button type="button" class="secondary" id="ucVoiceAdvanced">Herramientas de voz</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);
 $('#ucVoiceAdvanced').onclick=()=>{closeModal();openVoiceAdvanced('channels')};
 $('#ucVoiceSave').onclick=async e=>{
  const p={id:ch.id||'',name:$('#ucVoiceName').value.trim(),agent_id:Number($('#ucVoiceAgent').value),voice:$('#ucVoiceVoice').value,phone:$('#ucVoicePhone').value.trim(),max_minutes:Number($('#ucVoiceDuration').value),enabled:$('#ucVoiceActive').checked};
  if(p.name.length<3||p.max_minutes<1||p.max_minutes>15){notice('Revisa el nombre y la duración');return}
  e.currentTarget.disabled=true;
  try{await api('/api/voice/v2735/channels',{method:'POST',body:JSON.stringify(p)});closeModal();notice('Conexión guardada');await refresh()}
  catch(error){e.currentTarget.disabled=false;notice(error.message)}
 };
}
function inlineError(message){
 const field=$('#ucDescription');
 if(field){field.textContent=message;field.setAttribute('role','alert');field.style.color='#9f1239';field.style.borderColor='#fda4af';field.style.background='#fff1f2';}
 notice(message);
}
async function createWeb(c){
 if(!window.workticWebChat?.prepare||!window.workticWebChat?.create){
  inlineError('No se cargó la configuración de Chat Web. Actualiza la página (Ctrl+F5) e inténtalo de nuevo.');return false;
 }
 let info;
 try{info=await window.workticWebChat.prepare()}
 catch(e){inlineError('No se pudo consultar Chat Web: '+(e.message||'Error de conexión'));return false}
 if(!info?.feature){
  inlineError('Chat Web no está habilitado en el servidor. En Render cambia WORKTIC_WEBCHAT_ENABLED a true y espera el nuevo despliegue.'+(info?.error?' Detalle: '+info.error:''));
  return false;
 }
 if(!info.agents?.length){
  inlineError('No tienes agentes IA activos. Crea o activa uno desde Agentes IA para conectar el Chat Web.');return false;
 }
 closeModal();
 try{if(c)window.workticWebChat.edit(c.id);else window.workticWebChat.create();return true}
 catch(e){notice('Error abriendo configuración Chat Web: '+(e.message||e));return false}
}
function openEmailChannel(c,tab){
 if(!window.WorkticEmailChannel?.open){notice('La gestión modal de Email IA no cargó. Actualiza la página.');return}
 window.WorkticEmailChannel.open(c,tab);
}
async function createConnection(){
 window.modal(`<h2>Nueva conexión</h2><p class="muted">Todos los canales se administran desde aquí. Selecciona el tipo de conexión.</p>
 <div class="form wt-channel-form"><label>Tipo de canal<select id="ucType">${Object.entries(nativeTypes).map(([id,label])=>`<option value="${id}">${esc(label)}</option>`).join('')}</select></label>
 <div id="ucNativeFields"><label>Nombre del canal<input id="ucName" maxlength="80" placeholder="Ej. Ventas y soporte"></label><label>Agente IA asignado<select id="ucAgent"><option value="0">Selecciona un agente activo</option></select></label></div>
 <div class="notice" id="ucDescription">La conexión se creará dentro de tu empresa, respetando los límites de tu plan.</div>
 <div class="actions"><button type="button" id="ucContinue">Crear conexión</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);
 api('/api/agents').then(d=>{const s=$('#ucAgent');if(s)s.replaceChildren(new Option('Selecciona un agente activo','0'),...(d.agents||[]).filter(a=>a.status==='active').map(a=>new Option(a.name,a.id)))}).catch(()=>{});
 const input=$('#ucType'), fields=$('#ucNativeFields'), btn=$('#ucContinue'),desc=$('#ucDescription');
 const update=()=>{desc.removeAttribute('role');desc.removeAttribute('style');const web=input.value==='webchat',voice=input.value==='voice_ai',wa=input.value==='whatsapp_cloud',ms=input.value==='messenger',ig=input.value==='instagram',email=input.value==='email_ai';fields.hidden=web||voice||ig||email;btn.textContent=wa?'Crear y conectar con Meta':ms?'Crear y conectar con Facebook':ig?'Ir a conectar Instagram':email?'Conectar correo empresarial':web||voice?'Continuar a configuración':'Crear conexión';desc.textContent=email?'Conecta un buzón Gmail o Microsoft 365 autorizado. Cada buzón aparece como canal de tu empresa y se administra desde Mis Canales. La sincronización y los envíos solo se activan al configurarlos en el servidor.':ig?'Instagram se autoriza una sola vez desde Redes Sociales > Cuentas. Una vez conectada, aparecerá aquí para asignar un agente IA. La conexión de clientes externos mediante Unified Login sigue en pruebas; no se garantiza aún mensajería.':web?'Configura el dominio HTTPS, el agente y el código de instalación.':voice?'Selecciona la voz, el agente y los permisos de telefonía.':wa?'Conexión oficial paso a paso: 1. Elige un agente activo. 2. Autoriza tu negocio y número directamente en Meta. 3. Verifica tu conexión. No necesitas copiar tokens ni configurar webhooks. La disponibilidad depende de tu plan y de la autorización de Meta.':'El backend validará tu plan y la asignación del agente.'};
 input.onchange=update;update();
 btn.onclick=async()=>{
  const type=input.value;
  if(type==='webchat'){
   btn.disabled=true;btn.textContent='Comprobando Chat Web…';
   try{await createWeb()}finally{if($('#ucContinue')===btn){btn.disabled=false;btn.textContent='Continuar a configuración'}}
   return;
  }
  if(type==='voice_ai'){await voiceForm();return}
  if(type==='email_ai'){closeModal();openEmailChannel(null,'accounts');return}
  if(type==='instagram'){closeModal();const nav=$('aside nav button[data-view="socialhub"]');if(!nav){notice('No se encontro Redes Sociales');return}nav.click();setTimeout(()=>{if(typeof window.switchSocialTab==='function')window.switchSocialTab('connections');else $('button[data-social-tab="connections"]')?.click();notice('Conecta Instagram desde Cuentas. Cuando se autorice, vuelve a Mis Canales para asignar el agente.')},200);return}
  const p={type,name:$('#ucName').value.trim(),assigned_agent_id:Number($('#ucAgent').value||0)};
  if(!p.name){notice('Escribe el nombre del canal');return}
  if(type==='whatsapp_cloud'&&!p.assigned_agent_id){notice('Primero selecciona un agente IA activo de tu empresa.');return}
  btn.disabled=true;
  try{const created=await api('/api/channels/connections',{method:'POST',body:JSON.stringify(p)});closeModal();notice('Conexión creada en tu empresa');await refresh();
    if(type==='whatsapp_cloud'){
      // Second step: official Meta authorization, only after user clicks its own button.
      // Meta SDK is prepared by the existing dialog before FB.login.
      try{await window.connectTenantChannel({id:created.id,type:'whatsapp_cloud',name:p.name,status:'draft',assigned_agent_id:p.assigned_agent_id,config_json:'{}'});}
      catch(err){notice('Conexión creada. Ábrela desde Mis Canales para finalizar con Meta: '+err.message)}
    }
    if(type==='messenger'){
      try{await window.workticMessengerWizard.open({id:created.id,type:'messenger',name:p.name,status:'draft',assigned_agent_id:p.assigned_agent_id});}
      catch(err){notice('Canal creado. Configúralo desde Mis Canales: '+err.message)}
    }
  } catch(e){btn.disabled=false;notice(e.message)}
 };
}
function mapped(c,type){return {...c,type};}
function status(c){return c.type==='webchat'?(c.enabled?'Activo':'Pausado'):c.type==='voice_ai'?(c.enabled?'Activo':'Pausado'):(c.status||'Borrador')}
function details(c){if(c.type==='whatsapp_cloud'){let conf={};try{conf=JSON.parse(c.config_json||'{}')||{}}catch(_){conf={}}return conf.display_phone_number||c.external_account_id||'Pendiente de vincular'}return c.type==='webchat'?c.origin:c.type==='voice_ai'?(c.phone||'Sin número conectado'):c.external_account_id||'Pendiente de vincular'}
/* WORKTIC_B1_V273608_PREMIUM_CARD */
function wtPremiumWhatsAppCard(c,key,agentLookup){
 const raw=c.config_json;
 let cfg={};try{cfg=(typeof raw==='string'?JSON.parse(raw||'{}'):raw)||{}}catch(_){cfg={}}
 const state=status(c),online=state==='connected'||state==='active',isError=state==='error';
 const stateLabel=online?'Conectado':isError?'Revisar conexión':state==='paused'?'Pausado':state==='draft'?'Pendiente de conexión':state;
 const connectedNumber=cfg.display_phone_number||c.display_phone_number||c.external_account_id||'';
 const numberLinked=online&&String(connectedNumber).trim().length>0;
 const number=numberLinked?connectedNumber:'Número por vincular';
 const numberLabel=numberLinked?'NÚMERO VINCULADO':'NÚMERO PENDIENTE';
 const business=numberLinked?(cfg.verified_name||'Cuenta de WhatsApp Business'):'Pendiente de autorización en Meta';
 const waitingIcon='<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>';
 const agentId=Number(c.assigned_agent_id||0);
 const agent=agentLookup.get(agentId);
 const agentName=agentId===0?'Sin asignar':(agent?.name||'Agente asignado');
 const agentHint=agentId===0?'Elige un agente en Configurar':agent?.status==='active'?'Activo':agent?'Revisar estado del agente':'Asignado al canal';
 const initials=agentId===0?'IA':String(agent?.name||'IA').trim().slice(0,1).toUpperCase();
 const whatsIcon='<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20.2 11.7a8.2 8.2 0 0 1-12 7.2L3 20l1.2-5a8.2 8.2 0 1 1 16-3.3Z"/><path d="M8.4 9c.3 2.9 2.5 5 5.4 5.8.7.2 1.1-.2 1.6-.9"/></svg>';
 const action=(id,label,klass='')=>`<button type="button" class="${klass}" data-uc-action="${id}" data-uc-key="${esc(key)}">${label}</button>`;
 return `<article class="card wt-unified-card wt-wa-premium-card" data-key="${esc(key)}" aria-label="WhatsApp Business Oficial: ${esc(c.name||'Canal')}">
  <div class="wt-wa-head"><span class="wt-wa-brandicon">${whatsIcon}</span><div class="wt-wa-headtext"><div class="wt-wa-eyebrow">WHATSAPP BUSINESS</div><div class="wt-wa-subbrand">Cloud API oficial</div></div><span class="wt-wa-status ${online?'is-connected':isError?'is-error':'is-pending'}"><i aria-hidden="true"></i>${esc(stateLabel)}</span></div>
  <div class="wt-wa-body"><h3 class="wt-wa-title">${esc(c.name||'WhatsApp Oficial')}</h3>
   <div class="wt-wa-numberpanel ${numberLinked?'is-verified':'is-pending'}"><div><div class="wt-wa-label">${esc(numberLabel)}</div><div class="wt-wa-number">${esc(number)}</div><div class="wt-wa-verified">${esc(business)}</div></div><span class="wt-wa-numbericon ${numberLinked?'is-verified':'is-pending'}" role="img" aria-label="${numberLinked?'Número verificado':'Pendiente de vincular'}">${numberLinked?'✓':waitingIcon}</span></div>
   <div class="wt-wa-agent"><div class="wt-wa-agentavatar" aria-hidden="true">${esc(initials)}</div><div class="wt-wa-agentinfo"><span>AGENTE IA</span><strong>${esc(agentName)}</strong><small>${esc(agentHint)}</small></div><span class="wt-wa-agentchevron" aria-hidden="true">✦</span></div>
  </div>
  <div class="wt-wa-footer"><div class="wt-wa-primaryactions">${action('configure','Configurar','wt-wa-btn-primary')}${action('templates','Plantillas','wt-wa-btn-secondary')}</div>
   <div class="wt-wa-secondaryactions">${action('inbox','Conversaciones','wt-wa-textbtn')}${action('test','Probar conexión','wt-wa-textbtn')}${online?action('pause','Pausar','wt-wa-textbtn'):''}
   <details class="wt-wa-more"><summary aria-label="Más acciones" title="Más acciones">•••</summary><div class="wt-wa-moremenu">${action('delete','Eliminar conexión','wt-wa-btn-danger')}</div></details></div>
  </div></article>`;
}

/* WORKTIC_B1_V273614_MESSENGER_PREMIUM_CARD: solo presentacion */
function wtPremiumMessengerCard(c,key,agentLookup){
 let cfg={};try{const raw=c.config_json;cfg=(typeof raw==='string'?JSON.parse(raw||'{}'):raw)||{}}catch(_){cfg={}}
 const state=String(status(c)||'').toLowerCase();
 const online=state==='connected'||state==='active'||state==='activo';
 const isError=state==='error'||state==='failed';
 const paused=state==='paused'||state==='pausado';
 const stateLabel=online?'Conectado':isError?'Revisar conexión':paused?'Pausado':'Pendiente de conexión';
 const pageId=String(c.external_account_id||cfg.page_id||'').trim();
 const hasPage=pageId.length>0;
 // El servidor actual conserva el ID de pagina; usa el nombre si lo proporciona.
 // Como alternativa, solo se deduce de un nombre de canal con el sufijo exacto '- Messenger Oficial'.
 const channelName=String(c.name||'');
 const fromChannel=/^(.+?)\s*[-–—]\s*(?:Facebook\s+)?Messenger\s+Oficial\s*$/i.exec(channelName);
 const pageName=String(cfg.page_name||c.page_name||(fromChannel?fromChannel[1].trim():'Página de Facebook')).trim();
 const agentId=Number(c.assigned_agent_id||c.agent_id||0);
 const agent=agentLookup.get(agentId);
 const agentName=agentId===0?'Sin asignar':agent?.name||`Agente #${agentId}`;
 const agentHint=agentId===0?'Atención humana · sin agente IA':agent?.status==='active'?'Activo':agent?'Revisar estado del agente':'Asignado al canal';
 const initials=agentId===0?'IA':String(agentName).trim().slice(0,1).toUpperCase();
 const messengerIcon='<svg viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 3.4c-5.1 0-9.2 3.6-9.2 8.1 0 2.4 1.1 4.6 3 6.1v3l3.1-1.7c1 .3 2 .5 3.1.5 5.1 0 9.2-3.6 9.2-8S17.1 3.4 12 3.4Z"/><path d="m7.3 14.2 3.2-3.6 2.7 2.1 3.5-3.6-3.2 4.9-2.8-2.2-3.4 2.4Z" fill="currentColor" stroke="none"/></svg>';
 const waitingIcon='<svg viewBox="0 0 24 24" width="17" height="17" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>';
 const action=(id,label,klass='')=>`<button type="button" class="${klass}" data-uc-action="${id}" data-uc-key="${esc(key)}">${label}</button>`;
 return `<article class="card wt-unified-card wt-wa-premium-card wt-ms-premium-card" data-key="${esc(key)}" aria-label="Facebook Messenger: ${esc(channelName||'Canal')}">
  <div class="wt-wa-head"><span class="wt-wa-brandicon wt-ms-brandicon">${messengerIcon}</span><div class="wt-wa-headtext"><div class="wt-wa-eyebrow">FACEBOOK MESSENGER</div><div class="wt-wa-subbrand">Canal oficial de Meta</div></div><span class="wt-wa-status ${online?'is-connected':isError?'is-error':paused?'is-paused':'is-pending'}"><i aria-hidden="true"></i>${esc(stateLabel)}</span></div>
  <div class="wt-wa-body"><h3 class="wt-wa-title">${esc(channelName||'Messenger Oficial')}</h3>
   <div class="wt-wa-numberpanel ${hasPage?'is-verified':'is-pending'}"><div><div class="wt-wa-label">${hasPage?'PÁGINA VINCULADA':'PÁGINA PENDIENTE'}</div><div class="wt-wa-number wt-ms-page-name">${esc(hasPage?pageName:'Página por vincular')}</div><div class="wt-wa-verified wt-ms-page-id">${esc(hasPage?'ID de página: '+pageId:'Pendiente de autorización en Facebook')}</div></div><span class="wt-wa-numbericon ${online&&hasPage?'is-verified':'is-pending'}" role="img" aria-label="${online&&hasPage?'Página conectada':'Conexión pendiente'}">${online&&hasPage?'✓':waitingIcon}</span></div>
   <div class="wt-wa-agent"><div class="wt-wa-agentavatar" aria-hidden="true">${esc(initials)}</div><div class="wt-wa-agentinfo"><span>AGENTE IA</span><strong>${esc(agentName)}</strong><small>${esc(agentHint)}</small></div><span class="wt-wa-agentchevron" aria-hidden="true">✦</span></div>
  </div>
  <div class="wt-wa-footer"><div class="wt-wa-primaryactions">${action('configure','Configurar','wt-wa-btn-primary')}${action('test','Probar conexión','wt-wa-btn-secondary')}</div>
   <div class="wt-wa-secondaryactions">${action('inbox','Conversaciones','wt-wa-textbtn')}${online?action('pause','Pausar','wt-wa-textbtn'):''}
   <details class="wt-wa-more"><summary aria-label="Más acciones" title="Más acciones">•••</summary><div class="wt-wa-moremenu">${action('delete','Eliminar conexión','wt-wa-btn-danger')}</div></details></div>
  </div></article>`;
}

function wtEmailChannelCard(c,key){
 const linked=c.status==='connected';
 const provider=c.provider==='google'?'Gmail':c.provider==='microsoft'?'Microsoft 365':'Correo empresarial';
 return `<article class="card wt-unified-card wt-email-channel-card" data-key="${esc(key)}">
 <div class="top"><span class="badge">EMAIL IA · ${esc(provider)}</span><span class="pill ${linked?'wt-channel-on':''}">${linked?'Conectado':'Requiere reconexión'}</span></div>
 <h3>${esc(c.email||'Buzón empresarial')}</h3>
 <p class="wt-channel-detail">Canal de atención oficial · ${esc(provider)}. Correos asociados a tu empresa y CRM.</p>
 <small>Sincronización: ${esc(c.sync_at||'Pendiente')} · Respuestas automáticas apagadas</small>
 <div class="actions"><button type="button" data-uc-action="configure" data-uc-key="${esc(key)}">Gestionar canal</button><button type="button" class="secondary" data-uc-action="inbox" data-uc-key="${esc(key)}">Bandeja Email IA</button></div>
 </article>`;
}
function card(c,key,agentLookup){if(c.type==='email_ai')return window.WorkticEmailChannel?.card(c,key)||wtEmailChannelCard(c,key);if(c.type==='whatsapp_cloud')return wtPremiumWhatsAppCard(c,key,agentLookup);if(c.type==='messenger')return wtPremiumMessengerCard(c,key,agentLookup);const label=nativeTypes[c.type]||c.type,active=['connected','active','Activo'].includes(status(c));
 return `<article class="card wt-unified-card" data-key="${esc(key)}"><div class="top"><span class="badge ${esc(category[c.type]||'')}">${esc(label)}</span><span class="pill ${active?'wt-channel-on':''}">${esc(status(c))}</span></div><h3>${esc(c.name)}</h3><p class="wt-channel-detail">${esc(details(c))}</p><small>Agente asignado: ${esc(c.assigned_agent_id||c.agent_id||'Sin asignar')}</small>${c.type==='whatsapp_qr'?`<div id="qrbox-${esc(c.id)}"></div>`:''}<div class="actions">
 <button type="button" data-uc-action="configure" data-uc-key="${esc(key)}">Configurar</button>
 ${c.type==='webchat'?`<button type="button" class="secondary" data-uc-action="install" data-uc-key="${esc(key)}">Instalar widget</button><button type="button" class="secondary" data-uc-action="inbox" data-uc-key="${esc(key)}">Conversaciones</button>`:''}
 ${c.type==='voice_ai'?`<button type="button" class="secondary" data-uc-action="test" data-uc-key="${esc(key)}">Probar voz</button>`:''}
 ${c.type==='whatsapp_cloud'?`<button type="button" class="secondary" data-uc-action="templates" data-uc-key="${esc(key)}">Plantillas</button>`:''}
 ${!['webchat','voice_ai'].includes(c.type)?`<button type="button" class="secondary" data-uc-action="test" data-uc-key="${esc(key)}">Probar conexión</button><button type="button" class="secondary" data-uc-action="pause" data-uc-key="${esc(key)}">Pausar</button><button type="button" class="danger" data-uc-action="delete" data-uc-key="${esc(key)}">Eliminar</button>`:''}</div></article>`}
async function refresh(){
 const grid=$('#channelConnections');if(!grid)return;
 const [base,health,web,voice,agentResponse,emailResponse]=await Promise.allSettled([api('/api/channels/connections'),api('/api/channels/health'),api('/api/webchat/channels'),api('/api/voice/v2735/overview'),api('/api/agents'),api('/api/email/p1/accounts')]);
 if(base.status!=='fulfilled'){notice('Error consultando los canales: '+base.reason.message);return}
 const d=base.value,h=health.status==='fulfilled'?health.value:{states:{},messages:0};h.states=h.states||{};
 webAvailable=web.status==='fulfilled'&&web.value.feature_enabled!==false;voiceAvailable=voice.status==='fulfilled';
 const webItems=webAvailable?(web.value.channels||[]):[],voiceItems=voiceAvailable?(voice.value.channels||[]):[];
 const emailItems=emailResponse.status==='fulfilled'?(emailResponse.value.items||[]):[];
 allChannels=new Map();
 const agentLookup=new Map();if(agentResponse.status==='fulfilled'){for(const a of (agentResponse.value.agents||[])){if(Number.isSafeInteger(Number(a.id))&&Number(a.id)>0)agentLookup.set(Number(a.id),{name:a.name,status:a.status});}}
 (d.connections||[]).forEach(c=>allChannels.set('native:'+c.id,mapped(c,c.type)));
 webItems.forEach(c=>allChannels.set('web:'+c.id,mapped(c,'webchat')));
 voiceItems.forEach(c=>allChannels.set('voice:'+c.id,mapped(c,'voice_ai')));
 emailItems.forEach(c=>allChannels.set('email:'+c.id,mapped({...c,name:c.email},'email_ai')));
 if(!emailItems.length){const disabled=emailResponse.status==='rejected' && /no está habilitado/i.test(String(emailResponse.reason?.message||''));allChannels.set('email:setup',mapped({id:0,name:'Email IA',email:'',setup:true,status:disabled?'disabled':'pending'},'email_ai'));}
 const recordedWeb=Math.max(Number(d.web_count||0),webItems.length);
 const count=(d.connections||[]).length+recordedWeb+voiceItems.length+emailItems.length;
 $('#channelLimit').textContent=`${count} conexiones registradas · límite estándar del plan: ${Number(d.max_total||0)}`;
 const active=(h.states.connected||0)+webItems.filter(c=>c.enabled).length+voiceItems.filter(c=>c.enabled).length+emailItems.filter(c=>c.status==='connected').length;
 $('#channelHealth').innerHTML=[['Activos',active],['Esperando QR',h.states.waiting_qr||0],['Con error',h.states.error||0],['Mensajes aislados',h.messages||0]].map(([label,v])=>`<div class="stat"><b>${Number(v)||0}</b><small>${label}</small></div>`).join('');
 grid.innerHTML=allChannels.size?[...allChannels.entries()].map(([key,c])=>card(c,key,agentLookup)).join(''):'<p>No hay conexiones. Usa «Nueva conexión» para conectar tu primer canal.</p>';
 if(!webAvailable||!voiceAvailable){const p=document.createElement('p');p.className='wt-unified-note';p.textContent=[!webAvailable?'Chat Web no habilitado o no disponible.':'',!voiceAvailable?'Servicio de voz no disponible.':''].filter(Boolean).join(' ');grid.after(p);grid.parentElement.querySelectorAll('.wt-unified-note').forEach((el,i)=>{if(i>0)el.remove()})}
 else grid.parentElement.querySelectorAll('.wt-unified-note').forEach(el=>el.remove());
}
async function action(evt){const b=evt.target.closest('[data-uc-action]');if(!b)return;const c=allChannels.get(b.dataset.ucKey);if(!c)return;
 const name=b.dataset.ucAction;
 if(c.type==='email_ai'){if(name==='configure'||name==='inbox')openEmailChannel(c,name==='inbox'?'inbox':'accounts');return}
 if(c.type==='webchat'){
  if(name==='inbox'){$('aside nav button[data-view="inbox"]')?.click();const f=$('#channelConvFilter');if(f){f.value='web';f.dispatchEvent(new Event('change'))}return}
  await window.workticWebChat?.prepare();
  if(name==='configure')window.workticWebChat?.edit(c.id);
  if(name==='install')window.workticWebChat?.install(c.id);
  return;
 }
 if(c.type==='voice_ai'){
  if(name==='configure')await voiceForm(c);
  if(name==='test')openVoiceAdvanced('test');
  return;
 }
 if(name==='inbox'){$('aside nav button[data-view="inbox"]')?.click();return;}
 if(name==='configure')return window.connectTenantChannel(c);
 if(name==='test')return window.testTenantChannel(c);
 if(name==='templates')return window.manageWhatsAppTemplates(c);
 if(name==='pause'){if(confirm('¿Pausar esta conexión?')){await window.pauseTenantChannel(c.id);await refresh()}return}
 if(name==='delete'){await window.deleteTenantChannel(c.id);await refresh()}
}
function init(){
 const btn=$('#newChannel'),reload=$('#refreshChannels'),grid=$('#channelConnections');if(!btn||!grid)return;
 btn.onclick=createConnection;reload.onclick=refresh;grid.addEventListener('click',e=>{action(e).catch(err=>notice(err.message))});
 const nav=$('aside nav button[data-view="channels"]');nav?.addEventListener('click',()=>{setTimeout(refresh,0)});
 // Recargas y configuración avanzada siguen disponibles desde sus pantallas existentes.
 window.workticUnified={refresh,createConnection,voiceForm};
 refresh();
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();
