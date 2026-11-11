let inboxTeam=[],activeInboxMeta=null;
async function loadInboxOverview(){try{const x=await api('/api/inbox/overview');const el=$('#inboxKpis');if(el)el.innerHTML=[['Abiertas',x.open],['Pendientes',x.pending],['Modo humano',x.human],['Sin leer',x.unread],['Cerradas',x.closed]].map(v=>`<div class="inbox-kpi"><b>${v[1]||0}</b><small>${v[0]}</small></div>`).join('')}catch(e){console.warn(e)}}
async function loadInboxTeam(){try{const r=await api('/api/inbox/team');inboxTeam=Array.isArray(r)?r:(Array.isArray(r?.team)?r.team:Array.isArray(r?.users)?r.users:[])}catch(e){inboxTeam=[]}}
// P1C2: filtro visible de Email IA sin cambiar la plantilla HTML.
const emailFilter=document.getElementById('channelConvFilter');
if(emailFilter&&!emailFilter.querySelector('option[value="email"]')) emailFilter.add(new Option('Email IA','email'));
window.loadConvs=async function(){const removedBtn=document.getElementById("inboxDeletedBtn");if(removedBtn)removedBtn.hidden=!inboxCanManage();await Promise.all([loadInboxOverview(),loadInboxTeam()]);const r=await api('/api/conversations');allConvs=Array.isArray(r)?r:(Array.isArray(r?.conversations)?r.conversations:[]);applyConversationFilters()}
window.applyConversationFilters=function(){
 const q=($('#searchConv')?.value||'').toLowerCase().trim(),channel=$('#channelConvFilter')?.value||'all',status=$('#convStatusFilter')?.value||'all',mode=$('#convModeFilter')?.value||'all',from=$('#convDateFrom')?.value||'',to=$('#convDateTo')?.value||'',sort=$('#convSort')?.value||'newest';
 const fromDate=from?new Date(from+'T00:00:00'):null,toDate=to?new Date(to+'T23:59:59'):null;
 let xs=allConvs.filter(c=>{const txt=`${c.name||''} ${c.phone||''} ${c.last_text||''} ${c.tags||''} ${c.department||''}`.toLowerCase();const normalized=(c.channel||'whatsapp').toLowerCase();const mc=channel==='all'||normalized===channel||(channel==='whatsapp'&&(normalized==='whatsapp_qr'||normalized==='whatsapp_cloud'||normalized==='whatsapp'));const d=conversationDate(c.last_timestamp);return(!q||txt.includes(q))&&mc&&(status==='all'||(c.status||'open')===status)&&(mode==='all'||(c.mode||'ai')===mode)&&(!fromDate||!d||d>=fromDate)&&(!toDate||!d||d<=toDate)});
 xs.sort((a,b)=>{if(sort==='unread'){const u=(Number(b.unread)||0)-(Number(a.unread)||0);if(u)return u}if(sort==='priority'){const p={urgent:4,high:3,normal:2,low:1};const d=(p[b.priority]||2)-(p[a.priority]||2);if(d)return d}const da=conversationDate(a.last_timestamp)?.getTime()||0,db=conversationDate(b.last_timestamp)?.getTime()||0;return sort==='oldest'?da-db:db-da});
 const count=$('#convResultCount');if(count)count.textContent=`${xs.length} conversación${xs.length===1?'':'es'}`;renderConvs(xs)
}
window.renderConvs=function(xs){xs=Array.isArray(xs)?xs:[];const el=$('#convList');if(!el)return;el.innerHTML=xs.length?xs.map(c=>`<div class="conv ${c.chat_jid===activeChat?'active':''}" data-chat="${esc(c.chat_jid)}"><div class="conv-avatar">${esc((c.name||c.phone||'?').trim().charAt(0).toUpperCase())}</div><div class="conv-body"><div class="top"><b>${esc(c.name||c.phone)}</b><time>${formatConversationTime(c.last_timestamp)}</time></div><div class="conv-preview"><span class="channel-dot ${esc(c.channel)}">${c.channel==='telegram'?'TG':c.channel==='messenger'?'MS':c.channel==='web'?'WEB':c.channel==='email'?'EM':c.channel==='whatsapp_cloud'?'WC':'WA'}</span><small>${esc(c.last_text||'Sin mensajes')}</small>${c.unread?`<strong>${c.unread}</strong>`:''}</div><div class="conv-meta"><span class="mini-pill ${c.mode||'ai'}">${(c.mode||'ai')==='human'?'Humano':'IA'}</span><span class="mini-pill">${esc(c.status||'open')}</span>${c.priority&&c.priority!=='normal'?`<span class="mini-pill ${esc(c.priority)}">${esc(c.priority)}</span>`:''}${c.assigned_user_name?`<span class="mini-pill">${esc(c.assigned_user_name)}</span>`:''}</div></div></div>`).join(''):`<div class="empty-state compact"><b>No hay conversaciones</b><p>Ajusta los filtros.</p></div>`;$$('.conv').forEach(x=>x.onclick=()=>openChat(x.dataset.chat))}
// P1D.2: refresca SOLO los mensajes; conserva foco, panel de borradores y ediciones.
let workticEmailMessageRefreshBusy=false,workticEmailLastMessageDigest='';
window.refreshSelectedChatMessages=async function(){
 if(workticEmailMessageRefreshBusy||!activeChat)return;
 workticEmailMessageRefreshBusy=true;const chat=activeChat;
 try{
  const ms=await api('/api/messages?chat='+encodeURIComponent(chat));
  if(activeChat!==chat||!Array.isArray(ms))return;
  const digest=JSON.stringify(ms.map(x=>[x.id,x.wa_id,x.direction,x.text,x.status,x.timestamp]));
  if(workticEmailLastMessageDigest===digest)return;
  const target=$('#messages');if(!target)return;
  const nearBottom=target.scrollHeight-target.scrollTop-target.clientHeight<90;
  target.innerHTML=ms.map(m=>`<div class="msg ${m.direction==='out'?'out':''}">${esc(m.text)}<small>${new Date(m.timestamp).toLocaleString()} · ${esc(m.status)}</small></div>`).join('');
  if(nearBottom)target.scrollTop=target.scrollHeight;
  workticEmailLastMessageDigest=digest;
 }catch(e){console.warn('No se pudo refrescar mensajes',e)}finally{workticEmailMessageRefreshBusy=false}
};
window.openChat=async function(chat){activeChat=chat;workticEmailLastMessageDigest='';allConvs=Array.isArray(allConvs)?allConvs:[];const c=allConvs.find(x=>x.chat_jid===chat);const emailOnly=c?.channel==='email'||chat.startsWith('email:');const sf=$('#sendForm');if(sf)sf.style.display=emailOnly?'none':'';await api('/api/read',{method:'POST',body:JSON.stringify({chat})});const [ms,meta,notes]=await Promise.all([api('/api/messages?chat='+encodeURIComponent(chat)),api('/api/inbox/conversation?chat='+encodeURIComponent(chat)),api('/api/inbox/notes?chat='+encodeURIComponent(chat))]);activeInboxMeta=meta||{};const safeMessages=Array.isArray(ms)?ms:[];const safeNotes=Array.isArray(notes)?notes:[];renderInboxHead(c,activeInboxMeta);$('#messages').innerHTML=safeMessages.map(m=>`<div class="msg ${m.direction==='out'?'out':''}">${esc(m.text)}<small>${new Date(m.timestamp).toLocaleString()} · ${esc(m.status)}</small></div>`).join('');$('#messages').scrollTop=$('#messages').scrollHeight;renderInboxSide(c,activeInboxMeta,safeNotes);if(emailOnly&&inboxCanManage()){loadEmailP1D1Panel(chat);loadEmailP1D2Panel(chat);}const fresh=allConvs.find(x=>x.chat_jid===chat);if(fresh)fresh.unread=0;applyConversationFilters()}
function renderInboxHead(c,m){const h=$('#chatHead');if(!h)return;h.innerHTML=`<div class="identity"><b>${esc(c?.name||c?.phone||activeChat)}</b><small>${esc(c?.channel||'')} · ${esc(c?.phone||'')}</small></div><div class="inbox-status-row"><span class="mini-pill ${m.mode}">${m.mode==='human'?'Atención humana':'IA activa'}</span><span class="mini-pill">${esc(m.status)}</span></div><div class="inbox-head-actions">${inboxCanManage()?`<button type="button" class="inbox-quiet-btn" onclick="showInboxAudit()" title="Ver auditoría de acciones">Historial</button><button type="button" class="inbox-remove-btn" onclick="showInboxRemove()" title="Retirar conversación y conservar evidencia">Eliminar</button>`:''}</div>`;const banner=$('#inboxModeBanner');if(banner){if(c?.channel==='email'){banner.className='human-banner';banner.textContent='Email IA: envio bloqueado. Los borradores IA requieren solicitud manual y revision humana.'}else{banner.className=m.mode==='human'?'human-banner':'ai-banner';banner.textContent=m.mode==='human'?'La IA está pausada en esta conversación. El equipo humano tiene el control.':'Agentes IA habilitados. Un asesor puede tomar el control en cualquier momento.'}}}
function renderInboxSide(c,m,notes){const el=$('#inboxSide');if(!el)return;m=m||{};notes=Array.isArray(notes)?notes:[];inboxTeam=Array.isArray(inboxTeam)?inboxTeam:[];const teamOpts=['<option value="0">Sin asignar</option>',...inboxTeam.map(x=>`<option value="${x.id}" ${Number(m.assigned_user_id)===Number(x.id)?'selected':''}>${esc(x.name)} · ${esc(x.role)}</option>`)].join('');el.innerHTML=`<div class="inbox-side-heading"><span class="inbox-side-icon">⚙</span><div><h3>Gestión de conversación</h3><small>Asigna, organiza y documenta la atención</small></div></div><div class="inbox-contact-card" style="min-width:0;overflow-wrap:anywhere"><b style="overflow-wrap:anywhere;display:block">${esc(c?.name||c?.phone||'Contacto')}</b><small style="display:block;overflow-wrap:anywhere">${esc(c?.name===c?.phone?'':c?.phone||'')}${c?.name===c?.phone?'':' · '}${esc(c?.channel||'')}</small><div>${c?.message_count||0} mensajes</div></div><div class="inbox-mode"><button type="button" class="secondary ${m.mode==='ai'?'active':''}" onclick="setInboxMeta({mode:'ai'})">IA</button><button type="button" class="secondary ${m.mode==='human'?'active':''}" onclick="takeInboxConversation()">Tomar conversación</button></div><div class="field"><span>Estado</span><select id="inboxMetaStatus"><option value="open">Abierta</option><option value="pending">Pendiente</option><option value="closed">Cerrada</option></select></div><div class="field"><span>Prioridad</span><select id="inboxMetaPriority"><option value="low">Baja</option><option value="normal">Normal</option><option value="high">Alta</option><option value="urgent">Urgente</option></select></div><div class="field"><span>Responsable</span><select id="inboxMetaAssignee">${teamOpts}</select></div><div class="field"><span>Departamento</span><input id="inboxMetaDepartment" value="${esc(m.department||'')}" placeholder="Ventas, Soporte..."></div><div class="field"><span>Etiquetas</span><input id="inboxMetaTags" value="${esc(m.tags||'')}" placeholder="vip, caliente, soporte"></div><div class="field"><span>SLA</span><select id="inboxMetaSla"><option value="0">Sin cambio</option><option value="15">15 min</option><option value="30">30 min</option><option value="60">1 hora</option><option value="240">4 horas</option><option value="1440">24 horas</option></select>${m.sla_due_at?`<small class="${new Date(m.sla_due_at)<new Date()?'sla-danger':'sla-ok'}">Vence ${new Date(m.sla_due_at).toLocaleString()}</small>`:''}</div><button type="button" onclick="saveInboxMeta()">Guardar cambios de gestión</button>${inboxCanManage()?`<button type="button" class="inbox-audit-link" onclick="showInboxAudit()">↗ Ver historial de acciones</button>`:''}<hr><h3>Notas internas</h3><textarea id="inboxNoteText" rows="3" placeholder="Visible solo para el equipo"></textarea><button type="button" class="secondary" onclick="addInboxNote()">Agregar nota</button><div class="inbox-notes">${notes.length?notes.map(n=>`<div class="note-card">${esc(n.note)}<small>${esc(n.user_name||'Equipo')} · ${new Date(n.created_at).toLocaleString()}</small></div>`).join(''):'<small>Sin notas internas.</small>'}</div>`;if(c?.channel==='email'){const ai=el.querySelector('.inbox-mode button');if(ai){ai.disabled=true;ai.title='Respuestas automaticas de email no habilitadas';}if(inboxCanManage())el.insertAdjacentHTML('afterbegin','<section id="emailP1D2Panel" style="border:1px solid #dad6fb;border-radius:12px;padding:12px;margin:0 0 12px;background:rgba(111,78,255,.05)"><b>Automatización Email IA</b><p style="font-size:12px">Consultando configuración…</p></section><section id="emailP1D1Panel" style="border:1px solid #dad6fb;border-radius:12px;padding:12px;margin:0 0 12px;background:rgba(111,78,255,.05);overflow-wrap:anywhere"><b>Borrador de respuesta · Email IA</b><p style="font-size:12px">Cargando…</p></section>');}$('#inboxMetaStatus').value=m.status||'open';$('#inboxMetaPriority').value=m.priority||'normal'}
window.setInboxMeta=async function(p){if(!activeChat)return;await api('/api/inbox/conversation?chat='+encodeURIComponent(activeChat),{method:'PUT',body:JSON.stringify(p)});await openChat(activeChat);await loadInboxOverview()}
window.takeInboxConversation=async function(){const me=window.currentUser||null;await setInboxMeta({mode:'human',status:'open',assigned_user_id:me?.id||activeInboxMeta?.assigned_user_id||0})}
window.saveInboxMeta=async function(){await setInboxMeta({status:$('#inboxMetaStatus').value,priority:$('#inboxMetaPriority').value,assigned_user_id:Number($('#inboxMetaAssignee').value||0),department:$('#inboxMetaDepartment').value,tags:$('#inboxMetaTags').value,sla_minutes:Number($('#inboxMetaSla').value||0)})}
window.addInboxNote=async function(){const note=$('#inboxNoteText').value.trim();if(!note)return;await api('/api/inbox/notes?chat='+encodeURIComponent(activeChat),{method:'POST',body:JSON.stringify({note})});await openChat(activeChat)}
;['convStatusFilter','convModeFilter'].forEach(id=>document.getElementById(id)?.addEventListener('change',applyConversationFilters));
document.getElementById('clearConvFilters')?.addEventListener('click',()=>{if($('#convStatusFilter'))$('#convStatusFilter').value='all';if($('#convModeFilter'))$('#convModeFilter').value='all';setTimeout(applyConversationFilters,0)});


// V27.35.11: destructive-looking actions are soft removal only; the audit ledger
// and the original messages are preserved in the server database.
function inboxCanManage(){return ['owner','admin','superadmin'].includes(window.currentUser?.role||'')}
function inboxDialog(id,title,body,actions){
 let dialog=document.getElementById(id);
 if(!dialog){dialog=document.createElement('dialog');dialog.id=id;dialog.className='worktic-inbox-dialog';document.body.appendChild(dialog)}
 dialog.innerHTML=`<div class="inbox-dialog-shell"><div class="inbox-dialog-header"><div><small>WORKTIC · CONVERSACIONES</small><h3>${esc(title)}</h3></div><button type="button" class="inbox-close" aria-label="Cerrar" data-close>×</button></div><div class="inbox-dialog-body">${body}</div><footer class="inbox-dialog-footer">${actions}</footer></div>`;
 dialog.querySelectorAll('[data-close]').forEach(b=>b.addEventListener('click',()=>dialog.close()));
 if(!dialog.open)dialog.showModal();
 return dialog;
}
window.showInboxRemove=function(){
 if(!inboxCanManage()||!activeChat)return;
 const c=allConvs.find(v=>v.chat_jid===activeChat);
 const d=inboxDialog('inboxRemoveDialog','Eliminar conversación',`<div class="inbox-alert"><strong>¿Retirar esta conversación de la bandeja?</strong><p>Se ocultará para tu empresa. Los mensajes y la evidencia de auditoría se conservarán para revisión y restauración administrativa.</p></div><div class="inbox-dialog-contact">${esc(c?.name||c?.phone||activeChat)} <small>${esc(c?.channel||'')}</small></div><label class="inbox-audit-reason">Motivo de eliminación <span>Obligatorio</span><textarea id="inboxRemovalReason" maxlength="300" rows="3" placeholder="Ej.: conversación de prueba duplicada"></textarea></label><div id="inboxRemoveError" class="inbox-dialog-error" role="alert" hidden></div>`,`<button type="button" class="secondary" data-close>Cancelar</button><button type="button" class="inbox-danger-solid" id="inboxConfirmRemove">Eliminar de la bandeja</button>`);
 d.querySelector('#inboxConfirmRemove').addEventListener('click',async()=>{
  const reason=d.querySelector('#inboxRemovalReason').value.trim(), err=d.querySelector('#inboxRemoveError');
  if(reason.length<5){err.textContent='Escribe un motivo de al menos 5 caracteres.';err.hidden=false;return}
  const b=d.querySelector('#inboxConfirmRemove');b.disabled=true;b.textContent='Eliminando…';
  try{await api('/api/inbox/remove',{method:'POST',body:JSON.stringify({chat:activeChat,reason})});d.close();activeChat='';activeInboxMeta=null;$('#chatHead').innerHTML='Selecciona una conversación';$('#messages').innerHTML='<div class="inbox-empty-chat">Conversación retirada de la bandeja. La auditoría permanece disponible.</div>';$('#inboxSide').innerHTML='<h3>Gestión</h3><p>Selecciona una conversación.</p>';await loadConvs()}catch(e){err.textContent=e.message||'No se pudo eliminar';err.hidden=false}finally{b.disabled=false;b.textContent='Eliminar de la bandeja'}
 });
};
window.showInboxAudit=async function(chat=activeChat){
 if(!inboxCanManage()||!chat)return;
 const d=inboxDialog('inboxAuditDialog','Historial y auditoría','<p>Consultando acciones registradas…</p>','<button type="button" class="secondary" data-close>Cerrar</button>');
 try{const data=await api('/api/inbox/audit?chat='+encodeURIComponent(chat));const actions={'conversation.removed':'Eliminación','conversation.restored':'Restauración','conversation.updated':'Cambio de gestión','conversation.note_added':'Nota interna agregada'};d.querySelector('.inbox-dialog-body').innerHTML=data.length?`<div class="inbox-audit-timeline">${data.map(e=>`<div class="inbox-audit-entry"><span class="inbox-event-dot"></span><div><strong>${esc(actions[e.action]||e.action)}</strong><small>${esc(e.actor||'Usuario')} · ${esc(new Date(e.created_at).toLocaleString('es-CO'))}</small>${e.reason?`<p>Motivo: ${esc(e.reason)}</p>`:''}${e.action==='conversation.updated'?`<details><summary>Ver valores anteriores y posteriores</summary><pre>Antes: ${esc(e.before||'{}')}\nDespués: ${esc(e.after||'{}')}</pre></details>`:''}</div></div>`).join('')}</div>`:'<p>Aún no hay acciones registradas desde esta actualización.</p>'}
 catch(e){d.querySelector('.inbox-dialog-body').textContent='No se pudo consultar la auditoría: '+(e.message||'Error')}
};
window.showInboxRemoved=async function(){
 if(!inboxCanManage())return;
 const d=inboxDialog('inboxRemovedDialog','Conversaciones eliminadas','<p>Consultando conversaciones retiradas…</p>','<button type="button" class="secondary" data-close>Cerrar</button>');
 try{const rows=await api('/api/inbox/removed');d.querySelector('.inbox-dialog-body').innerHTML=rows.length?`<div class="inbox-removed-list">${rows.map((c,i)=>`<div class="inbox-removed-item"><div><strong>${esc(c.name||c.chat)}</strong><small>${esc(c.deleted_by)} · ${esc(new Date(c.deleted_at).toLocaleString('es-CO'))}</small><p>${esc(c.reason)}</p></div><div class="inbox-removed-actions"><button class="secondary" type="button" data-audit="${i}">Auditoría</button><button type="button" data-restore="${i}">Restaurar</button></div></div>`).join('')}</div>`:'<div class="inbox-empty-chat">No hay conversaciones eliminadas.</div>';
 d.querySelectorAll('[data-audit]').forEach(b=>b.addEventListener('click',()=>showInboxAudit(rows[Number(b.dataset.audit)].chat)));
 d.querySelectorAll('[data-restore]').forEach(b=>b.addEventListener('click',async()=>{
  const chat=rows[Number(b.dataset.restore)].chat;const reason=prompt('Motivo de restauración (mínimo 5 caracteres):','Revisión administrativa');if(reason===null)return;if(reason.trim().length<5){alert('Indica un motivo de al menos 5 caracteres.');return}
  b.disabled=true;try{await api('/api/inbox/restore',{method:'POST',body:JSON.stringify({chat,reason:reason.trim()})});d.close();await loadConvs()}catch(e){alert(e.message||'No se pudo restaurar')}finally{b.disabled=false}
 }));
 }catch(e){d.querySelector('.inbox-dialog-body').textContent='No se pudo consultar la lista: '+(e.message||'Error')}
};
document.getElementById('inboxDeletedBtn')?.addEventListener('click',showInboxRemoved);


// Email IA P1D.1: solicitud explicita de borrador, editable, SIN endpoint de envio.
async function loadEmailP1D1Panel(chat,agentID=0){
 const p=document.getElementById('emailP1D1Panel');if(!p||activeChat!==chat)return;
 try{
  const url='/api/email/p1/draft?chat='+encodeURIComponent(chat)+(agentID?'&agent_id='+encodeURIComponent(agentID):'');
  const resp=await fetch(url,{credentials:'same-origin',cache:'no-store'});
  if(activeChat!==chat)return;
  if(!resp.ok){p.remove();return}
  const d=await resp.json();if(!d.enabled){p.remove();return}
  const agents=Array.isArray(d.agents)?d.agents:[];
  const choices=agents.map(x=>'<option value="'+Number(x.id)+'" '+(Number(x.id)===Number(d.selected_agent_id)?'selected':'')+'>'+esc(x.name)+'</option>').join('');
  p.innerHTML='<b>Borrador IA · Revisión humana</b><p style="font-size:12px;margin:6px 0">El contenido se envía al modelo solo al pulsar Generar. No se envía ningún correo al destinatario.</p>'+
   '<label for="emailP1D1Agent" style="font-size:12px">Agente</label><select id="emailP1D1Agent" style="width:100%">'+choices+'</select>'+
   '<button type="button" id="emailP1D1Generate" style="margin:10px 0" '+(!agents.length?'disabled':'')+'>'+(d.has_draft?'Ver borrador guardado':'Generar borrador')+'</button>'+
   '<textarea id="emailP1D1Text" rows="8" maxlength="6000" placeholder="Aqui aparecera el borrador editable" style="width:100%;box-sizing:border-box;white-space:pre-wrap;resize:vertical"></textarea>'+
   '<div style="display:flex;flex-wrap:wrap;gap:6px;margin-top:8px"><button type="button" id="emailP1D1Save" class="secondary" '+(!d.has_draft?'disabled':'')+'>Guardar edicion</button><button type="button" id="emailP1D1Copy" class="secondary" '+(!d.has_draft?'disabled':'')+'>Copiar</button></div>'+
   '<small id="emailP1D1Status" role="status" aria-live="polite" style="display:block;margin-top:6px">'+(d.has_draft?'Borrador guardado. Sin envio.':'Sin borrador. Maximo 5 intentos diarios por empresa.')+'</small>';
  const ta=p.querySelector('#emailP1D1Text');ta.value=d.draft||'';
  p.querySelector('#emailP1D1Agent').onchange=e=>loadEmailP1D1Panel(chat,Number(e.target.value));
  const generate=p.querySelector('#emailP1D1Generate'),save=p.querySelector('#emailP1D1Save'),copy=p.querySelector('#emailP1D1Copy'),status=p.querySelector('#emailP1D1Status');
  generate.onclick=async()=>{
   if(!confirm('Preparar un borrador de respuesta con IA? Se procesara el contenido del correo y puede consumir creditos. No se enviara ningun email.'))return;
   generate.disabled=true;generate.textContent='Generando borrador…';status.textContent='Generando borrador con el agente seleccionado. No cierres este panel.';p.setAttribute('aria-busy','true');
   try{const ans=await api('/api/email/p1/draft',{method:'POST',body:JSON.stringify({action:'generate',chat,agent_id:Number(p.querySelector('#emailP1D1Agent').value)})});if(activeChat!==chat)return;ta.value=ans.draft||'';save.disabled=false;copy.disabled=false;generate.textContent='Borrador guardado';status.textContent='Borrador creado. Revisalo y edita antes de usarlo.'}
   catch(e){status.textContent=e.message||'No fue posible generar el borrador'}finally{generate.disabled=false;p.setAttribute('aria-busy','false');if(generate.textContent==='Generando borrador…')generate.textContent='Generar borrador'}
  };
  save.onclick=async()=>{save.disabled=true;status.textContent='Guardando…';try{await api('/api/email/p1/draft',{method:'POST',body:JSON.stringify({action:'save',chat,agent_id:Number(p.querySelector('#emailP1D1Agent').value),content:ta.value})});status.textContent='Edicion guardada. No se envio correo.'}catch(e){status.textContent=e.message||'Error guardando'}finally{save.disabled=false}};
  copy.onclick=async()=>{try{await navigator.clipboard.writeText(ta.value);status.textContent='Borrador copiado. Ningun correo enviado.'}catch(e){ta.focus();ta.select();status.textContent='Seleccionado. Usa Ctrl+C para copiar.'}};
 }catch(e){if(activeChat===chat){const p=document.getElementById('emailP1D1Panel');if(p)p.textContent='No se pudieron cargar los borradores.'}}
}

// P1D.2: activación por cuenta, solo administrador, sin envío desde el navegador.
async function loadEmailP1D2Panel(chat){
 const p=document.getElementById('emailP1D2Panel');if(!p||activeChat!==chat)return;
 try{
  const resp=await fetch('/api/email/p1/auto?chat='+encodeURIComponent(chat),{credentials:'same-origin',cache:'no-store'});
  if(activeChat!==chat)return;
  if(!resp.ok){p.remove();return}
  const d=await resp.json(),agents=Array.isArray(d.agents)?d.agents:[];
  const agentOptions=agents.map(x=>'<option value="'+Number(x.id)+'" '+(Number(x.id)===Number(d.agent_id)?'selected':'')+'>'+esc(x.name)+'</option>').join('');
  p.innerHTML='<b>Respuestas automáticas · Piloto</b>'+
   '<p style="font-size:12px">Solo buzón autorizado, correos de Ventas nuevos y destinatarios de prueba permitidos. Máximo '+Number(d.daily_limit||3)+' envíos por empresa/día. Puedes desactivar cuando quieras.</p>'+
   '<label for="emailP1D2Agent" style="font-size:12px">Agente automático</label><select id="emailP1D2Agent" style="width:100%">'+agentOptions+'</select>'+
   '<button type="button" id="emailP1D2Toggle" style="margin-top:8px">'+(d.enabled?'Desactivar respuestas automáticas':'Activar respuestas automáticas')+'</button>'+
   '<small id="emailP1D2Status" role="status" aria-live="polite" style="display:block;margin-top:8px">'+(d.enabled?'Activado para nuevos mensajes. '+(d.send_ready?'Envío habilitado.':'Envío bloqueado por configuración.'):'Desactivado: requiere autorización explícita.')+'</small>'+
   '<small style="display:block;margin-top:6px">Intentos automáticos hoy: '+Number(d.used_today||0)+' / '+Number(d.daily_limit||3)+'. Estado de este hilo: '+esc(d.last_reply?.status||'sin envíos')+(d.last_reply?.reason?' · '+esc(d.last_reply.reason):'')+'</small>';
  const btn=p.querySelector('#emailP1D2Toggle'),status=p.querySelector('#emailP1D2Status'),agent=p.querySelector('#emailP1D2Agent');
  if((!d.send_ready&&!d.enabled)||!agents.length){btn.disabled=true;status.textContent='Para habilitar: configura la bandera de envío y la lista de destinatarios de prueba en Render.'}
  btn.onclick=async()=>{
   const enable=!d.enabled;
   if(enable&&!confirm('¿Habilitar respuestas automáticas con '+agent.options[agent.selectedIndex].text+' para NUEVOS correos comerciales del buzón piloto y destinatarios permitidos? Se enviarán emails reales, sin revisión humana, con un límite diario.'))return;
   btn.disabled=true;status.textContent=enable?'Activando automatización…':'Desactivando automatización…';
   try{await api('/api/email/p1/auto',{method:'POST',body:JSON.stringify({chat,agent_id:Number(agent.value),enabled:enable})});if(activeChat===chat){status.textContent=enable?'Activada. Solo mensajes futuros; revisa el modo IA/Humano de cada conversación.':'Desactivada. No se generarán nuevas respuestas automáticas.';await openChat(chat)}}
   catch(e){status.textContent=e.message||'Error al guardar la configuración';btn.disabled=false}
  };
  // El botón IA se habilita solo cuando el servidor confirma configuración real.
  const ai=document.querySelector('#inboxSide .inbox-mode button');if(ai&&d.enabled&&d.send_ready){ai.disabled=false;ai.title='Habilitar IA para nuevos emails de esta conversación'}
 }catch(e){if(activeChat===chat){const p=document.getElementById('emailP1D2Panel');if(p)p.textContent='No se pudo consultar la automatización.'}}
}
