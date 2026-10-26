/* V27.22 - accurate, read-only plan feature status from the server. */
(()=>{'use strict';
 const el=(tag,cls,text)=>{const n=document.createElement(tag);if(cls)n.className=cls;if(text!==undefined)n.textContent=String(text);return n};
 let busy=false;
 async function refresh(){
   const root=document.getElementById('wcmFeaturesV2722');if(!root||busy)return;
   busy=true;root.replaceChildren(el('p','wcm-note','Consultando derechos del plan...'));
   try{
     const response=await fetch('/api/commercial/full-entitlements',{credentials:'same-origin',headers:{Accept:'application/json'}});
     if(!response.ok)throw Error('No se pudo consultar el plan ('+response.status+')');
     const data=await response.json();root.replaceChildren();
     const head=el('div','wcm-head');head.append(el('span','wcm-overline','WORKTICAI · PLAN Y FUNCIONALIDADES'),el('h3','','Lo que incluye tu plan'));
     const meta=el('p','wcm-note',`Plan ${data.plan_code||'—'} · ${data.module_gates_active?'Permisos aplicados en el servidor':'Vista de laboratorio: controles modulares sin activar'}`);
     root.append(head,meta);
     const grid=el('div','wcm-grid');
     for(const mod of data.modules||[]){
       const card=el('div','wcm-card');
       const line=el('div','wcm-card-head');line.append(el('strong','',mod.name||mod.key),el('span',mod.allowed?'wcm-status yes':'wcm-status no',mod.allowed?'Incluido':'No incluido'));
       card.append(line,el('p','wcm-detail',mod.metering||''));
       grid.append(card);
     }
     root.append(grid,el('p','wcm-note','Esta vista no confirma todavía las cuotas por volumen, la facturación externa ni la homologación final de todos los módulos.'));
   }catch(err){root.replaceChildren(el('p','wcm-note','No se pudieron cargar los derechos: '+err.message))}
   finally{busy=false}
 }
 function setup(){
   const billing=document.getElementById('billing');
   if(!billing)return;
   let detail=document.getElementById('wcc47PlanDiagnostics');if(!detail){detail=document.createElement('details');detail.id='wcc47PlanDiagnostics';detail.className='wcc47-diagnostics';const summary=document.createElement('summary');summary.textContent='Ver detalle técnico de funcionalidades y cuotas';detail.append(summary);billing.append(detail)}
   let root=document.getElementById('wcmFeaturesV2722');
   if(!root){root=el('section','wcm-entitlements');root.id='wcmFeaturesV2722';detail.append(root)}
   document.addEventListener('click',ev=>{const btn=ev.target.closest?.('[data-view]');if(btn&&btn.dataset.view==='billing')setTimeout(refresh,120)});
   refresh();
 }
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup);else setup();
})();
