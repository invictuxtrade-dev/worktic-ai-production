/* WORKTIC_EMAIL_ONLY_GMAIL_P1A5 */
/* Worktic AI · Email IA · Gestión modal en Mis Canales P1A.2.
   Sin almacenamiento de secretos en el cliente ni envío autónomo. */
(()=>{'use strict';
const E = v => String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const $ = s => document.querySelector(s);
const state = {accounts:[],messages:[],team:[],accountId:null,tab:'accounts',enabled:false,googleReady:false,syncEnabled:false,sendEnabled:false,selfServiceEnabled:false,p1bFastSync:false,p1cClassification:false,loading:false,error:'',info:'',replyId:null,busy:false};
// WORKTIC_EMAIL_P1B1_INCREMENTAL: read-only polling of the stored inbox, no autonomous sends.
let p1bPollTimer=null,p1bRequestActive=false;
const admin = () => ['superadmin','admin','owner'].includes(String(window.currentUser?.role||'').toLowerCase());
const provider = p => p==='google'?'Gmail':p==='microsoft'?'Microsoft 365':'Correo empresarial';
const dateText = x => {if(!x)return 'Sin sincronizar';const d=new Date(x);return Number.isNaN(d.getTime())?'Sin sincronizar':d.toLocaleString('es-CO')};
async function request(url,method='GET',payload){const opts={credentials:'same-origin',method,headers:{Accept:'application/json'}};if(payload!==undefined){opts.headers['Content-Type']='application/json';opts.body=JSON.stringify(payload)}
 const r=await fetch(url,opts);let data={};try{data=await r.json()}catch{}if(!r.ok){const err=new Error(data.error||data.message||`Error HTTP ${r.status}`);err.status=r.status;throw err}return data}
function card(c,key){const connected=c.status==='connected',empty=c.setup===true,disabled=c.status==='disabled';const name=empty?'Email IA · Correo empresarial':c.email||'Buzón empresarial';const p=empty?'Gmail':provider(c.provider);
 const s=connected?'Conectado':disabled?'Deshabilitado':empty?'Configuración pendiente':'Requiere reconexión';
 const hint=empty?'Conecta Gmail para empezar a recibir y atender correos en Worktic AI.':'Sincronización: '+dateText(c.sync_at)+' · Respuestas automáticas configurables por buzón';
 return `<article class="card wt-unified-card wt-email-p1a2-card" data-key="${E(key)}" aria-label="Email IA ${E(s)}">
 <div class="wt-email-card-head"><div class="wt-email-card-icon" aria-hidden="true">✉</div><div class="wt-email-brand"><strong>EMAIL IA</strong><small>${E(p)} · Canal de correo</small></div><span class="wt-email-status ${connected?'connected':disabled?'disabled':'pending'}">${E(s)}</span></div>
 <h3>${E(name)}</h3><div class="wt-email-info">${empty?'BUZÓN PENDIENTE':'BUZÓN VINCULADO'}<strong>${E(empty?'Sin cuenta autorizada':name)}</strong><small>${E(hint)}</small></div>
 <div class="wt-email-next"><span class="wt-email-next-icon">IA</span><div><b>Atención inteligente</b><small>CRM, clasificación y transferencia a humano · automatizaciones pendientes de habilitar</small></div></div>
 <div class="wt-email-card-actions"><button type="button" data-uc-action="configure" data-uc-key="${E(key)}">${empty?'Configurar Email IA':'Administrar canal'}</button><button type="button" class="secondary" data-uc-action="inbox" data-uc-key="${E(key)}">Ver bandeja</button></div></article>`;
}
function modalContents(){return `<div class="wt-email-modal"><div class="wt-email-modal-head"><div class="wt-email-modal-ico" aria-hidden="true">✉</div><div><h2>Email IA · Mis Canales</h2><p>Conecta y administra tus buzones de forma segura, sin salir de Mis Canales.</p></div></div>
 <div class="wt-email-tabs"><button type="button" data-email-action="tab-accounts" class="wt-email-tab">Conexiones</button><button type="button" data-email-action="tab-inbox" class="wt-email-tab">Bandeja de correo</button></div>
 <div id="wtEmailModalStatus" class="wt-email-modal-status" role="status" aria-live="polite">Cargando configuración…</div>
 <div id="wtEmailModalBody" class="wt-email-modal-body"></div>
 <div class="wt-email-modal-footer"><small>Las respuestas automáticas y las campañas comerciales no se activan desde esta ventana.</small><button type="button" data-email-action="close" class="secondary">Cerrar</button></div>
 </div>`;}
function statusHtml(){if(state.loading)return '<span>Consultando conexiones…</span>';
 if(!state.enabled)return `<strong>Configuración del servidor pendiente</strong><p>${E(state.error||'Email IA no está habilitado.')}</p><p>Un administrador debe activar <code>WORKTIC_EMAIL_P1_ENABLED=true</code> en Render. La sincronización y los envíos deben permanecer desactivados durante las pruebas.</p>`;
 const parts=[state.accounts.length+' buzón(es) registrado(s)',state.syncEnabled?'Sincronización habilitada':'Sincronización desactivada',state.sendEnabled?'Respuesta manual habilitada':'Respuesta manual desactivada'];
 return `<span>${E(parts.join(' · '))}</span>${state.info?`<p>${E(state.info)}</p>`:''}${state.error?`<p class="wt-email-error">${E(state.error)}</p>`:''}`;
}
function accountsHtml(){if(!state.enabled)return '<div class="wt-email-empty">Todavía no se pueden conectar buzones. Activa primero el módulo desde la configuración interna del servidor.</div>';
 const primary=`<div class="wt-email-provider-grid wt-email-provider-grid--single"><section class="wt-email-provider-featured"><h3>Gmail</h3><p>Conecta tu cuenta Gmail o Google Workspace para atender el correo empresarial desde Worktic AI.</p><button type="button" data-email-action="oauth-google" ${!state.googleReady||!admin()?'disabled':''}>${state.googleReady?'Conectar Gmail':'Pendiente de configurar OAuth'}</button><small>${state.googleReady?'Autorización oficial de Google disponible':'Configura las credenciales OAuth de Google en Render'}</small></section></div>`;
 const rows=state.accounts.map(c=>{const linked=c.status==='connected',selected=Number(c.id)===Number(state.accountId);return `<article class="wt-email-account ${selected?'selected':''}"><div><b>${E(c.email)}</b><small>${E(provider(c.provider))} · ${linked?'Conectado':'Desconectado'} · Última sincronización: ${E(dateText(c.sync_at))}</small>${c.last_error?`<small class="wt-email-error">${E(c.last_error)}</small>`:''}</div><div class="wt-email-account-buttons"><button type="button" data-email-action="account-inbox" data-email-id="${Number(c.id)}">Bandeja</button>${linked&&state.selfServiceEnabled&&!c.pilot_account?`<button type="button" class="secondary" data-email-action="sync-mode" data-email-id="${Number(c.id)}" data-email-enable="${!c.sync_allowed}" ${!admin()||state.busy?'disabled':''}>${c.sync_allowed?'Pausar recepción':'Activar recepción'}</button>`:''}<button type="button" class="secondary" data-email-action="sync" data-email-id="${Number(c.id)}" ${!linked||!c.sync_allowed||!state.syncEnabled||state.busy?'disabled':''}>Sincronizar</button><button type="button" class="secondary" data-email-action="disconnect" data-email-id="${Number(c.id)}" ${!linked||!admin()||state.busy?'disabled':''}>Desconectar</button></div></article>`}).join('');
 return `<h3 class="wt-email-section-title">Conectar buzón empresarial</h3>${primary}<div class="wt-email-account-list"><div class="wt-email-list-title"><h3>Mis buzones</h3><button type="button" class="secondary" data-email-action="refresh">Actualizar</button></div>${rows||'<div class="wt-email-empty">No hay buzones conectados. Usa uno de los proveedores anteriores.</div>'}</div><p class="wt-email-hint">Los buzones se vinculan a tu empresa mediante OAuth. La autorización del proveedor abrirá temporalmente su sitio web oficial y regresará a Worktic AI.</p>`;
}
// WORKTIC_EMAIL_P1B2_CHRONO_INBOX: sort by Gmail received timestamp, not database import order.
function emailP1B2Sorted(items){
 const millis=x=>{const n=Date.parse(String(x.received||''));return Number.isFinite(n)?n:0};
 return [...items].sort((a,b)=>(millis(b)-millis(a))||(Number(b.id||0)-Number(a.id||0)));
}
function messagesHtml(){if(!state.enabled)return '<div class="wt-email-empty">La bandeja estará disponible después de habilitar Email IA.</div>';
 const m=state.accountId?state.messages.filter(x=>Number(x.account_id)===Number(state.accountId)):state.messages;
 const selected=state.accounts.find(x=>Number(x.id)===Number(state.accountId));
 const options='<option value="">Asignar a humano…</option>'+state.team.map(x=>`<option value="${Number(x.id)}">${E(x.name)}</option>`).join('');
 const rows=m.map(x=>`<article class="wt-email-message"><div><b>${E(x.subject||'Sin asunto')}</b><span>${E(dateText(x.received))}</span></div><small>De: ${E(x.sender)} · Para: ${E(x.account)}</small><div class="wt-email-msglabels"><span>${E(x.category||'Sin clasificar')}</span><span>${E(x.priority||'Normal')}</span>${String(x.sender||'').toLowerCase()===String(x.account||'').toLowerCase()?'<span>Correo propio · sin CRM</span>':''}${x.state==='human'?'<span>Atención humana</span>':''}${['Promoción','Notificación','Spam','Revisión'].includes(x.category)?'<span>Sin creación automática de contacto</span>':''}</div><p>${E(x.summary||'')}</p><details><summary>Leer mensaje</summary><p class="wt-email-body-text">${E(String(x.body||'Sin contenido').slice(0,12000))}</p></details>
 <div class="wt-email-message-actions"><select data-email-select-for="${Number(x.id)}" aria-label="Integrante del equipo">${options}</select><button type="button" data-email-action="assign" data-email-id="${Number(x.id)}" ${state.busy?'disabled':''}>Asignar</button><button type="button" class="secondary" data-email-action="reply-open" data-email-id="${Number(x.id)}" ${!state.sendEnabled||state.busy?'disabled':''}>Responder</button></div>
 ${state.replyId===x.id?`<div class="wt-email-reply"><label>Respuesta a ${E(x.sender)}<textarea data-email-reply-for="${Number(x.id)}" rows="4" maxlength="8000" placeholder="Redacta una respuesta profesional"></textarea></label><button type="button" data-email-action="reply-send" data-email-id="${Number(x.id)}" ${state.busy?'disabled':''}>Enviar respuesta</button></div>`:''}</article>`).join('');
 return `<div class="wt-email-list-title"><div><h3>Bandeja Email IA</h3><small>${selected?'Buzón: '+E(selected.email):'Todos los buzones'}</small>${state.p1bFastSync&&state.syncEnabled?'<small>Actualización automática de esta bandeja cada 15 segundos</small>':''}</div><button type="button" class="secondary" data-email-action="refresh-messages">Actualizar</button></div>${!state.syncEnabled?'<p class="wt-email-hint">La sincronización está apagada en el servidor; no se recibirán mensajes nuevos hasta activarla.</p>':''}${rows||'<div class="wt-email-empty">Aún no hay correos sincronizados en este buzón.</div>'}<p class="wt-email-hint">${state.p1cClassification?'Clasificación conservadora activa para correos nuevos autorizados. Los mensajes históricos no se reclasifican.':'Clasificación no activada para estos buzones.'} Los correos comerciales nuevos se vinculan con Conversaciones cuando P1C.2 está activa; los borradores y envíos requieren activación independiente.</p>`;
}
function render(){const root=$('#modal .wt-email-modal');if(!root)return;
 const x=root.querySelector('#wtEmailModalBody'),s=root.querySelector('#wtEmailModalStatus');
 s.innerHTML=statusHtml();x.innerHTML=state.tab==='inbox'?messagesHtml():accountsHtml();
 root.querySelector('[data-email-action="tab-accounts"]').classList.toggle('active',state.tab==='accounts');root.querySelector('[data-email-action="tab-inbox"]').classList.toggle('active',state.tab==='inbox');
}
async function loadAccounts(){state.loading=true;state.error='';render();try{const d=await request('/api/email/p1/accounts');state.enabled=true;state.accounts=Array.isArray(d.items)?d.items:[];state.googleReady=!!d.google_ready;state.syncEnabled=!!d.sync_enabled;state.p1bFastSync=!!d.p1b_fast_sync;state.p1cClassification=!!d.p1c1_classification;state.sendEnabled=!!d.manual_send_enabled;state.selfServiceEnabled=!!d.self_service_enabled}catch(e){state.enabled=false;state.accounts=[];state.googleReady=false;state.syncEnabled=false;state.p1bFastSync=false;state.p1cClassification=false;state.sendEnabled=false;state.selfServiceEnabled=false;state.error=e.status===503?'Email IA no está habilitado o todavía no está disponible en el servidor.':e.message}finally{state.loading=false;render()}}
async function loadMessages(){if(!state.enabled){render();return}state.loading=true;render();try{const [m,u]=await Promise.all([request('/api/email/p1/messages'),request('/api/email/p1/team')]);state.messages=emailP1B2Sorted(Array.isArray(m.items)?m.items:[]);state.team=Array.isArray(u.items)?u.items:[];state.error=''}catch(e){state.messages=[];state.error=e.message}finally{state.loading=false;render()}}
async function p1bRefreshInboxQuietly(){
 const dialog=$('#modal'),root=$('#modal .wt-email-modal');
 if(!root||!dialog||!dialog.open){if(p1bPollTimer){clearInterval(p1bPollTimer);p1bPollTimer=null}return}
 if(document.hidden||!state.p1bFastSync||!state.syncEnabled||!state.enabled||state.tab!=='inbox'||state.loading||state.busy||state.replyId!==null||p1bRequestActive)return;
 if(root.contains(document.activeElement)&&document.activeElement?.matches('input,textarea,select'))return;
 p1bRequestActive=true;
 try{const d=await request('/api/email/p1/messages');const received=emailP1B2Sorted(Array.isArray(d.items)?d.items:[]);
 const fingerprint=items=>items.map(m=>[m.id,m.account_id,m.state,m.assigned_user_id].join(':')).join('|');
 if(fingerprint(received)!==fingerprint(state.messages)){
   const added=received.filter(m=>!state.messages.some(old=>Number(old.id)===Number(m.id))).length;
   state.messages=received;state.error='';if(added>0)state.info=added+' mensaje(s) incorporado(s) a la bandeja (pueden ser historicos)';
   render();
 }
 }catch(_){/* Keep last usable inbox on transient network failures. */}
 finally{p1bRequestActive=false}
}
async function execute(evt){const b=evt.target.closest('button[data-email-action]');if(!b||state.busy)return;const action=b.dataset.emailAction,id=Number(b.dataset.emailId||0);
 if(action==='close'){if(p1bPollTimer){clearInterval(p1bPollTimer);p1bPollTimer=null}document.querySelector('#modal')?.close();return}
 if(action==='tab-accounts'){state.tab='accounts';state.replyId=null;render();return}
 if(action==='tab-inbox'){state.tab='inbox';await loadMessages();return}
 if(action==='refresh'){await loadAccounts();return}
 if(action==='refresh-messages'){await loadMessages();return}
 if(action==='account-inbox'){state.accountId=id;state.tab='inbox';await loadMessages();return}
 if(action==='oauth-google'){
  if(!state.enabled||!admin()||!state.googleReady)return;
  location.assign('/api/email/p1/oauth/start?provider=google');return;
 }
 if(!['sync','sync-mode','disconnect','assign','reply-open','reply-send'].includes(action))return;
 if(action==='reply-open'){state.replyId=id;render();return}
 let url='',payload;
 if(action==='sync-mode'){
  if(!admin()||!state.selfServiceEnabled)return;
  const enable=b.dataset.emailEnable==='true';
  if(enable&&!confirm('¿Activar la recepción de Gmail para esta empresa? Solo los correos recibidos después de activar el buzón se incorporarán a Email IA y CRM. NO se activarán respuestas automáticas.'))return;
  url='/api/email/p1/account-settings';payload={account_id:id,enabled:enable};
 }
 else if(action==='sync'){if(!state.syncEnabled)return;url='/api/email/p1/sync?account_id='+id;}
 else if(action==='disconnect'){if(!admin()||!confirm('¿Desconectar el buzón? Se conserva el historial de correos ya recibido.'))return;url='/api/email/p1/accounts/disconnect';payload={id}}
 else if(action==='assign'){const sel=$(`#modal [data-email-select-for="${id}"]`);const userId=Number(sel?.value||0);if(!userId){state.error='Selecciona un integrante del equipo.';render();return}url='/api/email/p1/assign';payload={message_id:id,user_id:userId}}
 else if(action==='reply-send'){if(!state.sendEnabled)return;const txt=$(`#modal [data-email-reply-for="${id}"]`)?.value.trim();if(!txt){state.error='Escribe la respuesta.';render();return}if(!confirm('¿Enviar esta respuesta al remitente?'))return;url='/api/email/p1/reply';payload={message_id:id,text:txt,request_id:crypto.randomUUID()}}
 state.busy=true;state.error='';state.info=action==='sync'?'Sincronizando nuevos mensajes de Gmail…':'Procesando solicitud…';render();try{const out=await request(url,'POST',payload);state.info=action==='sync'?`Sincronización completada: ${Number(out.new||0)} mensajes nuevos`:action==='reply-send'?'Respuesta enviada al proveedor':action==='disconnect'?'Buzón desconectado':action==='sync-mode'?'Recepción de Gmail actualizada':'Asignación guardada';if(action==='disconnect'){state.accountId=null;state.tab='accounts'}if(action==='reply-send')state.replyId=null;await loadAccounts();if(state.tab==='inbox')await loadMessages();window.workticUnified?.refresh?.();}
 catch(e){state.info='';state.error=e.message}finally{state.busy=false;render()}
}
function open(c,tab='accounts'){
 if(typeof window.modal!=='function'){alert('La ventana de configuración no está disponible. Actualiza la página.');return}
 state.accountId=c&&!c.setup?Number(c.id)||null:null;state.tab=tab==='inbox'?'inbox':'accounts';state.info='';state.error='';state.replyId=null;state.messages=[];state.team=[];
 const existing=document.querySelector('#modal');if(existing?.open)existing.close();
 window.modal(modalContents());if(p1bPollTimer)clearInterval(p1bPollTimer);p1bPollTimer=setInterval(p1bRefreshInboxQuietly,15000);const root=$('#modal .wt-email-modal');root.addEventListener('click',e=>{execute(e).catch(err=>{state.error=err.message;render()})});
 loadAccounts().then(()=>{if(state.tab==='inbox')return loadMessages()});
}
window.WorkticEmailChannel={card,open};
function postOAuth(){const p=new URLSearchParams(location.search);if(p.get('email_connected')!=='1')return;setTimeout(()=>{const nav=document.querySelector('aside nav button[data-view="channels"]');nav?.click();open(null,'accounts')},700);
 // Evita abrir repetidamente el modal al recargar.
 p.delete('email_connected');p.delete('view');history.replaceState(null,'',location.pathname+(p.toString()?'?'+p.toString():'')+location.hash)}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',postOAuth);else postOAuth();
})();
