/* Worktic AI — Atracción de Clientes Autopilot P1B. Sin carga manual. */
(()=>{
'use strict';
const root=document.getElementById('attraction-p1');if(!root)return;
const $=(selector)=>root.querySelector(selector);
const $$=(selector)=>Array.from(root.querySelectorAll(selector));
let candidates=[],campaigns=[],config=null;
function status(s){$('#wtap1-status').textContent=s}
function publicError(value){return String(value||'Error desconocido').replace(/OPENAI_API_KEY/gi,'credenciales del motor IA').replace(/openai/gi,'motor IA').replace(/\bgpt-[\w.-]+/gi,'modelo interno de IA')}
function append(parent,tag,label,cls){const node=document.createElement(tag);node.textContent=label||'';if(cls)node.className=cls;parent.append(node);return node}
function safeURL(s){try{const u=new URL(s);if(!['https:','http:'].includes(u.protocol))return null;return u.href}catch{return null}}
async function request(path,opts={}){
 const res=await fetch(path,{credentials:'same-origin',headers:{'Content-Type':'application/json'},...opts});
 let data={};try{data=await res.json()}catch{}
 if(!res.ok)throw new Error(data.error||data.message||`HTTP ${res.status}`);
 return data;
}
function metrics(){
 $('#wtap1-total').textContent=candidates.length;
 $('#wtap1-pending').textContent=candidates.filter(x=>x.status!=='promoted').length;
 $('#wtap1-promoted').textContent=candidates.filter(x=>x.status==='promoted').length;
}
function timeLabel(iso){if(!iso)return 'Sin ejecutar';const d=new Date(iso);return isNaN(d)?'Por determinar':d.toLocaleString('es-CO')}
function renderCampaigns(){
 const out=$('#wtap1-campaigns');out.replaceChildren();
 if(!campaigns.length){append(out,'p','No hay campañas aún. Crea la primera y la IA comenzará a investigar.');return}
 for(const c of campaigns){
  const row=append(out,'div','','wtap1-item');const info=append(row,'div','');
  append(info,'strong',`${c.sector} · ${c.city}, ${c.country}`);
  const mode={'business':'Empresas B2B','community':'Comunidades públicas','both':'B2B + comunidades'}[c.mode]||c.mode;
  append(info,'small',`${mode} · Cada ${c.interval_hours} h · ${c.enabled?'Activa':'Pausada'}`);
  append(info,'small',`Último ciclo: ${timeLabel(c.last_run)} · Próximo: ${timeLabel(c.next_run)}`);
  append(info,'small',`Estado: ${c.last_status||'pendiente'} · Nuevos hallazgos: ${c.last_found||0}`);
  if(c.last_error)append(info,'small',publicError(c.last_error));
  const a=append(row,'div','','wtap1-item-actions');
  const toggle=append(a,'button',c.enabled?'Pausar':'Reanudar','wtap1-secondary');
  toggle.addEventListener('click',async()=>{
   toggle.disabled=true;
   try{await request('/api/attraction/p1b/control',{method:'POST',body:JSON.stringify({id:c.id,action:c.enabled?'pause':'resume'})});await load();status('Campaña actualizada correctamente.');}
   catch(e){toggle.disabled=false;status(publicError(e.message))}
  });
 }
}
function renderCandidates(){
 const out=$('#wtap1-results');out.replaceChildren();
 if(!candidates.length){append(out,'p','El agente todavía no encontró oportunidades respaldadas por fuentes públicas. La primera campaña se ejecutará automáticamente.');return}
 for(const x of candidates){
  const row=append(out,'div','','wtap1-item');const info=append(row,'div','');
  append(info,'strong',x.name);append(info,'small',`${x.kind==='group_signal'?'Comunidad pública':'Empresa'} · ${x.niche||''} · ${x.city||''} ${x.country||''}`);
  if(x.signal)append(info,'small',x.signal);
  if(x.source_url){const u=safeURL(x.source_url);if(u){const a=append(info,'a','Abrir fuente verificada');a.href=u;a.target='_blank';a.rel='noopener noreferrer'}}
  const actions=append(row,'div','','wtap1-item-actions');
  append(actions,'span',`Score ${x.score}/100`,'wtap1-score');
  append(actions,'span',x.status==='promoted'?'En CRM':'En investigación','wtap1-state');
 }
}
async function load(){
 try{
  const r=await Promise.allSettled([request('/api/attraction/p1b/config'),request('/api/attraction/p1b/campaigns'),request('/api/attraction/p1/candidates')]);
  if(r[0].status==='fulfilled'){config=r[0].value;let ready=config.enabled&&config.key_configured&&config.worker_configured;
   $('#wtap1-health').textContent=ready?`Motor de Atracción IA activo · Hasta ${config.daily_run_limit} investigaciones diarias`:'Motor de Atracción IA no disponible. Contacta al administrador de Worktic AI.';
   $('#wtap1-create').disabled=!ready;
  }
  if(r[1].status==='fulfilled')campaigns=r[1].value.items||[];
  if(r[2].status==='fulfilled')candidates=r[2].value.items||[];
  metrics();renderCampaigns();renderCandidates();
  const err=r.find(x=>x.status==='rejected');status(err?'No se pudo completar la actualización: '+publicError(err.reason.message):'Autopilot IA actualizado. No se envían mensajes a prospectos.');
 }catch(e){status('No se pudo actualizar el módulo: '+publicError(e.message))}
}
function tab(key){
 $$('[data-wtap1-tab]').forEach(b=>b.classList.toggle('is-active',b.dataset.wtap1Tab===key));
 $$('[data-wtap1-panel]').forEach(p=>p.hidden=p.dataset.wtap1Panel!==key);
 if(key==='list')renderCandidates();
}
$('#wtap1-campaign-form').addEventListener('submit',async e=>{
 e.preventDefault();if(!config?.enabled)return status('Esta función no está disponible por el momento. Contacta al administrador.');
 const form=e.currentTarget,btn=$('#wtap1-create');btn.disabled=true;
 const x=Object.fromEntries(new FormData(form).entries());x.interval_hours=Number(x.interval_hours)||24;
 try{await request('/api/attraction/p1b/campaigns',{method:'POST',body:JSON.stringify(x)});form.reset();await load();status('Campaña creada. El servidor investigará automáticamente las fuentes públicas.');}
 catch(err){status('No se pudo crear la campaña: '+publicError(err.message))}finally{btn.disabled=false}
});
$$('[data-wtap1-tab]').forEach(b=>b.addEventListener('click',()=>tab(b.dataset.wtap1Tab)));
$('#wtap1-refresh').addEventListener('click',load);
$('#wtap1-reload').addEventListener('click',load);
const nav=document.querySelector('#appSidebar button[data-view="attraction-p1"]');
if(nav)nav.addEventListener('click',()=>{
 requestAnimationFrame(()=>{const title=document.getElementById('title'),sub=document.getElementById('subtitle');if(title)title.textContent='Atracción de Clientes';if(sub)sub.textContent='Prospección autónoma impulsada por Worktic AI';});
 load();
});
window.setInterval(()=>{if(root.classList.contains('active')&&document.visibilityState==='visible')load()},30000);
})();
