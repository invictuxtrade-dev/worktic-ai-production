/* WorkticAI Web Chat V27.36 - management shell. Feature-gated on backend. */
(()=>{'use strict';
const $=(s)=>document.querySelector(s);
const E=(v)=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const moduleURL='/api/webchat/channels';
const state={channels:[],agents:[],feature:false,aiAvailable:false};
function notice(message,cls=''){const n=$('#ww36Notice');if(n){n.className='ww36-notice '+cls;n.textContent=message}}
const nav=()=>document.querySelector('nav button[data-view="webchat"]');
const open=()=>nav()?.click();
const planNotice='Si el servicio aún no está habilitado en Render, mantén WORKTIC_WEBCHAT_ENABLED=false y realiza primero las pruebas de seguridad.';
async function load(){
 if(!$('#ww36List'))return;
 notice('Consultando canales de tu empresa…');
 let data;
 try{data=await window.api(moduleURL)}catch(e){
   state.feature=false;notice('Chat Web aún no está habilitado en este servidor. '+planNotice,'error');
   $('#ww36Stats').replaceChildren();$('#ww36List').innerHTML='<div class="ww36-empty"><strong>Canal pendiente de activación</strong>La administración aparecerá cuando el backend Chat Web esté desplegado y autorizado.</div>';return;
 }
 state.feature=true;state.channels=data.channels||[];state.aiAvailable=!!data.ai_available;
 try{const r=await window.api('/api/agents');state.agents=(r.agents||[]).filter(a=>a.status==='active')}catch(e){state.agents=[]}
 const active=state.channels.filter(c=>c.enabled).length;
 $('#ww36Stats').innerHTML=[['Canales configurados',state.channels.length],['Activos',active],['Con agente asignado',state.channels.filter(c=>c.agent_id).length]].map(x=>`<div class="ww36-stat"><b>${x[1]}</b><small>${x[0]}</small></div>`).join('');
 notice(state.aiAvailable?'Canal disponible. La atención automática IA puede configurarse por canal.':'Canal disponible para atención humana en la Bandeja. La respuesta automática IA sigue deshabilitada.','good');
 $('#ww36List').innerHTML=state.channels.length?state.channels.map(c=>`<article class="ww36-card">
 <div class="ww36-card-top"><strong>${E(c.name)}</strong><span class="ww36-label ${c.enabled?'on':''}">${c.enabled?'Activo':'Inactivo'}</span></div>
 <p>${E(c.origin)}</p><dl><div><dt>Agente asignado</dt><dd>${E(state.agents.find(x=>x.id===c.agent_id)?.name||'#'+c.agent_id)}</dd></div><div><dt>Atención automática</dt><dd>${c.auto_ai?'IA habilitada':'Respuesta humana'}</dd></div><div><dt>ID del canal</dt><dd>${c.id}</dd></div></dl>
 <div class="ww36-actions"><button type="button" data-ww-edit="${c.id}">Configurar</button><button type="button" class="secondary" data-ww-code="${c.id}">Instalar</button><button type="button" class="secondary" data-ww-inbox="${c.id}">Conversaciones</button>${c.enabled?`<button type="button" class="secondary" data-ww-stop="${c.id}">Pausar</button>`:''}</div></article>`).join(''):'<div class="ww36-empty"><strong>Conecta tu primer Chat Web</strong>Tu sitio puede atender visitantes y guardar conversaciones en WorkticAI.<p><button type="button" data-ww-create>Crear canal</button></p></div>';
}
function form(c={}){
 if(!state.feature){notice('El administrador debe habilitar primero el canal en el servidor.','error');return}
 if(!state.agents.length){notice('Primero crea un agente activo en Agentes IA.','error');return}
 const opts=state.agents.map(a=>`<option value="${a.id}" ${Number(a.id)===Number(c.agent_id)?'selected':''}>${E(a.name)}</option>`).join('');
 window.modal(`<div class="ww36-form"><span class="ww36-eyebrow">CHAT WEB · CONEXIÓN</span><h2>${c.id?'Configurar canal':'Crear canal Web'}</h2><p class="muted">Solo los visitantes del dominio HTTPS autorizado podrán utilizar el widget de este canal.</p><label>Nombre del canal<input id="wwName" maxlength="80" value="${E(c.name||'Chat de mi empresa')}" required></label><label>Origen HTTPS de tu página<input id="wwOrigin" type="url" value="${E(c.origin||'')}" placeholder="https://miempresa.com" required></label><label>Agente IA asignado<select id="wwAgent">${opts}</select></label><label class="check"><input type="checkbox" id="wwEnabled" ${c.enabled?'checked':''}> Permitir que los visitantes utilicen este chat</label><label class="check"><input type="checkbox" id="wwAI" ${c.auto_ai?'checked':''} ${state.aiAvailable?'':'disabled'}> Respuestas automáticas con el agente IA</label><p class="ww36-privacy">${state.aiAvailable?'La IA se ejecutará cuando el visitante escriba y puede consumir créditos de tu plan.':'La atención automática requiere autorización del servidor y configuración de OpenAI. Se puede atender manualmente desde la Bandeja.'}</p><div class="actions"><button type="button" id="wwSave">${c.id?'Guardar cambios':'Crear canal'}</button><button value="cancel" class="secondary">Cancelar</button></div></div>`);
 const save=$('#wwSave');if(!save)return;
 save.onclick=async()=>{
   const payload={id:c.id||0,name:$('#wwName').value.trim(),origin:$('#wwOrigin').value.trim(),agent_id:Number($('#wwAgent').value),enabled:$('#wwEnabled').checked,auto_ai:$('#wwAI').checked};
   if(!/^https:\/\/[^/\s]+\/?$/.test(payload.origin)){window.toast('Indica un dominio HTTPS válido, sin rutas.');return}
   save.disabled=true;try{await window.api(moduleURL,{method:c.id?'PUT':'POST',body:JSON.stringify(payload)});$('#modal')?.close();await load();await window.workticUnified?.refresh?.();window.toast('Chat Web guardado')}catch(e){window.toast(e.message);save.disabled=false}
 };
}
function snippet(c){
 const endpoint=location.origin+'/worktic-webchat-v1.js';
 const code=`<script src="${endpoint}" data-key="${c.public_key}"></`+`script>`;
 window.modal(`<div class="ww36-form"><span class="ww36-eyebrow">INSTALACIÓN CHAT WEB</span><h2>${E(c.name)}</h2><p class="muted">Añade este fragmento antes de cerrar la etiqueta body de <b>${E(c.origin)}</b>.</p><textarea class="ww36-snippet" id="wwSnippet" readonly>${E(code)}</textarea><div class="actions"><button id="wwCopy" type="button">Copiar código</button><button value="cancel" class="secondary">Cerrar</button></div><p class="ww36-privacy">Si el dominio está inactivo o es diferente al autorizado, el widget no podrá iniciar conversaciones. No insertes el código en sitios que no controlas.</p></div>`);
 $('#wwCopy').onclick=async()=>{try{await navigator.clipboard.writeText(code);window.toast('Código copiado')}catch(e){$('#wwSnippet').select();document.execCommand('copy');window.toast('Código seleccionado')}};
}
async function pause(c){if(!confirm(`¿Pausar el canal ${c.name}? Los visitantes dejarán de poder iniciar y continuar conversaciones.`))return;
 try{await window.api(moduleURL+'?id='+encodeURIComponent(c.id),{method:'DELETE'});await load();await window.workticUnified?.refresh?.();window.toast('Chat Web pausado')}catch(e){window.toast(e.message)}
}
document.addEventListener('click',(e)=>{
 const n=e.target.closest('[data-open-webchat]');if(n){open();return}
 const create=e.target.closest('#ww36New,[data-ww-create]');if(create){form();return}
 const refresh=e.target.closest('#ww36Refresh');if(refresh){load();return}
 const inbox=e.target.closest('#ww36GoInbox,[data-ww-inbox]');if(inbox){document.querySelector('nav button[data-view="inbox"]')?.click();if($('#channelConvFilter')){$('#channelConvFilter').value='web';$('#channelConvFilter').dispatchEvent(new Event('change'))}return}
 const c=state.channels.find(x=>Number(x.id)===Number(e.target.closest('[data-ww-edit],[data-ww-code],[data-ww-stop]')?.getAttribute('data-ww-edit')||e.target.closest('[data-ww-code]')?.getAttribute('data-ww-code')||e.target.closest('[data-ww-stop]')?.getAttribute('data-ww-stop')));
 if(!c)return;
 if(e.target.closest('[data-ww-edit]'))form(c);
 else if(e.target.closest('[data-ww-code]'))snippet(c);
 else if(e.target.closest('[data-ww-stop]'))pause(c);
});
window.workticWebChat={prepare:load,create:()=>form(),edit:id=>{const c=state.channels.find(x=>Number(x.id)===Number(id));if(c)form(c);else notice('Conexión no encontrada','error')},install:id=>{const c=state.channels.find(x=>Number(x.id)===Number(id));if(c)snippet(c);else notice('Conexión no encontrada','error')}};
nav()?.addEventListener('click',()=>{load()});
$('#refreshChannels')?.addEventListener('click',()=>{if(state.feature)load()});
})();
