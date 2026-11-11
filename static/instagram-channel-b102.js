/* Worktic AI P1A.4: Instagram como canal dentro del listado unificado.
   Mantiene el endpoint oficial y la asignación de agente; NO activa respuestas automáticas. */
(()=>{'use strict';
 const e=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 // P1A4_INSTAGRAM_IN_GRID: el render y paginador pertenecen a channels-unified.
 function card(x,agentLookup){
  const id=Number(x.id),account=e(x.account_name||x.name||''),current=Number(x.agent_id||0);
  const agents=[...(agentLookup||new Map()).entries()].filter(([aid,a])=>a.status==='active'||aid===current);
  const options='<option value="0">Sin agente asignado</option>'+agents.map(([aid,a])=>'<option value="'+aid+'" '+(aid===current?'selected':'')+'>'+e(a.name)+'</option>').join('');
  return '<article class="card wt-unified-card wt-b102-card" data-ig-id="'+id+'">'+
  '<div class="wt-b102-heading"><div class="wt-b102-icon">◎</div><div><b>INSTAGRAM</b><small>Canal oficial de Meta · Cuenta de redes sociales</small></div><span class="wt-b102-pill">● Conectado</span></div>'+
  '<h3>@'+account+'</h3><div class="wt-b102-account"><small>CUENTA VINCULADA</small><strong>'+account+'</strong><small>ID: '+e(x.external_account_id)+'</small></div>'+
  '<label class="wt-b102-label">Agente IA previsto<select data-b102-agent>'+options+'</select></label>'+
  '<div class="wt-b102-warning">Atención IA pendiente de completar webhooks y envío de mensajes. Asignar un agente <b>no</b> activa respuestas automáticas.</div>'+
  '<div class="wt-b102-actions"><button type="button" data-b102-save>Guardar asignación</button><button type="button" class="secondary" data-b102-social>Crear publicación</button></div></article>';
 }
 document.addEventListener('click',async ev=>{
  const social=ev.target.closest('[data-b102-social]');
  if(social){const nav=document.querySelector('aside nav button[data-view="socialhub"]');nav?.click();setTimeout(()=>{if(typeof window.switchSocialTab==='function')window.switchSocialTab('composer');else document.querySelector('[data-social-tab="composer"]')?.click()},180);return;}
  const save=ev.target.closest('[data-b102-save]');if(!save)return;
  const host=save.closest('[data-ig-id]');const id=Number(host?.dataset.igId),agent=Number(host?.querySelector('[data-b102-agent]')?.value);
  if(!Number.isSafeInteger(id)||id<1||!Number.isSafeInteger(agent)||agent<0)return;
  save.disabled=true;
  try{await window.api('/api/social/b10/instagram-channel',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({social_connection_id:id,agent_id:agent})});(window.toast||alert)('Asignación guardada. La atención automática de Instagram continúa desactivada.');await window.workticUnified?.refresh?.();}
  catch(err){(window.toast||alert)(err.message||'No se pudo guardar la asignación');save.disabled=false;}
 });
 window.workticInstagramB102={card,refresh:()=>window.workticUnified?.refresh?.()};
})();
