/* WorkticAI V27.35.5 — one wallet screen, separate Studio / Voice ledgers, one USDT recharge workflow.
   QR is generated locally from the official network address; it does NOT certify the payment. */
(()=>{'use strict';
const el=(tag,cls,txt)=>{const e=document.createElement(tag);if(cls)e.className=cls;if(txt!==undefined)e.textContent=String(txt);return e};
const $=(s,r=document)=>r.querySelector(s);
const fmt=n=>Number(n||0).toLocaleString('es-CO',{maximumFractionDigits:0});
const usd=n=>(Number(n||0)/100).toLocaleString('es-CO',{minimumFractionDigits:2,maximumFractionDigits:2})+' USDT';
async function api(path,body){const o={credentials:'same-origin',headers:{Accept:'application/json'}};if(body!==undefined){o.method='POST';o.headers['Content-Type']='application/json';o.body=JSON.stringify(body)}const r=await fetch(path,o);if(!r.ok){throw Error((await r.text()).slice(0,200)||'HTTP '+r.status)}return r.json()}
const state={voice:null,voiceError:null,seq:0,preferredCategory:null};
function walletButton(){document.querySelector('button[data-view="studio-credits"]')?.click()}
async function balances(root,studio){
 const old=$('.wcc55-balances',root);if(old)old.remove();const anchor=$('.wcc46-cards',root);if(!anchor)return;
 const section=el('section','wcc55-balances');section.append(el('div','wcc55-label','SALDOS INDEPENDIENTES · UNA SOLA BILLETERA'));
 const grid=el('div','wcc55-balance-grid');section.append(grid);anchor.before(section);
 const card=(icon,title)=>{const c=el('article','wcc55-balance-card');const h=el('div','wcc55-card-head');h.append(el('span','wcc55-icon',icon),el('b','',title));c.append(h);const strong=el('strong','wcc55-value','—');const sub=el('p','wcc55-sub','Consultando…');const mini=el('div','wcc55-balance-meta');c.append(strong,sub,mini);grid.append(c);return {c,strong,sub,mini}};
 const s=card('✦','Studio IA');s.strong.textContent=fmt(studio.available_credits)+' créditos';s.sub.textContent='Imágenes, videos y narraciones';s.mini.textContent=`${fmt(studio.plan_available)} del plan · ${fmt(studio.purchased_available)} comprados`;
 const v=card('☎','Llamadas IA');v.sub.textContent='Consultando saldo de voz…';
 const id=++state.seq;
 try{const voice=await api('/api/voice/v2735/overview');if(id!==state.seq||!section.isConnected)return;state.voice=voice;state.voiceError=null;
  v.strong.textContent=fmt(voice.wallet?.available)+' créditos';v.sub.textContent=voice.active?'Voz IA disponible según tus canales':'Servicio de voz pendiente de activación técnica';v.mini.textContent=`${fmt(voice.wallet?.included)} del plan · ${fmt(voice.wallet?.purchased)} comprados`;
  const rate=Number(voice.price_per_minute_credits||0);if(rate>0){const detail=el('small','wcc55-minutes',`Equivale aproximadamente a ${Math.floor(Number(voice.wallet?.available||0)/rate)} min a tarifa actual`);v.c.append(detail)}
 }catch(e){if(id!==state.seq||!section.isConnected)return;state.voice=null;state.voiceError=e.message;v.strong.textContent='No disponible';v.sub.textContent='No se pudo consultar el saldo de Llamadas IA';v.mini.textContent='Reintentar abriendo de nuevo la billetera';}
 const label=el('div','wcc55-studio-section-title','Detalle del consumo · Studio IA');anchor.before(label);
 voiceAdmin(section).catch(()=>{});
}
async function voiceAdmin(section){
 // Render strictly for authenticated superadministrators; 403/401 remain invisible.
 let admin,config;
 try{[admin,config]=await Promise.all([api('/api/admin/voice/v2735'),api('/api/commercial/topups/config')])}catch{return}
 if(!section.isConnected)return;
 const details=el('details','wcc56-sales-admin');
 details.append(el('summary','','⚙ Administrar recargas de Llamadas IA'));
 const content=el('div','wcc56-sales-content');details.append(content);
 content.append(el('p','','Configuración comercial de voz dentro de la misma billetera. La activación de cobros no habilita llamadas telefónicas salientes.'));
 const status=el('div','wcc56-sales-status');status.setAttribute('role','status');content.append(status);
 const form=el('form','');content.append(form);
 const toggle=el('label','wcc56-toggle');const sales=el('input');sales.type='checkbox';sales.checked=!!admin.sales_enabled_setting;
 toggle.append(sales,el('span','','Habilitar compra de paquetes de créditos para Llamadas IA'));form.append(toggle);
 const price=el('p','',`Precio de referencia: ${fmt(admin.price_per_minute_credits)} créditos/minuto. Créditos de Empresa: ${fmt(admin.enterprise_monthly_credits)} mensuales.`);form.append(price);
 const instrLabel=el('label','wcc56-sales-field');instrLabel.append(el('span','','Instrucciones de recarga para clientes'));
 const instructions=el('textarea');instructions.maxLength=500;instructions.placeholder='USDT BEP20 o TRC20; verificar importe, red y hash de la transacción antes de aprobar.';
 instructions.value=admin.payment_instructions||'';instrLabel.append(instructions);form.append(instrLabel);
 const actions=el('div','wcc56-sales-actions');const save=el('button','','Guardar configuración de recargas');save.type='submit';actions.append(save);form.append(actions);
 const addressReady=!!(config?.addresses?.BEP20||config?.addresses?.TRC20);
 const configReady=!!admin.provider_ready;
 const describe=(saved='')=>{
  const ready=configReady&&addressReady&&sales.checked&&instructions.value.trim().length>0;
  status.classList.toggle('ready',ready);
  status.textContent=saved||(ready?'Recargas de voz habilitadas para planes de pago. Acreditación manual tras verificar USDT.':
    !configReady?'Servicio de voz sin activar: configurar WORKTIC_VOICE_ENABLED=true y OPENAI_API_KEY en Render antes de vender paquetes.':
    !addressReady?'Falta dirección USDT oficial en la configuración de pagos.':
    !sales.checked?'Las ventas de créditos de voz están desactivadas.':
    !instructions.value.trim()?'Introduce las instrucciones de pago para habilitar las ventas.':'');
 };
 sales.addEventListener('change',()=>describe());instructions.addEventListener('input',()=>describe());describe();
 form.addEventListener('submit',async ev=>{ev.preventDefault();
  if(sales.checked&&(!configReady||!addressReady||!instructions.value.trim())){
   describe();return;
  }
  save.disabled=true;status.textContent='Guardando configuración…';
  try{
   await api('/api/admin/voice/v2735',{action:'settings',price_per_minute_credits:Number(admin.price_per_minute_credits),enterprise_monthly_credits:Number(admin.enterprise_monthly_credits),sales_enabled:sales.checked,payment_instructions:instructions.value.trim(),payment_url:admin.payment_url||''});
   admin.sales_enabled_setting=sales.checked;admin.payment_instructions=instructions.value.trim();
   describe('Configuración guardada. Abre Recargar créditos → Llamadas IA para comprobar los paquetes.');status.classList.add('ready');
  }catch(e){status.classList.remove('ready');status.textContent='No se pudo guardar: '+e.message}
  finally{save.disabled=false}
 });
 section.append(details);
}
async function movements(list){
 list.replaceChildren(el('p','wcc55-loading','Cargando movimientos de Studio y Llamadas IA…'));
 const [studio,voice]=await Promise.allSettled([api('/api/commercial/movements'),api('/api/voice/v2735/overview')]);
 const entries=[];if(studio.status==='fulfilled')for(const m of studio.value.movements||[]){entries.push({at:m.at||'',title:`Studio · ${m.operation||'Movimiento'}`,meta:`${m.status||'Registrado'}`,credits:m.credits||0,positive:m.kind==='topup'&&m.status==='approved',status:m.status})}
 if(voice.status==='fulfilled'){
  for(const m of voice.value.calls||[])entries.push({at:m.created_at||'',title:'Llamadas IA · '+(m.phone||m.direction||'Llamada'),meta:m.status||'Registrada',credits:m.charged||0,positive:false,status:'spent'});
  for(const m of voice.value.topups||[])entries.push({at:m.date||m.created_at||'',title:'Llamadas IA · Recarga',meta:m.status||'Pendiente',credits:m.credits||0,positive:m.status==='approved',status:m.status});
 }
 entries.sort((a,b)=>String(b.at).localeCompare(String(a.at)));list.replaceChildren();
 if(!entries.length)list.append(el('p','wcc46-state-note',studio.status==='rejected'&&voice.status==='rejected'?'No se pudieron consultar los movimientos.':'Todavía no hay movimientos registrados.'));
 for(const m of entries.slice(0,80)){
  const row=el('div','wcc46-move');row.append(el('span','wcc46-move-icon',m.title.startsWith('Studio')?'◈':'☎'));
  const info=el('div','wcc46-move-desc');info.append(el('strong','',m.title),el('span','',(m.meta||'')+' · '+(m.at?new Date(m.at).toLocaleString('es-CO'):'—')));
  const count=el('b',m.positive?'wcc46-positive':'wcc46-neutral',(m.positive?'+':m.status==='spent'?'−':'')+fmt(m.credits)+' créditos');row.append(info,count);list.append(row);
 }
 if(studio.status==='rejected'||voice.status==='rejected')list.append(el('p','wcc55-warning','No se pudo consultar uno de los dos historiales; los movimientos disponibles se muestran arriba.'));
}
function makeLabel(text,control){const l=el('label','wcc55-field');l.append(el('span','',text),control);return l}
async function store(root,studioCatalog,studioWallet){
 root.replaceChildren();const box=el('section','wcc55-store');root.append(box);
 const lead=el('div','wcc55-store-title');lead.append(el('p','','Un solo proceso para Studio y Llamadas IA. Los saldos se acreditan por separado después de verificar el pago.'));box.append(lead);
 const types=el('div','wcc55-segment');types.setAttribute('role','group');types.setAttribute('aria-label','Servicio a recargar');const studioBtn=el('button','','✦ Studio IA');const voiceBtn=el('button','','☎ Llamadas IA');for(const b of [studioBtn,voiceBtn]){b.type='button';types.append(b)}box.append(types);
 const body=el('div');box.append(body);
 let selection=state.preferredCategory==='voice'?'voice':'studio',voice=null,config=null,voiceError='';state.preferredCategory=null;
 const render=()=>{
  studioBtn.classList.toggle('is-selected',selection==='studio');voiceBtn.classList.toggle('is-selected',selection==='voice');body.replaceChildren();
  const packs=(selection==='studio'?(studioCatalog.packages||[]):(voice?.packages||[])).filter(x=>x?.code&&x.active!==false);
  const enabled=selection==='studio'?Boolean(studioWallet.topups_enabled):Boolean(voice?.sales_enabled&&voice?.plan!=='free');
  if(selection==='voice'&&!voice){body.append(el('p','wcc55-warning',voiceError||'Consultando paquetes de Llamadas IA…'));return}
  if(!enabled){body.append(el('p','wcc55-warning',selection==='studio'?'Las recargas Studio están deshabilitadas por administración. No realices pagos.':'Las recargas de Llamadas IA no están disponibles para este plan o aún no están habilitadas por administración.'));return}
  if(!packs.length){body.append(el('p','wcc55-warning','No hay paquetes de créditos disponibles para este servicio.'));return}
  const form=el('form','wcc55-payment');const packagePick=el('select','wcc55-select');for(const p of packs){const opt=el('option','',`${p.name||p.code} · ${fmt(p.credits)} créditos · ${usd(p.price_cents)}`);opt.value=p.code;packagePick.append(opt)}form.append(makeLabel('1. Paquete de créditos',packagePick));
  const price=el('div','wcc55-payment-total');form.append(price);
  const network=el('select','wcc55-select');const destinations=config?.addresses||{};for(const [key,desc] of [['BEP20','BNB Smart Chain · BEP20'],['TRC20','TRON · TRC20']]){if(!destinations[key])continue;const o=el('option','',desc);o.value=key;network.append(o)};
  if(!network.options.length){body.append(el('p','wcc55-warning','No existe una dirección oficial USDT configurada. No realices pagos.'));return}
  form.append(makeLabel('2. Red para pagar con USDT',network));
  const payArea=el('div','wcc55-qr-layout');const qrWrap=el('div','wcc55-qr-container');payArea.append(qrWrap);const aside=el('div','wcc55-qr-aside');aside.append(el('b','','Escanea el QR con tu billetera'),el('p','','Este código contiene únicamente la dirección de recepción. Selecciona USDT, comprueba la red e introduce el importe exacto de forma manual.'));
  const code=el('code','wcc55-destination');aside.append(code);const copy=el('button','wcc55-copy','Copiar dirección');copy.type='button';aside.append(copy);payArea.append(aside);form.append(payArea);
  const note=el('div','wcc55-network-note');form.append(note);
  const hash=el('input','wcc55-select');hash.type='text';hash.autocomplete='off';hash.placeholder='0x… (BEP20) o 64 caracteres (TRC20)';hash.maxLength=66;form.append(makeLabel('3. Hash de la transacción',hash));
  const accept=el('input');accept.type='checkbox';const ack=el('label','wcc55-accept');ack.append(accept,el('span','','Confirmo que verifiqué la red, el token USDT, el importe y que la dirección coincide con la mostrada.'));form.append(ack);
  const info=el('div','wcc55-feedback');info.setAttribute('aria-live','polite');form.append(info);const submit=el('button','wcc55-submit','Registrar pago para revisión');submit.type='submit';form.append(submit);
  function currentPack(){return packs.find(x=>x.code===packagePick.value)||packs[0]}
  function ready(){const tx=hash.value.trim(),valid=network.value==='BEP20'?/^0x[a-fA-F0-9]{64}$/.test(tx):/^[a-fA-F0-9]{64}$/.test(tx);submit.disabled=!valid||!accept.checked||!destinations[network.value]}
  function updatePrice(){const p=currentPack();price.replaceChildren(el('b','',usd(p.price_cents)),el('span','',`${fmt(p.credits)} créditos de ${selection==='studio'?'Studio IA':'Llamadas IA'}`));}
  function updateAddress(){const address=destinations[network.value]||'';code.textContent=address;qrWrap.replaceChildren();try{if(!window.WorkticLocalQRV27355?.render(qrWrap,address))throw Error('QR no disponible')}catch(e){qrWrap.append(el('p','wcc55-warning','No fue posible generar el QR. Puedes copiar la dirección manualmente.'))}note.textContent=`Transferencia USDT por ${network.value}. Dirección oficial y monto: comprueba ambos antes de enviar; el QR no incluye automáticamente el importe.`;ready()}
  packagePick.addEventListener('change',updatePrice);network.addEventListener('change',updateAddress);hash.addEventListener('input',ready);accept.addEventListener('change',ready);
  copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(destinations[network.value]);copy.textContent='Copiada ✓'}catch{info.textContent='No se pudo copiar. Selecciona la dirección manualmente.'}});
  form.addEventListener('submit',async e=>{e.preventDefault();ready();if(submit.disabled)return;const p=currentPack();if(!confirm(`¿Confirmas que pagaste ${usd(p.price_cents)} USDT en ${network.value}?\nLos créditos ${selection==='studio'?'Studio':'Llamadas IA'} se acreditarán solo después de revisión.`))return;
   submit.disabled=true;info.textContent='Registrando comprobante…';try{const payload={package:p.code,network:network.value,tx_hash:hash.value.trim()};const path=selection==='studio'?'/api/commercial/topups':'/api/voice/v2735/topups';const out=await api(path,payload);info.classList.add('is-success');info.textContent=`Solicitud ${out.id||''} registrada correctamente. Pendiente de aprobación; tu saldo no ha cambiado.`;hash.value='';accept.checked=false}catch(err){info.classList.remove('is-success');info.textContent='No se pudo registrar el pago: '+err.message}finally{ready()}});
  updatePrice();updateAddress();body.append(form);
 };
 studioBtn.addEventListener('click',()=>{selection='studio';render()});voiceBtn.addEventListener('click',()=>{selection='voice';render()});render();
 const [conf,voiceResult]=await Promise.allSettled([api('/api/commercial/topups/config'),api('/api/voice/v2735/overview')]);
 if(!box.isConnected)return;
 config=conf.status==='fulfilled'?conf.value:null;
 if(conf.status==='rejected'){body.replaceChildren(el('p','wcc55-warning','No se pudo consultar la configuración oficial de pagos. No envíes dinero.'));studioBtn.disabled=voiceBtn.disabled=true;return}
 if(voiceResult.status==='fulfilled'){voice=voiceResult.value;voiceError=''}else{voiceError='Error consultando Llamadas IA: '+voiceResult.reason.message}
 render();
}
window.WorkticUnifiedCreditsV27355={balances,movements,store,open:(category='studio')=>{state.preferredCategory=category==='voice'?'voice':'studio';walletButton();window.WorkticWalletV27346?.open('topups');}};
})();
