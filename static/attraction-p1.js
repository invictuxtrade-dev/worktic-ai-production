/* Worktic AI - Atracción de Clientes P1A. Investigación guiada y CRM, no scraping. */
(()=>{
'use strict';
const root=document.getElementById('attraction-p1'); if(!root) return;
const $=(q)=>root.querySelector(q);
const $$=(q)=>Array.from(root.querySelectorAll(q));
let items=[];
function status(msg){$('#wtap1-status').textContent=msg;}
async function request(path,options){
  const r=await fetch(path,{credentials:'same-origin',headers:{'Content-Type':'application/json'},...options});
  let j={};try{j=await r.json()}catch{}
  if(!r.ok)throw new Error(j.error||`HTTP ${r.status}`);return j;
}
const append=(el,tag,txt,cls)=>{const n=document.createElement(tag);n.textContent=txt||'';if(cls)n.className=cls;el.append(n);return n;};
const safeURL=(u)=>{try{const v=new URL(u);return ['http:','https:'].includes(v.protocol)?v.href:null}catch{return null}};
function metrics(){
  $('#wtap1-total').textContent=items.length;
  $('#wtap1-pending').textContent=items.filter(x=>x.status!=='promoted').length;
  $('#wtap1-promoted').textContent=items.filter(x=>x.status==='promoted').length;
}
async function load(){
  try{const j=await request('/api/attraction/p1/candidates');items=j.items||[];metrics();render();status('Datos actualizados. Cada hallazgo pertenece a tu empresa.');}
  catch(e){status('No se pudo cargar: '+e.message);}
}
function tab(id){
  $$('[data-wtap1-tab]').forEach(b=>b.classList.toggle('is-active',b.dataset.wtap1Tab===id));
  $$('[data-wtap1-panel]').forEach(p=>p.hidden=p.dataset.wtap1Panel!==id);
  if(id==='list')render();
}
function render(){
  const out=$('#wtap1-results');out.replaceChildren();
  if(!items.length){append(out,'p','No hay hallazgos todavía. Investiga una empresa o registra una señal de comunidad.');return}
  items.forEach(x=>{
    const row=append(out,'div','','wtap1-item'); const info=append(row,'div','');
    append(info,'strong',x.name);append(info,'small',`${x.kind==='group_signal'?'Señal en grupo':'Empresa'} · ${x.niche||'Sin sector'} · ${x.city||'Sin ciudad'}${x.country?', '+x.country:''}`);
    if(x.signal)append(info,'small',x.signal.slice(0,220));
    if(x.source_url){const u=safeURL(x.source_url);if(u){let a=append(info,'a','Ver fuente pública');a.href=u;a.target='_blank';a.rel='noopener noreferrer';}}
    const actions=append(row,'div','','wtap1-item-actions');append(actions,'span',`Score ${x.score}/100`,'wtap1-score');
    const b=append(actions,'button',x.status==='promoted'?'En CRM':'Enviar al CRM');b.disabled=x.status==='promoted';
    b.addEventListener('click',async()=>{
      if(!confirm(`¿Convertir «${x.name}» en contacto y oportunidad del CRM? No se enviarán mensajes.`))return;
      b.disabled=true;
      try{await request('/api/attraction/p1/promote',{method:'POST',body:JSON.stringify({id:x.id})});await load();status('Contacto y oportunidad registrados en tu CRM.');}
      catch(e){b.disabled=false;status('No se pudo convertir: '+e.message);}
    });
  });
}
function formData(form,kind){
  const o=Object.fromEntries(new FormData(form).entries());
  Object.keys(o).forEach(k=>o[k]=String(o[k]||'').trim());o.kind=kind;if('consent_confirmed' in o)o.consent_confirmed=o.consent_confirmed==='true';return o;
}
async function save(form,kind){
  const b=form.querySelector('[type="submit"]');b.disabled=true;
  try{const obj=formData(form,kind);await request('/api/attraction/p1/candidates',{method:'POST',body:JSON.stringify(obj)});form.reset();status('Hallazgo guardado correctamente.');await load();tab('list');}
  catch(e){status('Error guardando: '+e.message)}finally{b.disabled=false}
}

function parseCSV(raw){
 const lines=[];let line=[],value='',quoted=false;
 raw=raw.replace(/^\uFEFF/,'');
 for(let i=0;i<raw.length;i++){
   const c=raw[i];
   if(quoted){if(c==='"' && raw[i+1]==='"'){value+='"';i++;}else if(c==='"')quoted=false;else value+=c;}
   else if(c==='"') quoted=true;
   else if(c===',' || c===';' ){line.push(value);value='';}
   else if(c==='\n'){line.push(value.replace(/\r$/,''));if(line.some(v=>v.trim()))lines.push(line);line=[];value='';}
   else value+=c;
 }
 line.push(value.replace(/\r$/,''));if(line.some(v=>v.trim()))lines.push(line);
 if(!lines.length)return [];
 const headers=lines.shift().map(v=>v.trim().toLowerCase());
 return lines.filter(row=>row.some(v=>v.trim())).map(row=>{
   const data={kind:'business'};headers.forEach((h,i)=>{
     const key=({'empresa':'name','nombre':'name','ciudad':'city','pais':'country','país':'country','sector':'niche','sitio_web':'website','correo':'email','telefono':'phone','teléfono':'phone','fuente':'source_url','señal':'signal'}[h]||h);
     if(['name','city','country','niche','website','email','phone','source_url','signal'].includes(key))data[key]=String(row[i]||'').trim();
   });return data;
 });
}
$('#wtap1-import').addEventListener('click',async()=>{
 const file=$('#wtap1-csv').files[0];if(!file){status('Selecciona un archivo CSV.');return}
 if(file.size>512000){status('El archivo supera 500 KB.');return}
 let rows=[];try{rows=parseCSV(await file.text());}catch(e){status('CSV inválido: '+e.message);return}
 if(!rows.length||rows.length>50||rows.some(x=>!x.name)){status('CSV: requiere 1 a 50 empresas con columna name (o empresa).');return}
 const btn=$('#wtap1-import');btn.disabled=true;let saved=0;let errors=0;
 try{
   for(const row of rows){try{await request('/api/attraction/p1/candidates',{method:'POST',body:JSON.stringify(row)});saved++;}catch{errors++;}}
   await load();status(`Importación completada: ${saved} registros aceptados y ${errors} con error.`);tab('list');
 }finally{btn.disabled=false}
});

$('#wtap1-business-form').addEventListener('submit',e=>{e.preventDefault();save(e.currentTarget,'business')});
$('#wtap1-group-form').addEventListener('submit',e=>{e.preventDefault();save(e.currentTarget,'group_signal')});
$$('[data-wtap1-tab]').forEach(b=>b.addEventListener('click',()=>tab(b.dataset.wtap1Tab)));
$('#wtap1-reload').addEventListener('click',load);
$('#wtap1-search-business').addEventListener('click',()=>{
  const niche=$('#wtap1-business-niche').value.trim(),city=$('#wtap1-business-city').value.trim();
  if(!niche){status('Indica el sector o nicho para investigar.');return}
  window.open('https://www.google.com/search?q='+encodeURIComponent([niche,city,'empresas sitio oficial'].filter(Boolean).join(' ')),'_blank','noopener,noreferrer');
  const form=$('#wtap1-business-form');form.elements.niche.value=niche;form.elements.city.value=city;
  status('Explora los resultados públicos y guarda solamente empresas verificadas.');
});
$('#wtap1-discover-groups').addEventListener('click',async()=>{
  const niche=$('#wtap1-group-niche').value.trim();if(!niche){status('Escribe el nicho que deseas investigar.');return}
  const out=$('#wtap1-group-links');out.replaceChildren();
  try{const j=await request('/api/groups/discovery?niche='+encodeURIComponent(niche));
    (j.results||[]).forEach(x=>{const u=safeURL(x.url);if(u){const a=append(out,'a',x.label||x.platform);a.href=u;a.target='_blank';a.rel='noopener noreferrer';}});
    const form=$('#wtap1-group-form');form.elements.niche.value=niche;status('Consulta las fuentes públicas y documenta oportunidades verificadas.');
  }catch(e){status('No se pudo explorar: '+e.message)}
});
$('#wtap1-open-groups').addEventListener('click',()=>document.querySelector('#appSidebar button[data-view="groups"]')?.click());
const nav=document.querySelector('#appSidebar button[data-view="attraction-p1"]');
if(nav)nav.addEventListener('click',()=>{
  requestAnimationFrame(()=>{const title=document.getElementById('title'),sub=document.getElementById('subtitle');if(title)title.textContent='Atracción de Clientes';if(sub)sub.textContent='Prospección comercial y oportunidades en comunidades';});
  load();
});
window.addEventListener('worktic:langchange',()=>{if(root.classList.contains('active')){const title=document.getElementById('title');if(title)title.textContent='Atracción de Clientes';}});
})();
