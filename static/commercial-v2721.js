/* WorkticAI Commercial Studio V27.21 - first-party progressive enhancement.
 * Server owns permissions, reservations, wallet balances and payment approval. */
(()=>{'use strict';
 const h=(tag,cls,text)=>{const e=document.createElement(tag);if(cls)e.className=cls;if(text!==undefined)e.textContent=String(text);return e};
 const api=async(url,body,method)=>{const opt={credentials:'same-origin',headers:{'Accept':'application/json'}};if(body!==undefined){opt.method=method||'POST';opt.headers['Content-Type']='application/json';opt.body=JSON.stringify(body)}const r=await fetch(url,opt);if(!r.ok){const t=await r.text();throw Error((t||`Error HTTP ${r.status}`).slice(0,210))}return r.json()};
 const money=c=>(Number(c||0)/100).toLocaleString('es-CO',{style:'currency',currency:'USD'});
 const state={catalog:null,wallet:null};
 function field(label,type,value){const wrap=h('label','wcc-field');wrap.append(h('span','',label));const input=h('input','');input.type=type||'text';if(value!==undefined)input.value=value;wrap.append(input);return {wrap,input}}
 function notice(target,msg,bad){target.replaceChildren(h('p',bad?'wcc-bad':'wcc-good',msg))}
 function row(label,value){const p=h('div','wcc-line');p.append(h('span','',label),h('b','',value));return p}
 async function wallet(){
  const el=document.getElementById('wccWalletV2721');if(!el)return;
  el.replaceChildren(h('p','wcc-muted','Consultando créditos…'));
  try{
   const [d,cat]=await Promise.all([api('/api/commercial/wallet'),api('/api/commercial/catalog')]);state.wallet=d;state.catalog=cat;
   el.replaceChildren();el.append(h('div','wcc-small','PLAN '+d.plan.toUpperCase()+' · '+d.period));
   const nums=h('div','wcc-balance-row');const total=h('div','wcc-balance');total.append(h('span','', 'Disponible'),h('strong','', d.available_credits+' créditos'));nums.append(total);
   const pack=h('div','wcc-balance');pack.append(h('span','','Créditos comprados'),h('strong','',d.purchased_available));nums.append(pack);el.append(nums);
   el.append(row('Créditos incluidos en el plan',d.monthly_grant),row('Consumidos este mes',d.plan_spent),row('Reservados (generaciones en curso)',d.plan_reserved),row('Saldo comprado disponible',d.purchased_available));
   const equivalent=h('p','wcc-muted',`Referencias: ${Math.floor(Number(d.monthly_grant||0)/4)} imágenes o ${Math.floor(Number(d.monthly_grant||0)/15)} videos IA de 5 s si usaras todos tus créditos en una sola categoría. Las combinaciones consumen el mismo saldo.`);el.append(equivalent);el.append(row('Respuestas IA generales incluidas', d.core_ai_monthly_limit||0),row('Respuestas IA utilizadas este mes',d.core_ai_used||0),row('Respuestas IA disponibles',d.core_ai_available||0));
   const rule=d.rule;const caps=h('div','wcc-capabilities');
   for(const [label,yes] of [['Editor manual',rule.editor],['Imágenes IA',rule.image_ai],['Videos IA',rule.video_ai],['Textos IA',rule.text_ai],['Voz IA',rule.voice_ai]]){
     caps.append(h('span',yes?'wcc-feature-yes':'wcc-feature-no',(yes?'✓ ':'× ')+label));
   }
   el.append(caps);el.append(h('p','wcc-muted',`Hasta ${rule.max_projects} proyectos · Almacenamiento de referencia: ${rule.storage_mb} MB. Las cuotas de almacenamiento requieren comprobación integral antes del lanzamiento.`));
   el.append(h('p','wcc-muted',d.enabled?'Las operaciones externas autorizadas consumen primero los créditos del plan y después los comprados.':'La generación IA aún requiere activación técnica y presupuesto del proveedor; no se consumen créditos mientras esté deshabilitada.'));
   buildStore(el,cat,d);
  }catch(e){notice(el,'Billetera temporalmente no disponible: '+e.message,true)}
 }
 function buildStore(root,cat,d){
   const box=h('div','wcc-shop');box.append(h('h3','','Ampliar mis créditos'),h('p','wcc-muted','Selecciona un paquete. Tu pago se acredita únicamente después de que administración compruebe el ingreso de USDT.'));
   const cards=h('div','wcc-package-grid');let selected=cat.packages?.[0]?.code;
   for(const p of cat.packages||[]){const b=h('button','wcc-pack');b.type='button';b.append(h('b','',p.name),h('strong','',p.credits+' créditos'),h('span','',money(p.price_cents)));if(p.code===selected)b.classList.add('is-active');
     b.addEventListener('click',()=>{selected=p.code;cards.querySelectorAll('.wcc-pack').forEach(x=>x.classList.remove('is-active'));b.classList.add('is-active')});cards.append(b)}box.append(cards);
   const form=h('form','wcc-topup-form');const payNetwork=h('label','wcc-field');payNetwork.append(h('span','','Red USDT'));const select=h('select','');for(const net of ['BEP20','TRC20']){const o=h('option','',net);o.value=net;select.append(o)}payNetwork.append(select);form.append(payNetwork);
   const tx=field('Hash de transacción confirmado en blockchain','text','');tx.input.placeholder='0x... (BEP20) o hash TRC20';tx.input.maxLength=70;form.append(tx.wrap);
   const address=h('div','wcc-muted','Consulta la dirección de pago en Membresía/Planes. Comprueba la red, el token y el importe antes de enviar.');form.append(address);
   const feedback=h('div');form.append(feedback);
   const submit=h('button','wcc-buy','Registrar pago y solicitar verificación');submit.type='submit';submit.disabled=!d.topups_enabled||!selected;form.append(submit);
   form.addEventListener('submit',async ev=>{ev.preventDefault();if(!window.confirm('Confirma que ya transferiste exactamente el USDT requerido a la dirección oficial de tu plataforma. ¿Registrar el comprobante?'))return;submit.disabled=true;
     try{const out=await api('/api/commercial/topups',{package:selected,network:select.value,tx_hash:tx.input.value.trim()});notice(feedback,`Solicitud ${out.id} pendiente de verificación. El saldo no cambia hasta la aprobación.`,false);tx.input.value='';}
     catch(e){notice(feedback,e.message,true)}finally{submit.disabled=!d.enabled}});
   box.append(form);if(!d.topups_enabled)box.append(h('p','wcc-warn','La compra de recargas requiere habilitación y verificación de pagos por separado.'));
   root.append(box);
   const trans=h('details','wcc-history');trans.append(h('summary','','Mis recargas y verificaciones'));const body=h('div','wcc-muted','Consultar historial…');trans.append(body);trans.addEventListener('toggle',async()=>{if(!trans.open)return;try{const data=await api('/api/commercial/topups');body.replaceChildren();for(const x of data.topups){body.append(row(`${x.package} · ${x.credits} créditos`,`${money(x.price_cents)} · ${x.status}`))}if(!data.topups.length)body.textContent='Todavía no hay recargas'}catch(e){body.textContent='Error consultando historial: '+e.message}});root.append(trans);
 }
 async function admin(){const el=document.getElementById('wccCommercialAdmin');if(!el)return;el.replaceChildren(h('p','wcc-muted','Cargando matriz comercial y solicitudes…'));
  try{
    const [cat,rs,ts]=await Promise.all([api('/api/commercial/catalog'),api('/api/admin/commercial/rules'),api('/api/admin/commercial/topups')]);el.replaceChildren();
    const summary=h('div','wcc-com-summary');summary.append(h('h3','','Matriz de planes · Derechos y restricciones'),h('p','wcc-muted','Puedes ajustar los créditos mensuales de Personal, Negocio y Empresa. Los cambios se guardan en PostgreSQL y se reflejan en la web y checkout. Las generaciones IA también requieren presupuestos de proveedor y activación técnica independiente.'));summary.append(h('p',cat.studio_credits_enabled?'wcc-good':'wcc-warn',cat.studio_credits_enabled?'Débito de créditos Studio habilitado.':'Débito de créditos Studio pendiente de activación técnica en Render.'));el.append(summary);
    const cards=h('div','wcc-commercial-plans');for(const r of rs.rules){const price=(cat.plans||[]).find(x=>x.code===r.plan);const card=h('form','wcc-com-plan');card.append(h('h4','',`${price?.name||r.plan} · ${price?.price_usdt||0} USDT/mes`));
      const credits=field('Créditos Studio / mes','number',r.monthly_credits);credits.input.min='0';credits.input.max='100000';card.append(credits.wrap);
      const equivalent=h('p','wcc-muted');const refreshEq=()=>{const n=Math.max(0,Number(credits.input.value||0));equivalent.textContent=`Aprox. ${Math.floor(n/4)} imágenes IA o ${Math.floor(n/15)} clips IA de 5 s, usando todo el saldo (4 y 15 créditos respectivamente).`};credits.input.addEventListener('input',refreshEq);refreshEq();card.append(equivalent);
      const projects=field('Máx. proyectos','number',r.max_projects);projects.input.min='0';card.append(projects.wrap);
      const storage=field('Almacenamiento MB (propuesta)','number',r.storage_mb);storage.input.min='0';card.append(storage.wrap);
      const flags=h('div','wcc-toggle-grid');const checks={};for(const [prop,label] of [['editor','Editor'],['image_ai','Imagen IA'],['video_ai','Video IA'],['text_ai','Textos IA'],['voice_ai','Voz IA']]){const chk=h('input');chk.type='checkbox';chk.checked=Boolean(r[prop]);const labelEl=h('label','wcc-check');labelEl.append(chk,h('span','',label));flags.append(labelEl);checks[prop]=chk}card.append(flags);
      const feedback=h('div','wcc-msg');card.append(feedback);const save=h('button','wcc-admin-save','Guardar restricciones');save.type='submit';card.append(save);
      card.addEventListener('submit',async e=>{e.preventDefault();save.disabled=true;try{const payload={plan:r.plan,monthly_credits:Number(credits.input.value),max_projects:Number(projects.input.value),storage_mb:Number(storage.input.value)};for(const [k,ch]of Object.entries(checks))payload[k]=ch.checked;await api('/api/admin/commercial/rules',payload);notice(feedback,'Reglas guardadas. Se aplicarán a solicitudes nuevas.',false)}catch(ex){notice(feedback,ex.message,true)}finally{save.disabled=false}});
      cards.append(card)}el.append(cards);
    const top=h('section','wcc-admin-topups');top.append(h('h3','','Recargas recibidas'));top.append(h('p','wcc-muted','No apruebes ninguna recarga sin verificar USDT, importe exacto, red, destino, confirmaciones y ausencia de reutilización del hash.'));
    const list=h('div','wcc-topup-list');for(const x of ts.topups){const item=h('article','wcc-topup');item.append(h('b','',`Empresa #${x.tenant_id} · ${x.credits} créditos · ${money(x.price_cents)}`),h('span','',`${x.network} · ${x.status}`),h('code','',x.tx_hash));
      if(x.status==='pending'){const actions=h('div','wcc-actions');for(const action of ['approve','reject']){const b=h('button',action==='approve'?'wcc-approve':'wcc-reject',action==='approve'?'Aprobar pago comprobado':'Rechazar');b.type='button';b.addEventListener('click',async()=>{
          if(!window.confirm(action==='approve'?'¿Has comprobado independientemente en blockchain la transacción, importe USDT y destinatario? Se acreditarán créditos reales.':'¿Confirmas rechazar la solicitud?'))return;
          b.disabled=true;try{await api('/api/admin/commercial/topups',{id:x.id,action,verified_on_chain:action==='approve'});await admin()}catch(e){window.alert(e.message);b.disabled=false}
        });actions.append(b)}item.append(actions)}list.append(item)}if(!ts.topups?.length)list.append(h('p','wcc-muted','Todavía no hay recargas registradas.'));top.append(list);el.append(top);
  }catch(e){notice(el,'No fue posible cargar administración comercial: '+e.message,true)}
 }
 function setup(){
  const s=document.querySelector('#studio .wts-app')||document.getElementById('studio');if(s&&!document.getElementById('wccCommercialSection')){
    const el=h('section','wcc-wallet');el.id='wccCommercialSection';el.append(h('div','wcc-wallet-head'),h('div',''));const head=el.firstElementChild;head.append(h('div','wcc-small','WORKTIC STUDIO · CREATIVE CREDITS'),h('h2','','Mi billetera creativa'));
    const refresh=h('button','wcc-refresh','Actualizar saldo');refresh.type='button';refresh.addEventListener('click',wallet);head.append(refresh);const body=el.lastElementChild;body.id='wccWalletV2721';body.textContent='Abre Worktic Studio para ver tus créditos.';s.append(el)
  }
  const adm=document.getElementById('admin');if(adm&&!document.getElementById('wccCommercialAdmin')){
    const sec=h('section','wcc-admin commercial-v2721');sec.append(h('div','wcc-title'),h('div',''));sec.firstElementChild.append(h('div','wcc-small','V27.21 · PLANES Y MONETIZACIÓN'),h('h2','','Commercial Control Center'));
    const refresh=h('button','wcc-refresh','Actualizar administración');refresh.type='button';refresh.addEventListener('click',admin);sec.firstElementChild.append(refresh);sec.lastElementChild.id='wccCommercialAdmin';adm.append(sec)
  }
  document.addEventListener('click',e=>{const b=e.target.closest('[data-view]');if(!b)return;if(b.dataset.view==='studio')setTimeout(wallet,350);if(b.dataset.view==='admin')setTimeout(admin,350)});
  const old=window.WorkticCostConfirm;window.WorkticCostConfirm=async(action,extra)=>{
    try{const q=new URLSearchParams({action,duration:String(extra?.duration||0),chars:String(extra?.chars||0)});const data=await api('/api/commercial/quote?'+q);
      if(!data.allowed_by_plan_and_cost){alert('Esta generación no está permitida por tu plan o cotización.');return false}
      const status=await api('/api/commercial/wallet');if(status.enabled&&status.available_credits<data.credits){alert(`Saldo insuficiente: necesitas ${data.credits} créditos y tienes ${status.available_credits}.`);return false}
      return confirm(`WorkticAI · Generación con IA\n\nCréditos a consumir: ${data.credits}\nCosto estimado del proveedor: ${money(data.estimated_vendor_cost_cents)}\nSaldo de tu billetera: ${status.available_credits} créditos\n\n${status.enabled?'Los créditos se reservarán al solicitar la generación.':'Piloto: el débito comercial todavía está desactivado.'}\n¿Confirmar?`);
    }catch(e){alert('No se pudo cotizar la generación: '+e.message);return false}
  };
 }
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup);else setup();
})();
