/* WorkticAI V27.35.7 — Finances: single superadmin pricing & sales administration.
   All edits use server-side superadmin-only endpoints; no client-side fake balances.
   Existing Studio package tiers/prices are preserved until explicitly changed. */
(()=>{'use strict';
 const root=document.getElementById('wtc57AdminRoot');
 const tab=document.querySelector('[data-admin-tab="wallet"]');
 if(!root||!tab)return;
 const $=(s,p=root)=>p.querySelector(s);
 const E=(tag,cls,text)=>{const x=document.createElement(tag);if(cls)x.className=cls;if(text!==undefined)x.textContent=String(text);return x};
 const fmt=n=>Number(n||0).toLocaleString('es-CO');
 const usdt=c=>Number(c/100).toLocaleString('es-CO',{minimumFractionDigits:2,maximumFractionDigits:2});
 let studio=null,voice=null,busy=false;
 async function api(path,body){
  const opts={credentials:'same-origin',cache:'no-store',headers:{Accept:'application/json'}};
  if(body!==undefined){opts.method='POST';opts.headers['Content-Type']='application/json';opts.body=JSON.stringify(body)}
  let response;
  try{response=await fetch(path,opts)}catch{throw Error('Sin conexión con el servidor. Intenta de nuevo.');}
  const raw=await response.text();let payload={};try{payload=JSON.parse(raw)}catch{}
  if(!response.ok)throw Error(payload.error||raw.slice(0,220)||`HTTP ${response.status}`);
  return payload;
 }
 const state=(text,kind='normal')=>{const status=$('#wtc57Status');if(!status)return;status.textContent=text;status.className='wtc57-status '+kind;status.setAttribute('role','status')};
 function field(name,input,help){const label=E('label','wtc57-field');label.append(E('span','',name),input);if(help)label.append(E('small','',help));return label}
 function num(v,min,max){const x=E('input');x.type='number';x.inputMode='decimal';x.step='1';x.min=String(min);x.max=String(max);x.value=String(v??0);return x}
 const checkbox=(label,checked)=>{const l=E('label','wtc57-check');const x=E('input');x.type='checkbox';x.checked=!!checked;l.append(x,E('span','',label));return {label:l,input:x}}
 const button=(label,handler)=>{const b=E('button','wtc57-save',label);b.type='button';b.addEventListener('click',handler);return b}
 async function saveStudioSetting(enabled,b){
  if(busy)return;busy=true;b.disabled=true;state('Guardando estado comercial de Studio…');
  try{await api('/api/admin/studio/topups',{action:enabled?'enable':'disable'});studio=await api('/api/admin/studio/topups');draw();state('Configuración de Studio guardada y verificada con el servidor.','ok')}
  catch(e){state('No se guardó Studio: '+e.message,'bad');b.disabled=false;}
  finally{busy=false}
 }
 async function savePackage(pack,inputs,b){
  if(busy)return;const credits=Number(inputs.credits.value),cents=Math.round(Number(inputs.price.value)*100);
  if(!Number.isInteger(credits)||credits<1||credits>50000||!Number.isInteger(cents)||cents<credits*5||cents>5000000){state('Paquete inválido: mínimo US$0,05 por crédito; revisa importe y cantidad.','bad');return}
  busy=true;b.disabled=true;state('Guardando el paquete '+pack.name+'…');
  try{await api('/api/admin/studio/topups',{action:'save_package',code:pack.code,name:pack.name,credits,price_cents:cents,active:inputs.active.checked});studio=await api('/api/admin/studio/topups');draw();state('Paquete '+pack.name+' actualizado. Los pagos pendientes conservan su importe original.','ok')}
  catch(e){b.disabled=false;state('No se guardó el paquete: '+e.message,'bad')}
  finally{busy=false}
 }
 async function saveVoice(fields,b){
  if(busy)return;
  const unit=Number(fields.unit.value), perMin=Number(fields.minute.value), enterprise=Number(fields.enterprise.value);
  if(!Number.isInteger(unit)||unit<1||unit>100||!Number.isInteger(perMin)||perMin<10||perMin>1000||!Number.isInteger(enterprise)||enterprise<0||enterprise>100000){state('Revisa los límites de precio, consumo y créditos mensuales de voz.','bad');return}
  const instr=fields.instructions.value.trim(),url=fields.link.value.trim();
  if(instr.length>500||url.length>300){state('Instrucciones o enlace demasiado largos.','bad');return}
  if(fields.sales.checked&&!instr){state('Añade las instrucciones de pago antes de activar las ventas de voz.','bad');fields.instructions.focus();return}
  busy=true;b.disabled=true;state('Guardando tarifas y ventas de Llamadas IA…');
  try{
   await api('/api/admin/voice/v2735',{action:'settings',credit_price_cents:unit,price_per_minute_credits:perMin,enterprise_monthly_credits:enterprise,sales_enabled:fields.sales.checked,payment_instructions:instr,payment_url:url});
   voice=await api('/api/admin/voice/v2735');draw();
   const effective=voice.sales_enabled?'Ventas efectivamente habilitadas.':'Configuración guardada, pero las ventas aún están bloqueadas por requisitos técnicos o de pago.';
   state('Llamadas IA: '+effective,'ok');
  }catch(e){b.disabled=false;state('No se guardó Llamadas IA: '+e.message,'bad')}
  finally{busy=false}
 }
 function renderStudio(){
  const card=E('section','wtc57-panel');card.append(E('div','wtc57-eyebrow','STUDIO IA · CRÉDITOS CREATIVOS'),E('h3','', 'Paquetes y ventas Studio'));
  card.append(E('p','wtc57-muted','El precio por crédito varía según el paquete. Conservamos tus cinco paquetes y valores actuales; puedes ajustar cada tarifa sin cambiar los créditos existentes.'));
  const bar=E('div','wtc57-row');const sw=checkbox('Habilitar venta de créditos Studio IA',studio.admin_enabled);bar.append(sw.label);
  const b=button('Guardar estado',()=>saveStudioSetting(sw.input.checked,b));bar.append(b);card.append(bar);
  if(!studio.topups_enabled){const missing=[];if(!studio.studio_generation_enabled)missing.push('presupuesto Studio');if(!studio.image_provider_configured)missing.push('OpenAI');if(!studio.video_provider_configured)missing.push('proveedor de video');if(!Object.values(studio.addresses||{}).some(Boolean))missing.push('dirección USDT');
   card.append(E('p','wtc57-notice',studio.admin_enabled?'Ventas solicitadas, pero bloqueadas: falta '+(missing.join(', ')||'una condición técnica')+'.':'Ventas Studio desactivadas por administración.'));}
  else card.append(E('p','wtc57-success','Venta de créditos Studio habilitada y disponible.'));
  const table=E('div','wtc57-pack-list');const packages=studio.packages||[];
  for(const pack of packages){const wrap=E('article','wtc57-package');wrap.append(E('strong','',pack.name+' · '+pack.code));
   const fields=E('div','wtc57-pack-fields');const credits=num(pack.credits,1,50000),price=E('input');price.type='number';price.inputMode='decimal';price.step='0.01';price.min='0.01';price.max='50000';price.value=(Number(pack.price_cents)/100).toFixed(2);
   const active=checkbox('Paquete disponible',pack.active);
   const value=E('p','wtc57-unit','Precio actual por crédito: '+usdt(pack.price_cents/pack.credits)+' USDT');
   credits.addEventListener('input',reprice);price.addEventListener('input',reprice);
   function reprice(){const n=Number(credits.value);const c=Number(price.value);value.textContent='Precio por crédito: '+(n>0?Number(c/n).toLocaleString('es-CO',{minimumFractionDigits:2,maximumFractionDigits:4}):'—')+' USDT'}
   fields.append(field('Créditos',credits),field('Precio del paquete (USDT)',price),active.label);
   const save=button('Guardar paquete',()=>savePackage(pack,{credits,price,active:active.input},save));
   wrap.append(fields,value,save);table.append(wrap);
  }
  card.append(table);return card;
 }
 function renderVoice(){
  const card=E('section','wtc57-panel');card.append(E('div','wtc57-eyebrow','LLAMADAS IA · VOZ Y TELEFONÍA'),E('h3','','Tarifas, paquetes y ventas de voz'));
  card.append(E('p','wtc57-muted','Valores anteriores preservados: 1 crédito = 0,01 USDT, 30 créditos por minuto y 300 créditos mensuales incluidos en Empresa. Cualquier modificación se aplicará a nuevas compras y llamadas.'));
  const unit=num(voice.credit_price_cents||1,1,100),minute=num(voice.price_per_minute_credits,10,1000),enterprise=num(voice.enterprise_monthly_credits,0,100000);
  const grid=E('div','wtc57-three');grid.append(field('Precio de 1 crédito (centavos USD/USDT)',unit,'1 = US$0,01 por crédito'),field('Créditos consumidos por minuto',minute,'Tarifa de llamadas futuras'),field('Créditos mensuales del plan Empresa',enterprise,'Nuevas asignaciones mensuales'));
  card.append(grid);
  const prices=E('div','wtc57-voice-preview');card.append(prices);
  function preview(){const credit=Number(unit.value),rate=Number(minute.value);prices.textContent=`Paquetes actuales: 10 minutos ≈ ${fmt(rate*10)} créditos / ${usdt(rate*10*credit)} USDT · 30 minutos ≈ ${fmt(rate*30)} créditos / ${usdt(rate*30*credit)} USDT · 100 minutos ≈ ${fmt(rate*100)} créditos / ${usdt(rate*100*credit)} USDT`}
  unit.addEventListener('input',preview);minute.addEventListener('input',preview);preview();
  const sales=checkbox('Habilitar venta de paquetes de Llamadas IA',voice.sales_enabled_setting);card.append(sales.label);
  const instructions=E('textarea');instructions.maxLength=500;instructions.rows=3;instructions.value=voice.payment_instructions||'';
  const link=E('input');link.type='url';link.placeholder='https:// (opcional)';link.maxLength=300;link.value=voice.payment_url||'';
  card.append(field('Instrucciones de pago (USDT, red y confirmación)',instructions),field('Enlace de pago HTTPS (opcional)',link));
  const warn=E('p',voice.sales_enabled?'wtc57-success':'wtc57-notice');
  const warnings=[];if(!voice.provider_ready)warnings.push('habilitar WORKTIC_VOICE_ENABLED y OPENAI_API_KEY');if(!voice.sales_enabled_setting)warnings.push('activar ventas');if(!voice.payment_address_ready)warnings.push('dirección USDT');if(!(voice.payment_instructions||'').trim())warnings.push('guardar instrucciones');
  warn.textContent=voice.sales_enabled?'Ventas de voz habilitadas. Las llamadas salientes mantienen su control independiente.':'Ventas actualmente bloqueadas. '+(warnings.length?'Revisar: '+warnings.join('; '):'Revisar dirección USDT y plan del cliente.');card.append(warn);
  const save=button('Guardar tarifas y configuración de voz',()=>saveVoice({unit,minute,enterprise,sales:sales.input,instructions,link},save));card.append(save);
  if(Array.isArray(voice.pending)&&voice.pending.length){const note=E('p','wtc57-muted',`${voice.pending.length} solicitud(es) de voz pendientes de revisión. El panel de aprobaciones conserva las validaciones originales del servidor.`);card.append(note)}
  return card;
 }
 function draw(){root.replaceChildren();const heading=E('div','wtc57-header');heading.append(E('div','wtc57-eyebrow','ADMINISTRACIÓN · CENTRO FINANCIERO'),E('h2','','Billetera de Créditos'),E('p','wtc57-muted','Precios y ventas centralizados para Studio IA y Llamadas IA. Los clientes solo ven sus saldos, QR, paquetes e historial.'));root.append(heading);root.append(E('div','wtc57-status','Configuración cargada.'));
  const cols=E('div','wtc57-cols');cols.append(renderStudio(),renderVoice());root.append(cols);
  root.append(E('p','wtc57-disclaimer','Seguridad: únicamente el superadministrador modifica tarifas. Los cambios no recalculan pagos históricos ni acreditan saldos automáticamente; los comprobantes requieren validación administrativa.'));
 }
 async function load(){if(root.closest('.wt-admin-panel')?.classList.contains('active')===false)return;
  root.replaceChildren(E('div','wt-admin-empty','Cargando precios y controles de venta…'));
  try{[studio,voice]=await Promise.all([api('/api/admin/studio/topups'),api('/api/admin/voice/v2735')]);draw()}
  catch(e){root.replaceChildren(E('div','wtc57-status bad','No se pudo cargar la configuración administrativa: '+e.message));}
 }
 tab.addEventListener('click',()=>{setTimeout(load,0)});
})();
