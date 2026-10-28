/* WorkticAI V27.35.3 — Corrección de formulario Chat Web sobre Canales unificados.
 * Una sola experiencia de creación/listado. Las API de telefonía y chat web
 * siguen siendo sus fuentes de verdad: no se duplican registros ni saldos.
 */
(()=>{'use strict';
const $=s=>document.querySelector(s);
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const nativeTypes={whatsapp_cloud:'WhatsApp Business Cloud (Oficial)',whatsapp_qr:'WhatsApp QR',telegram:'Telegram Bot',messenger:'Facebook Messenger',webchat:'Chat Web',voice_ai:'Llamadas IA'};
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
async function createConnection(){
 window.modal(`<h2>Nueva conexión</h2><p class="muted">Todos los canales se administran desde aquí. Selecciona el tipo de conexión.</p>
 <div class="form wt-channel-form"><label>Tipo de canal<select id="ucType">${Object.entries(nativeTypes).map(([id,label])=>`<option value="${id}">${esc(label)}</option>`).join('')}</select></label>
 <div id="ucNativeFields"><label>Nombre del canal<input id="ucName" maxlength="80" placeholder="Ej. Ventas y soporte"></label><label>Agente IA asignado<select id="ucAgent"><option value="0">Agente principal (sin asignación)</option></select></label></div>
 <div class="notice" id="ucDescription">La conexión se creará dentro de tu empresa, respetando los límites de tu plan.</div>
 <div class="actions"><button type="button" id="ucContinue">Crear conexión</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);
 api('/api/agents').then(d=>{const s=$('#ucAgent');if(s)s.replaceChildren(new Option('Agente principal (sin asignación)','0'),...(d.agents||[]).filter(a=>a.status==='active').map(a=>new Option(a.name,a.id)))}).catch(()=>{});
 const input=$('#ucType'), fields=$('#ucNativeFields'), btn=$('#ucContinue'),desc=$('#ucDescription');
 const update=()=>{desc.removeAttribute('role');desc.removeAttribute('style');const web=input.value==='webchat',voice=input.value==='voice_ai';fields.hidden=web||voice;btn.textContent=web||voice?'Continuar a configuración':'Crear conexión';desc.textContent=web?'Configura el dominio HTTPS, el agente y el código de instalación.':voice?'Selecciona la voz, el agente y los permisos de telefonía.': 'El backend validará tu plan y la asignación del agente.'};
 input.onchange=update;update();
 btn.onclick=async()=>{
  const type=input.value;
  if(type==='webchat'){
   btn.disabled=true;btn.textContent='Comprobando Chat Web…';
   try{await createWeb()}finally{if($('#ucContinue')===btn){btn.disabled=false;btn.textContent='Continuar a configuración'}}
   return;
  }
  if(type==='voice_ai'){await voiceForm();return}
  const p={type,name:$('#ucName').value.trim(),assigned_agent_id:Number($('#ucAgent').value||0)};
  if(!p.name){notice('Escribe el nombre del canal');return}
  btn.disabled=true;
  try{await api('/api/channels/connections',{method:'POST',body:JSON.stringify(p)});closeModal();notice('Conexión creada');await refresh()}
  catch(e){btn.disabled=false;notice(e.message)}
 };
}
function mapped(c,type){return {...c,type};}
function status(c){return c.type==='webchat'?(c.enabled?'Activo':'Pausado'):c.type==='voice_ai'?(c.enabled?'Activo':'Pausado'):(c.status||'Borrador')}
function details(c){return c.type==='webchat'?c.origin:c.type==='voice_ai'?(c.phone||'Sin número conectado'):c.external_account_id||'Pendiente de vincular'}
function card(c,key){const label=nativeTypes[c.type]||c.type,active=['connected','active','Activo'].includes(status(c));
 return `<article class="card wt-unified-card" data-key="${esc(key)}"><div class="top"><span class="badge ${esc(category[c.type]||'')}">${esc(label)}</span><span class="pill ${active?'wt-channel-on':''}">${esc(status(c))}</span></div><h3>${esc(c.name)}</h3><p class="wt-channel-detail">${esc(details(c))}</p><small>Agente asignado: ${esc(c.assigned_agent_id||c.agent_id||'Sin asignar')}</small>${c.type==='whatsapp_qr'?`<div id="qrbox-${esc(c.id)}"></div>`:''}<div class="actions">
 <button type="button" data-uc-action="configure" data-uc-key="${esc(key)}">Configurar</button>
 ${c.type==='webchat'?`<button type="button" class="secondary" data-uc-action="install" data-uc-key="${esc(key)}">Instalar widget</button><button type="button" class="secondary" data-uc-action="inbox" data-uc-key="${esc(key)}">Conversaciones</button>`:''}
 ${c.type==='voice_ai'?`<button type="button" class="secondary" data-uc-action="test" data-uc-key="${esc(key)}">Probar voz</button>`:''}
 ${c.type==='whatsapp_cloud'?`<button type="button" class="secondary" data-uc-action="templates" data-uc-key="${esc(key)}">Plantillas</button>`:''}
 ${!['webchat','voice_ai'].includes(c.type)?`<button type="button" class="secondary" data-uc-action="test" data-uc-key="${esc(key)}">Probar conexión</button><button type="button" class="secondary" data-uc-action="pause" data-uc-key="${esc(key)}">Pausar</button><button type="button" class="danger" data-uc-action="delete" data-uc-key="${esc(key)}">Eliminar</button>`:''}</div></article>`}
async function refresh(){
 const grid=$('#channelConnections');if(!grid)return;
 const [base,health,web,voice]=await Promise.allSettled([api('/api/channels/connections'),api('/api/channels/health'),api('/api/webchat/channels'),api('/api/voice/v2735/overview')]);
 if(base.status!=='fulfilled'){notice('Error consultando los canales: '+base.reason.message);return}
 const d=base.value,h=health.status==='fulfilled'?health.value:{states:{},messages:0};h.states=h.states||{};
 webAvailable=web.status==='fulfilled'&&web.value.feature_enabled!==false;voiceAvailable=voice.status==='fulfilled';
 const webItems=webAvailable?(web.value.channels||[]):[],voiceItems=voiceAvailable?(voice.value.channels||[]):[];
 allChannels=new Map();
 (d.connections||[]).forEach(c=>allChannels.set('native:'+c.id,mapped(c,c.type)));
 webItems.forEach(c=>allChannels.set('web:'+c.id,mapped(c,'webchat')));
 voiceItems.forEach(c=>allChannels.set('voice:'+c.id,mapped(c,'voice_ai')));
 const recordedWeb=Math.max(Number(d.web_count||0),webItems.length);
 const count=(d.connections||[]).length+recordedWeb+voiceItems.length;
 $('#channelLimit').textContent=`${count} conexiones registradas · límite estándar del plan: ${Number(d.max_total||0)}`;
 const active=(h.states.connected||0)+webItems.filter(c=>c.enabled).length+voiceItems.filter(c=>c.enabled).length;
 $('#channelHealth').innerHTML=[['Activos',active],['Esperando QR',h.states.waiting_qr||0],['Con error',h.states.error||0],['Mensajes aislados',h.messages||0]].map(([label,v])=>`<div class="stat"><b>${Number(v)||0}</b><small>${label}</small></div>`).join('');
 grid.innerHTML=allChannels.size?[...allChannels.entries()].map(([key,c])=>card(c,key)).join(''):'<p>No hay conexiones. Usa «Nueva conexión» para conectar tu primer canal.</p>';
 if(!webAvailable||!voiceAvailable){const p=document.createElement('p');p.className='wt-unified-note';p.textContent=[!webAvailable?'Chat Web no habilitado o no disponible.':'',!voiceAvailable?'Servicio de voz no disponible.':''].filter(Boolean).join(' ');grid.after(p);grid.parentElement.querySelectorAll('.wt-unified-note').forEach((el,i)=>{if(i>0)el.remove()})}
 else grid.parentElement.querySelectorAll('.wt-unified-note').forEach(el=>el.remove());
}
async function action(evt){const b=evt.target.closest('[data-uc-action]');if(!b)return;const c=allChannels.get(b.dataset.ucKey);if(!c)return;
 const name=b.dataset.ucAction;
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
