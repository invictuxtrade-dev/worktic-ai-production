/* Worktic AI B1 V27.36.13 — Messenger self-service via server-side Meta OAuth.
 * No Meta token, App Secret or verify token is sent to the browser.
 */
(()=>{'use strict';
const esc=s=>String(s??'').replace(/[&<>"']/g,x=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[x]));
const $=sel=>document.querySelector(sel);
const api=(p,o)=>window.api(p,o);
const notify=x=>window.toast?.(x)||alert(x);
const close=()=>{if($('#modal')?.open)$('#modal').close()};
function render(s){close();window.modal(s)}
async function start(c){
 const r=await api('/api/messenger/selfservice/start',{method:'POST',body:JSON.stringify({connection_id:c.id})});
 if(!r?.authorization_url||!/^https:\/\/www\.facebook\.com\//.test(r.authorization_url))throw Error('Meta no devolvió una URL de autorización válida');
 window.location.assign(r.authorization_url);
}
function intro(c){
 render(`<div class="modal-heading"><span class="eyebrow">META · MESSENGER</span><h2>Conectar Facebook Messenger</h2><p>Vincula una página de tu empresa con una autorización oficial de Facebook. No necesitas copiar credenciales.</p></div>
 <div class="form wt-channel-form"><div class="form-section">
 <div class="notice success"><b>1. Conecta con Facebook</b><p>Meta te permitirá autorizar las páginas que administras.</p></div>
 <div class="notice"><b>2. Selecciona la página</b><p>Solo mostraremos las páginas a las que concedas acceso.</p></div>
 <div class="notice"><b>3. Asigna un agente IA</b><p>Elige uno de los agentes activos de esta empresa.</p></div>
 <div class="notice"><b>4. Activa Messenger</b><p>Worktic comprobará los permisos y la suscripción del webhook.</p></div>
 </div><div class="actions"><button type="button" id="wtMessengerLogin">Continuar con Facebook</button><button type="button" class="secondary" id="wtMessengerClose">Cerrar</button></div></div>`);
 $('#wtMessengerClose').onclick=close;
 $('#wtMessengerLogin').onclick=async e=>{
  const b=e.currentTarget;b.disabled=true;b.textContent='Preparando autorización…';
  try{await start(c)}catch(err){b.disabled=false;b.textContent='Continuar con Facebook';notify(err.message)}
 };
}
async function choose(c,fromRedirect=false){
 let info;try{info=await api('/api/messenger/selfservice/pages?connection_id='+encodeURIComponent(c.id))}catch(err){notify(err.message);intro(c);return}
 const pages=info.pages||[];
 if(!pages.length){render(`<div class="modal-heading"><h2>Sin páginas autorizadas</h2><p>Facebook no devolvió páginas con permisos de administración y mensajes.</p></div><div class="form"><div class="notice">Verifica que administras una página y que Meta concedió los permisos necesarios.</div><div class="actions"><button type="button" id="wtMessengerRetry">Reintentar con Facebook</button><button type="button" class="secondary" id="wtMessengerClose">Cerrar</button></div></div>`);$('#wtMessengerRetry').onclick=()=>intro(c);$('#wtMessengerClose').onclick=close;return}
 const res=await api('/api/agents').catch(()=>({agents:[]}));
 const agents=(res.agents||[]).filter(a=>a.status==='active');
 render(`<div class="modal-heading"><span class="eyebrow">META · MESSENGER</span><h2>Selecciona tu página</h2><p>Tu cuenta de Facebook fue autorizada. Elige la página que Worktic atenderá.</p></div>
 <div class="form wt-channel-form"><label>Página de Facebook<select id="wtMessengerPage">${pages.map(p=>`<option value="${esc(p.id)}">${esc(p.name)} · ${esc(p.id)}</option>`).join('')}</select></label>
 <label>Agente IA asignado<select id="wtMessengerAgent"><option value="0">Atención humana (sin agente)</option>${agents.map(a=>`<option value="${Number(a.id)}" ${Number(a.id)===Number(c.assigned_agent_id)?'selected':''}>${esc(a.name)}</option>`).join('')}</select></label>
 <div class="notice">Al activar se comprueban los permisos de la página y se solicita a Meta la suscripción a los eventos. No se marcará conectado si Meta no confirma el resultado.</div>
 <div class="actions"><button type="button" id="wtMessengerActivate">Activar Messenger</button><button type="button" class="secondary" id="wtMessengerClose">Cancelar</button></div></div>`);
 $('#wtMessengerClose').onclick=close;
 $('#wtMessengerActivate').onclick=async e=>{
  const b=e.currentTarget;b.disabled=true;b.textContent='Verificando y conectando…';
  try{
   const result=await api('/api/messenger/selfservice/activate',{method:'POST',body:JSON.stringify({connection_id:Number(c.id),page_id:$('#wtMessengerPage').value,agent_id:Number($('#wtMessengerAgent').value)})});
   if(!result.ok||result.status!=='connected')throw Error('Meta no confirmó la conexión');
   render(`<div class="modal-heading"><span class="eyebrow">CONEXIÓN COMPLETADA</span><h2>Messenger conectado</h2><p>La página fue autorizada y Worktic registró la suscripción de eventos.</p></div><div class="form"><div class="notice success"><b>Conexión activa.</b> Envía un mensaje real desde una cuenta de prueba para comprobar la recepción y la respuesta.</div><div class="actions"><button type="button" id="wtMessengerDone">Finalizar</button></div></div>`);
   $('#wtMessengerDone').onclick=()=>{close();window.workticUnified?.refresh?.();window.loadChannels?.()};
  }catch(err){notify(err.message);b.disabled=false;b.textContent='Activar Messenger'}
 };
}
async function open(c){if(!c?.id)return notify('Primero crea el canal Messenger');if(c.status==='connected'){return notify('Este canal ya está conectado. Usa Probar conexión para revisar su estado.')}return intro(c)}
// Maintain existing manual configuration only for Worktic's internal support.
const legacy=window.connectTenantChannel;
window.connectTenantChannel=async function(c){
 if(c?.type==='messenger'&&window.currentUser?.role!=='superadmin')return open(c);
 if(c?.type==='messenger'&&window.currentUser?.role==='superadmin')return open(c);
 return legacy?.(c);
};
window.workticMessengerWizard={open};
function checkCallback(){
 const url=new URL(window.location.href),oauth=url.searchParams.get('messenger_oauth');
 if(!oauth)return;
 const id=Number(url.searchParams.get('connection_id'));
 url.searchParams.delete('messenger_oauth');url.searchParams.delete('connection_id');
 history.replaceState(null,'',url.pathname+url.search+url.hash);
 if(oauth==='error'){notify('Facebook no completó la autorización; intenta conectar nuevamente.');return}
 if(oauth==='ready'&&Number.isSafeInteger(id)&&id>0){
  api('/api/channels/connections').then(r=>{
   const c=(r.connections||[]).find(c=>Number(c.id)===id&&c.type==='messenger');
   if(!c)throw Error('El canal autorizado no pertenece a esta empresa');
   return choose(c,true);
  }).catch(err=>notify(err.message));
 }
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',checkCallback);else checkCallback();
})();
