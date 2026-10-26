/* V27.27 · progressive superadmin preflight, read-only. No price/vendor actions. */
(()=>{'use strict';
const make=(tag,cls,text)=>{const n=document.createElement(tag);if(cls)n.className=cls;if(text!==undefined)n.textContent=String(text);return n;};
const labels={pass:'Comprobación local',block:'Bloqueo',manual:'Evidencia pendiente'};
function install(){const admin=document.getElementById('admin');if(!admin||document.getElementById('wcv2727'))return;
 const root=make('section','wcv-root');root.id='wcv2727';
 const head=make('div','wcv-head'),group=make('div');group.append(make('small','','WORKTICAI V27.27 · STAGING'),make('h2','','Validación comercial integral'),make('p','','Panel de auditoría de solo lectura. No activa funciones, publica precios ni valida facturas automáticamente.'));
 const action=make('button','wcv-refresh','Ejecutar preflight');action.type='button';head.append(group,action);root.append(head);
 const status=make('p','wcv-note','Ejecuta la validación para comprobar configuraciones y bloqueos.');root.append(status);
 const overview=make('div','wcv-counts');root.append(overview);
 const list=make('div','wcv-results');root.append(list);
 async function load(){action.disabled=true;status.textContent='Consultando el backend…';overview.replaceChildren();list.replaceChildren();try{
  const res=await fetch('/api/admin/commercial/v2727/preflight',{credentials:'same-origin',headers:{Accept:'application/json'},cache:'no-store'});
  if(!res.ok)throw Error(`HTTP ${res.status} · solo superadministrador o backend no disponible`);
  const data=await res.json();
  for(const [name,count]of [['Correctas',data.summary?.pass],['Bloqueos',data.summary?.block],['Pendientes',data.summary?.manual]]){const card=make('div','wcv-count');card.append(make('small','',name),make('strong','',count??'—'));overview.append(card)}
  const byMod=new Map();for(const c of data.checks||[]){const arr=byMod.get(c.module)||[];arr.push(c);byMod.set(c.module,arr)}
  for(const [mod,items]of byMod){const sec=make('section','wcv-module');sec.append(make('h3','',mod));for(const c of items){const row=make('div','wcv-item'),badge=make('span',`wcv-pill wcv-${c.status}`,labels[c.status]||'Pendiente');row.append(badge,make('p','',c.detail));sec.append(row)}list.append(sec)}
  // V27.28: separate read-only payment lifecycle check. Never auto-approves.
  try{const lr=await fetch('/api/admin/billing/v2728/audit',{credentials:'same-origin',cache:'no-store'});
    if(lr.ok){const audit=await lr.json();const sec=make('section','wcv-module');sec.append(make('h3','','Pagos, renovaciones y créditos · V27.28'));
      for(const [key,value] of Object.entries(audit.metrics||{})){const row=make('div','wcv-item');row.append(make('span','wcv-pill',key.replaceAll('_',' ')),make('p','',value));sec.append(row)}
      sec.append(make('p','','Solo auditoría. No autoriza ventas ni verifica blockchain.'));list.prepend(sec)}
  }catch(_ignored){}
  status.textContent='VENTA PÚBLICA: NO APROBADA. Cada hallazgo externo necesita evidencia de staging y conciliación. Fecha UTC: '+(data.checked_at_utc||'—');
 }catch(e){status.textContent='Auditoría no disponible: '+e.message}finally{action.disabled=false}}
 action.addEventListener('click',load);admin.append(root);
 document.addEventListener('click',e=>{const btn=e.target.closest?.('[data-view="admin"]');if(btn)setTimeout(load,450)});
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',install);else install();
})();
