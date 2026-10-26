/* WorkticAI V27.34.9 — prepaid USDT credit packages, admin settings and evidence-based review. */
(()=>{'use strict';
const el=(tag,cls,txt)=>{const n=document.createElement(tag);if(cls)n.className=cls;if(txt!==undefined)n.textContent=String(txt);return n};
const amount=c=>(Number(c||0)/100).toLocaleString('es-CO',{minimumFractionDigits:2,maximumFractionDigits:2})+' USDT';
const esc=s=>String(s||'').trim();
const info=(target,msg,error=false)=>{target.replaceChildren(el('div',error?'wt49-msg is-error':'wt49-msg',msg))};
const control=(label,type,value='',required=true)=>{const l=el('label','wt49-field');l.append(el('span','',label));const input=el('input');input.type=type;input.value=value;input.required=required;l.append(input);return {wrap:l,input}};
async function renderStore(root,cat,d,api){
 root.replaceChildren(el('div','wt49-loading','Cargando paquetes y medios de pago oficiales…'));
 try{
  const [c,catalog]=await Promise.all([api('/api/commercial/topups/config'),api('/api/commercial/catalog')]);
  root.replaceChildren();
  const wrap=el('div','wt49-shop');root.append(wrap);
  const head=el('div','wt49-shop-head');const title=el('div');title.append(el('span','wt49-eyebrow','WORKTIC STUDIO · CRÉDITOS'),el('h3','','Elige cuánto quieres crear'),el('p','','Amplía tu saldo cuando lo necesites. Cada compra queda pendiente hasta que verifiquemos el pago en blockchain.'));
  head.append(title,el('span',c.topups_enabled?'wt49-status is-open':'wt49-status',c.topups_enabled?'● Recargas disponibles':'● Compras en preparación'));wrap.append(head);
  const packs=(catalog.packages||[]).filter(x=>x.active!==false);if(!packs.length){info(wrap,'No hay paquetes disponibles. Consulta con el administrador.');return}
  const grid=el('div','wt49-packages');wrap.append(grid);let selected=packs[0];const buttons=new Map();
  const refreshSelect=()=>{for(const [code,button] of buttons)button.classList.toggle('is-selected',selected?.code===code);choice.textContent=`${selected?.credits||0} créditos · ${amount(selected?.price_cents)}`};
  for(const pack of packs){const b=el('button','wt49-pack');b.type='button';b.append(el('span','wt49-pack-label',pack.name),el('strong','',Number(pack.credits||0).toLocaleString('es-CO')+' créditos'),el('span','wt49-pack-price',amount(pack.price_cents)),el('small','','Acreditación tras comprobación'));b.addEventListener('click',()=>{selected=pack;refreshSelect()});buttons.set(pack.code,b);grid.append(b)}
  const steps=el('div','wt49-steps');for(const [n,t] of [['01','Elige un paquete'],['02','Envía USDT a la dirección oficial'],['03','Registra el hash'],['04','Recibe los créditos tras la aprobación']]){const step=el('div');step.append(el('b','',n),el('span','',t));steps.append(step)}wrap.append(steps);
  if(!c.topups_enabled){const reasons=[];if(!c.studio_generation_enabled)reasons.push('la generación IA debe tener presupuestos técnicos habilitados');if(!c.image_provider_configured||!c.video_provider_configured)reasons.push('OpenAI y Luma necesitan credenciales operativas');if(!Object.values(c.addresses||{}).some(Boolean))reasons.push('falta configurar la billetera oficial de USDT');if(!c.admin_enabled)reasons.push('Administración todavía no activó la tienda');info(wrap,'Las compras están desactivadas: '+reasons.join('; ')+'. No envíes dinero antes de que la tienda esté habilitada.',true);return}
  const pay=el('form','wt49-checkout');wrap.append(pay);pay.append(el('h4','','Datos para pagar con USDT'));
  const choice=el('div','wt49-choice','');pay.append(choice);
  const networkField=el('label','wt49-field');networkField.append(el('span','','Red de pago'));const network=el('select');for(const [key,label] of [['BEP20','BNB Smart Chain · BEP20'],['TRC20','TRON · TRC20']]){if(!c.addresses?.[key])continue;const op=el('option','',label);op.value=key;network.append(op)}networkField.append(network);pay.append(networkField);
  const wallet=el('div','wt49-pay-wallet');wallet.append(el('small','','Dirección oficial de destino'));const walletText=el('code');wallet.append(walletText);const copy=el('button','','Copiar dirección');copy.type='button';wallet.append(copy);pay.append(wallet);
  const walletRefresh=()=>{walletText.textContent=c.addresses?.[network.value]||'No configurada'};network.addEventListener('change',walletRefresh);walletRefresh();copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(walletText.textContent);copy.textContent='Copiada ✓'}catch{copy.textContent='Selecciona y copia la dirección'}});
  const check=el('label','wt49-checkbox');const checked=el('input');checked.type='checkbox';checked.required=true;check.append(checked,el('span','','Confirmo que comprobé token USDT, red, destinatario e importe antes de transferir.'));pay.append(check);
  const hash=control('Hash de la transacción confirmada','text','');hash.input.placeholder='0x… para BEP20 o hash TRC20';hash.input.autocomplete='off';hash.input.maxLength=70;hash.input.minLength=64;pay.append(hash.wrap);
  const feedback=el('div');pay.append(feedback);const submit=el('button','wt49-primary','Registrar mi pago para verificación');submit.type='submit';pay.append(submit);
  pay.addEventListener('submit',async ev=>{ev.preventDefault();if(!selected||!checked.checked)return;if(!window.confirm('¿Ya enviaste el importe EXACTO de '+amount(selected.price_cents)+' a la dirección oficial? No podemos revertir transferencias equivocadas.'))return;
   submit.disabled=true;try{const result=await api('/api/commercial/topups',{package:selected.code,network:network.value,tx_hash:hash.input.value.trim()});info(feedback,`Solicitud ${result.id} registrada. Está pendiente de verificación del administrador; aún no se han acreditado créditos.`);hash.input.value='';checked.checked=false}
   catch(e){info(feedback,e.message,true)}finally{submit.disabled=false}});
  refreshSelect();
 }catch(e){info(root,'No fue posible cargar los medios de pago: '+e.message,true)}
}
async function renderAdmin(root,existing,api,reloadAdmin){
 const sec=el('section','wt49-admin');root.append(sec);sec.append(el('div','wt49-eyebrow','STUDIO · MONETIZACIÓN'),el('h3','','Recargas y paquetes'),el('p','','Activa la tienda de créditos sin activar los controles comerciales globales. Precio y cupos configurables; aprobación manual verificable.'));
 try{
  const c=await api('/api/admin/studio/topups');const switchRow=el('div','wt49-admin-switch');const status=el('div');status.append(el('strong','',c.topups_enabled?'Tienda de créditos activa':c.admin_enabled?'Tienda activada; falta configuración técnica':'Tienda de créditos desactivada'));
  const flags=el('small','',`Presupuesto IA: ${c.studio_generation_enabled?'configurado':'pendiente'} · OpenAI: ${c.image_provider_configured?'configurado':'pendiente'} · Luma: ${c.video_provider_configured?'configurado':'pendiente'} · BEP20: ${c.addresses?.BEP20?'lista':'sin dirección'} · TRC20: ${c.addresses?.TRC20?'lista':'sin dirección'}`);status.append(flags);switchRow.append(status);
  const switchBtn=el('button','wt49-primary',c.admin_enabled?'Desactivar recargas':'Habilitar recargas');switchBtn.type='button';switchBtn.disabled=!c.admin_enabled&&(!c.studio_generation_enabled||!c.image_provider_configured||!c.video_provider_configured||!Object.values(c.addresses||{}).some(Boolean));switchRow.append(switchBtn);sec.append(switchRow);
  const note=el('div');sec.append(note);switchBtn.addEventListener('click',async()=>{if(!window.confirm(c.admin_enabled?'¿Desactivar nuevas compras? Las solicitudes pendientes seguirán disponibles para revisión.':'¿Ya probaste las generaciones y confirmaste que existen presupuestos técnicos y billetera oficial?'))return;switchBtn.disabled=true;try{await api('/api/admin/studio/topups',{action:c.admin_enabled?'disable':'enable'});await reloadAdmin()}catch(e){info(note,e.message,true);switchBtn.disabled=false}});
  sec.append(el('h4','','Paquetes de recarga'));const grid=el('div','wt49-admin-grid');sec.append(grid);
  function packageForm(pack){const form=el('form','wt49-package-edit');const isNew=!pack;form.append(el('h5','',isNew?'Nuevo paquete':pack.name));const code=control('Código','text',pack?.code||'',true);code.input.pattern='[a-z0-9_-]{2,24}';code.input.readOnly=!isNew;const name=control('Nombre','text',pack?.name||'');name.input.maxLength=64;const credits=control('Créditos','number',pack?.credits||30);credits.input.min='1';credits.input.max='50000';const price=control('Precio USDT','number',pack?String(pack.price_cents/100):'3.50');price.input.min='0.05';price.input.step='0.01';price.input.max='50000';for(const f of [code,name,credits,price])form.append(f.wrap);
   const active=el('label','wt49-checkbox');const chk=el('input');chk.type='checkbox';chk.checked=pack?.active??true;active.append(chk,el('span','','Disponible para clientes'));form.append(active);const save=el('button','wt49-secondary','Guardar paquete');save.type='submit';form.append(save);const feedback=el('div');form.append(feedback);
   form.addEventListener('submit',async ev=>{ev.preventDefault();save.disabled=true;const n=Number(price.input.value);try{await api('/api/admin/studio/topups',{action:'save_package',code:esc(code.input.value).toLowerCase(),name:esc(name.input.value),credits:Number(credits.input.value),price_cents:Math.round(n*100),active:chk.checked});info(feedback,'Paquete guardado. Las solicitudes ya realizadas conservan sus importes originales.');if(isNew)setTimeout(reloadAdmin,250)}catch(e){info(feedback,e.message,true)}finally{save.disabled=false}});return form}
  for(const pack of c.packages||[])grid.append(packageForm(pack));grid.append(packageForm(null));
  sec.append(el('h4','','Pagos pendientes de aprobación'));
  const payments=el('div','wt49-payments');sec.append(payments);
  for(const x of existing.topups||[]){if(x.status!=='pending')continue;
   const card=el('article','wt49-review');card.append(el('strong','',`Empresa #${x.tenant_id} · ${x.credits} créditos · ${amount(x.price_cents)}`),el('div','wt49-review-meta',`${x.network} · ${x.package} · ${x.created_at}`));card.append(el('code','',x.tx_hash));
   const form=el('form','wt49-review-form');form.append(el('p','','Revisa esta operación en el explorador blockchain y escribe manualmente los datos RECEPCIONADOS antes de aprobar.'));
   const tx=control('Hash verificado en blockchain','text','');const amountIn=control('Importe USDT recibido','number','');amountIn.input.min='0';amountIn.input.step='0.000001';const wallet=control('Dirección receptora que figura en cadena','text','');for(const f of [tx,amountIn,wallet])form.append(f.wrap);
   const confirm=el('label','wt49-checkbox');const check=el('input');check.type='checkbox';check.required=true;confirm.append(check,el('span','','Confirmo verificación independiente de token, red, destinatario, monto y confirmaciones.'));form.append(confirm);
   const actions=el('div','wt49-review-actions');const approve=el('button','wt49-primary','Aprobar y acreditar');approve.type='submit';const reject=el('button','wt49-secondary','Rechazar');reject.type='button';actions.append(approve,reject);form.append(actions);const error=el('div');form.append(error);
   form.addEventListener('submit',async ev=>{ev.preventDefault();if(!check.checked)return;if(!window.confirm('Aprobar acreditará '+x.credits+' créditos REALES. ¿Verificaste la transferencia independientemente?'))return;approve.disabled=true;
    try{await api('/api/admin/commercial/topups',{id:x.id,action:'approve',verified_on_chain:true,verified_amount_usdt:Number(amountIn.input.value),verified_wallet:esc(wallet.input.value),verified_tx_hash:esc(tx.input.value)});await reloadAdmin()}
    catch(e){info(error,e.message,true);approve.disabled=false}});
   reject.addEventListener('click',async()=>{if(!window.confirm('¿Rechazar esta solicitud pendiente?'))return;reject.disabled=true;try{await api('/api/admin/commercial/topups',{id:x.id,action:'reject'});await reloadAdmin()}catch(e){info(error,e.message,true);reject.disabled=false}});card.append(form);payments.append(card)
  }
  if(!payments.childElementCount)payments.append(el('p','wt49-empty','No hay recargas pendientes.'));
 }catch(e){info(sec,'No se pudo consultar configuración de recargas: '+e.message,true)}
}
window.WorkticStudioTopupsV27349={renderStore,renderAdmin};
})();
