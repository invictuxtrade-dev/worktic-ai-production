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
   if(window.WorkticWalletV27346?.render){window.WorkticWalletV27346.render(el,d,cat,{buildStore,api,wallet});return;}
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
   // V27.34.9 Lite: un solo recorrido, importes oficiales del catálogo y pago verificado en el servidor.
   const box=h('section','wcc49-shop');root.append(box);
   const packs=(Array.isArray(cat?.packages)?cat.packages:[]).filter(p=>p&&p.code&&p.active!==false&&Number(p.credits)>0);
   const head=h('div','wcc49-heading');head.append(h('h3','','Compra créditos en 3 pasos'),h('p','', 'Elige un paquete, realiza la transferencia y envía el comprobante. Acreditación tras verificación.'));box.append(head);
   if(!d.topups_enabled){
     const off=h('div','wcc49-state');off.append(h('strong','','Recargas temporalmente desactivadas'),h('p','','Tu saldo actual y los créditos de tu plan se mantienen. La tienda estará disponible cuando Administración termine su habilitación segura.'));box.append(off);
     if(packs.length){const line=h('p','wcc49-pack-preview','Paquetes previstos: '+packs.map(p=>`${p.credits} créditos por ${money(p.price_cents)}`).join(' · '));box.append(line)}
     return;
   }
   if(!packs.length){box.append(h('p','wcc49-state','Actualmente no hay paquetes activos.'));return}
   const form=h('form','wcc49-form');form.noValidate=true;box.append(form);
   const main=h('div','wcc49-first');form.append(main);
   const pick=h('label','wcc49-label');pick.append(h('span','','1. Paquete de créditos'));const selectPack=h('select','wcc49-select');for(const p of packs){const op=h('option','',`${p.name||p.code} · ${p.credits} créditos · ${money(p.price_cents)}`);op.value=p.code;selectPack.append(op)}pick.append(selectPack);main.append(pick);
   const summary=h('div','wcc49-amount');main.append(summary);
   const detail=h('details','wcc49-details');const summaryNode=h('summary','','2. Datos para pagar con USDT');detail.append(summaryNode);form.append(detail);
   const detailBody=h('div','wcc49-detail-body');detail.append(detailBody);
   const netLabel=h('label','wcc49-label');netLabel.append(h('span','','Red de pago'));const network=h('select','wcc49-select');for(const [n,t] of [['BEP20','BNB Smart Chain (BEP20)'],['TRC20','Tron (TRC20)']]){const op=h('option','',t);op.value=n;network.append(op)}netLabel.append(network);detailBody.append(netLabel);
   const address=h('div','wcc49-address');const addressTitle=h('span','','Dirección oficial de pago');const addressValue=h('code','','Consultando…');const copy=h('button','wcc49-copy','Copiar');copy.type='button';copy.disabled=true;address.append(addressTitle,addressValue,copy);detailBody.append(address);
   const checkLabel=h('label','wcc49-verify');const acknowledged=h('input');acknowledged.type='checkbox';checkLabel.append(acknowledged,h('span','','Verifiqué la red, token USDT, dirección e importe antes de enviar.'));detailBody.append(checkLabel);
   const hashLabel=h('label','wcc49-label');hashLabel.append(h('span','','3. Hash de la transacción'));const tx=h('input','wcc49-select');tx.type='text';tx.autocomplete='off';tx.placeholder='Pega el hash después de enviar el pago';tx.maxLength=66;hashLabel.append(tx);detailBody.append(hashLabel);
   const footer=h('div','wcc49-footer');const feedback=h('div','wcc49-feedback');feedback.setAttribute('aria-live','polite');const submit=h('button','wcc49-submit','Enviar comprobante');submit.type='submit';submit.disabled=true;footer.append(feedback,submit);detailBody.append(footer);
   const currentPack=()=>packs.find(p=>p.code===selectPack.value)||packs[0];
   function refreshPack(){const p=currentPack();summary.replaceChildren(h('strong','',money(p.price_cents)),h('small','',`${p.credits} créditos · aprobación manual`));}
   selectPack.addEventListener('change',refreshPack);refreshPack();
   let addresses=null;
   async function loadAddresses(){try{const response=await api('/api/billing/config');addresses={BEP20:String(response.bep20_address||'').trim(),TRC20:String(response.trc20_address||'').trim()};updateAddress()}catch(e){addresses=null;addressValue.textContent='No se pudo confirmar la dirección oficial';copy.disabled=true;submit.disabled=true;feedback.textContent='No realices transferencias hasta consultar los datos oficiales.'}}
   function updateAddress(){const value=addresses?.[network.value]||'';addressValue.textContent=value||'Esta red no tiene dirección oficial configurada';copy.disabled=!value;updateSubmit()}
   function updateSubmit(){const value=addresses?.[network.value]||'';const hash=tx.value.trim();const valid=network.value==='BEP20'?/^0x[0-9a-fA-F]{64}$/.test(hash):/^[0-9a-fA-F]{64}$/.test(hash);submit.disabled=!value||!acknowledged.checked||!valid||!d.topups_enabled;}
   network.addEventListener('change',updateAddress);tx.addEventListener('input',updateSubmit);acknowledged.addEventListener('change',updateSubmit);
   copy.addEventListener('click',async()=>{const v=addresses?.[network.value];if(!v)return;try{await navigator.clipboard.writeText(v);copy.textContent='Copiada';setTimeout(()=>{if(copy.isConnected)copy.textContent='Copiar'},1400)}catch{feedback.textContent='Selecciona y copia manualmente la dirección.'}});
   // El servidor valida titular, red, paquete, hash único y aprobación real.
   form.addEventListener('submit',async ev=>{ev.preventDefault();updateSubmit();if(submit.disabled)return;
     const p=currentPack();if(!window.confirm(`¿Ya enviaste ${money(p.price_cents)} USDT por ${network.value} a la dirección oficial?\nRegistrarás el hash para verificación. No se acreditará saldo automáticamente.`))return;
     submit.disabled=true;feedback.textContent='Registrando solicitud…';
     try{const out=await api('/api/commercial/topups',{package:p.code,network:network.value,tx_hash:tx.value.trim()});feedback.textContent=`Solicitud ${out.id||''} recibida. Estado: pendiente de verificación.`;feedback.classList.add('wcc49-ok');tx.value='';acknowledged.checked=false;detail.open=false;}
     catch(e){feedback.textContent='No se pudo registrar: '+e.message;feedback.classList.remove('wcc49-ok')}
     finally{updateSubmit()}
   });
   const history=h('details','wcc49-history');history.append(h('summary','','Ver mis solicitudes de recarga'));const historyBody=h('div','wcc49-history-body','Abrir para consultar las solicitudes reales.');history.append(historyBody);box.append(history);
   history.addEventListener('toggle',async()=>{if(!history.open)return;historyBody.textContent='Consultando…';try{const data=await api('/api/commercial/topups');historyBody.replaceChildren();const xs=Array.isArray(data.topups)?data.topups:[];if(!xs.length)historyBody.textContent='Aún no hay recargas registradas.';for(const x of xs){const line=h('div','wcc49-request');line.append(h('span','',`${x.credits} créditos · ${x.network}`),h('b','',x.status==='approved'?'Aprobada':x.status==='rejected'?'Rechazada':'Pendiente'));historyBody.append(line)}}catch(e){historyBody.textContent='No se pudieron consultar las solicitudes: '+e.message}});
   loadAddresses();
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
 window.WorkticWalletLegacyRefresh=wallet;
 function setup(){
  const s=document.getElementById('studio-credits');if(s&&!document.getElementById('wccCommercialSection')){
    const el=h('section','wcc-wallet');el.id='wccCommercialSection';el.append(h('div','wcc-wallet-head'),h('div',''));const head=el.firstElementChild;head.append(h('div','wcc-small','WORKTIC STUDIO · CREATIVE CREDITS'),h('h2','','Mi billetera creativa'));
    const refresh=h('button','wcc-refresh','Actualizar saldo');refresh.type='button';refresh.addEventListener('click',wallet);head.append(refresh);const body=el.lastElementChild;body.id='wccWalletV2721';body.textContent='Abre Worktic Studio para ver tus créditos.';s.append(el)
  }
  const adm=document.getElementById('admin');if(adm&&!document.getElementById('wccCommercialAdmin')){
    const sec=h('section','wcc-admin commercial-v2721');sec.append(h('div','wcc-title'),h('div',''));sec.firstElementChild.append(h('div','wcc-small','V27.21 · PLANES Y MONETIZACIÓN'),h('h2','','Commercial Control Center'));
    const refresh=h('button','wcc-refresh','Actualizar administración');refresh.type='button';refresh.addEventListener('click',admin);sec.firstElementChild.append(refresh);sec.lastElementChild.id='wccCommercialAdmin';adm.append(sec)
  }
  document.addEventListener('click',e=>{const b=e.target.closest('[data-view]');if(!b)return;if(b.dataset.view==='studio-credits')setTimeout(wallet,100);if(b.dataset.view==='admin')setTimeout(admin,350)});
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
