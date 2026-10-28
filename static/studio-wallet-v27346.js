/* WorkticAI Studio Wallet V27.34.6. Pure presentation of authenticated commercial APIs.
   No fake activity, client-side balances, payment approvals or entitlement bypasses. */
(()=>{'use strict';
 const $=(s,root=document)=>root.querySelector(s);
 const $$=(s,root=document)=>Array.from(root.querySelectorAll(s));
 const fmt=n=>Number(n||0).toLocaleString('es-CO',{maximumFractionDigits:0});
 const clamp=n=>Math.max(0,Number(n)||0);
 const text=(root,sel,value)=>{const x=$(sel,root);if(x)x.textContent=String(value)};
 let current=null,catalog=null,actions=null,panel='overview',expanded=true,loaded=false;
 let popup=null,popupFocus=null,bodyOverflow=null;
 const state={};
 async function api(url){const r=await fetch(url,{credentials:'same-origin',headers:{Accept:'application/json'}});if(!r.ok)throw Error('Servicio no disponible (HTTP '+r.status+')');return r.json()}
 const icons={wallet:'◈',images:'▧',video:'▶',voice:'♫',hold:'◷',topup:'＋',pending:'◌',done:'✓'};
 const statusMap={spent:'Consumido',released:'Liberado',held:'Reservado',approved:'Aprobada',pending:'Pendiente',rejected:'Rechazada',refunded:'Reembolsada'};
 function badge(){
  if(!$('#wccNavCredits')){
   const top=$('#v276GlobalTopbar .v276-top-actions');
   if(top){const wrap=document.createElement('div');wrap.id='wccNavCredits';wrap.className='wcc46-nav';wrap.innerHTML='<button type="button" class="wcc46-nav-balance" aria-label="Ver billetera de créditos"><span class="wcc46-nav-ic">◈</span><b id="wccNavAmount">Créditos</b></button><button type="button" class="wcc46-nav-add" aria-label="Recargar créditos" title="Recargar créditos">+</button>';top.prepend(wrap);$('.wcc46-nav-balance',wrap).addEventListener('click',()=>openWallet('overview'));$('.wcc46-nav-add',wrap).addEventListener('click',()=>openWallet('topups'))}
  }
  if(!$('#wccMobileCredits')){
   const mobile=$('.mobile-appbar');if(mobile){const chip=document.createElement('button');chip.id='wccMobileCredits';chip.type='button';chip.className='wcc46-mobile-balance';chip.textContent='◈ Créditos';chip.addEventListener('click',()=>openWallet('overview'));const menu=$('#mobileStatus',mobile);mobile.insertBefore(chip,menu||null)}
  }
 }
 function updateBadge(d){
  const n=d?fmt(d.available_credits)+' créditos':'Créditos';
  text(document,'#wccNavAmount',n);text(document,'#wccMobileCredits',d?'◈ '+fmt(d.available_credits):'◈ Créditos');
  $('#wccNavCredits')?.classList.toggle('wcc46-disabled',Boolean(d&&!d.enabled));
  if(d)$('#wccNavCredits')?.setAttribute('title',d.enabled?'Saldo disponible de Worktic Studio':'Créditos asignados; la generación IA aún requiere activación');
 }
 function goCredits(){const view=$('#studio-credits');if(view&&!view.classList.contains('active'))document.querySelector('button[data-view="studio-credits"]')?.click()}
 function openWallet(target){
  const requested=target==='moves'||target==='topups'?target:'overview';
  expanded=true;goCredits();
  if(current&&actions)draw();else window.WorkticWalletLegacyRefresh?.();
  if(requested!=='overview'){openDialog(requested);return}
  closeDialog();
  setTimeout(()=>{const el=$('#wccCommercialSection');if(el){el.classList.remove('wcc46-collapsed');el.scrollIntoView({behavior:'smooth',block:'start'});}},120);
 }
 function walletMark(){const old=$('#wccCommercialSection');const s=$('#studio-credits');if(!old||!s)return;
   old.classList.add('wcc46-shell');
   if(old.parentElement!==s)s.append(old);
 }
 function showNotice(target,message){target.replaceChildren();const box=document.createElement('div');box.className='wcc46-state-note';box.textContent=message;target.append(box)}
 async function movements(root){const list=$('#wcc46MovementsList',root);if(!list)return;
  if(window.WorkticUnifiedCreditsV27355?.movements)return window.WorkticUnifiedCreditsV27355.movements(list);
  showNotice(list,'Consultando movimientos reales…');
  try{
    const data=await api('/api/commercial/movements');const ms=(data.movements||[]).sort((a,b)=>String(b.at||'').localeCompare(String(a.at||''))).slice(0,60);
    list.replaceChildren();if(!ms.length){showNotice(list,'Todavía no hay generaciones ni recargas registradas.');return}
    for(const m of ms){const isTop=m.kind==='topup';const row=document.createElement('div');row.className='wcc46-move';
      const icon=document.createElement('span');icon.className='wcc46-move-icon';icon.textContent=isTop?'＋':m.status==='spent'?'✦':m.status==='released'?'↶':'◷';
      const desc=document.createElement('div');desc.className='wcc46-move-desc';const title=document.createElement('strong');title.textContent=m.operation||'Movimiento';const meta=document.createElement('span');let date=String(m.at||'');try{const parsed=new Date(m.at);if(!Number.isNaN(parsed.getTime()))date=parsed.toLocaleString('es-CO')}catch{}meta.textContent=(statusMap[m.status]||m.status||'Registrado')+' · '+date;desc.append(title,meta);
      const cost=document.createElement('b');const counted=isTop?m.status==='approved':m.status==='spent';cost.textContent=counted?(isTop?'+':'−')+fmt(m.credits)+' créditos':fmt(m.credits)+' créditos';cost.className=counted&&isTop?'wcc46-positive':'wcc46-neutral';row.append(icon,desc,cost);list.append(row)
    }
  }catch(e){showNotice(list,'No fue posible cargar los movimientos: '+e.message)}
 }
 function render(el,d,cat,ctx){current=d;catalog=cat;actions=ctx;loaded=true;badge();walletMark();updateBadge(d);draw();if(popup&&!popup.hidden)populateDialog(panel);}
 function draw(){const root=$('#wccCommercialSection'),el=$('#wccWalletV2721');if(!root||!el||!current)return;
  walletMark();root.classList.toggle('wcc46-collapsed',!expanded);
  const d=current,r=d.rule||{};const grant=clamp(d.monthly_grant),used=clamp(d.plan_spent),reserve=clamp(d.plan_reserved);const credits=clamp(d.available_credits),pct=grant?Math.min(100,Math.round(used/grant*100)):0;
  const name={free:'Free',personal:'Personal',business:'Negocio',enterprise:'Empresa'}[d.plan]||d.plan;
  root.querySelector('.wcc-wallet-head')?.setAttribute('hidden','');
  el.innerHTML=`<div class="wcc46-hero"><div class="wcc46-hero-title"><span class="wcc46-hero-icon" aria-hidden="true">◈</span><div><div class="wcc46-overline">WORKTIC AI · CENTRO DE CRÉDITOS</div><h2>Billetera de Créditos <span class="wcc46-plan" id="wcc46Plan"></span></h2><p>Consulta los saldos de Studio y Llamadas IA, sus consumos y recargas desde un único lugar.</p></div></div><div class="wcc46-hero-actions"><button type="button" id="wcc46MovesButton" class="wcc46-btn-ghost">◷ Ver movimientos</button><button type="button" id="wcc46BuyButton" class="wcc46-btn-main">＋ Recargar créditos</button><button type="button" id="wcc46BackToEditor" class="wcc46-btn-ghost">↗ Abrir editor</button></div></div><div id="wcc46Content"><div class="wcc46-cards"><article class="wcc46-card wcc46-main-card"><span>◈ &nbsp; ${d.enabled?'Créditos disponibles':'Créditos asignados'}</span><strong id="wcc46Available"></strong><small id="wcc46AvailableNote"></small></article><article class="wcc46-card"><span>◷ &nbsp; Consumidos este mes</span><strong id="wcc46Spent"></strong><small id="wcc46SpendNote"></small><div class="wcc46-small-track"><div id="wcc46SmallProgress"></div></div></article><article class="wcc46-card"><span>♛ &nbsp; Incluidos en tu plan</span><strong id="wcc46Grant"></strong><small id="wcc46Period"></small></article><article class="wcc46-card"><span>▤ &nbsp; Créditos comprados</span><strong id="wcc46Bought"></strong><small>Saldo adicional verificado</small></article></div><div id="wcc46Banner" class="wcc46-banner"></div><div class="wcc46-main-grid"><section class="wcc46-pane"><h3>Uso de créditos este mes</h3><p class="wcc46-sub">Tu consumo del plan mensual</p><div class="wcc46-progress"><div id="wcc46ProgressBar"></div></div><div class="wcc46-progress-meta"><span id="wcc46Percent"></span><span id="wcc46ProgressUnits"></span></div><div class="wcc46-split"><div><b id="wcc46Remaining"></b><small>Créditos del plan disponibles</small></div><div><b id="wcc46Reserved"></b><small>Reservados en proceso</small></div></div></section><section class="wcc46-pane"><h3>¿Qué puedes crear?</h3><p class="wcc46-sub">Estimación usando el saldo mensual completo</p><div class="wcc46-examples"><div><span class="wcc46-example-ic">▧</span><b id="wcc46Images"></b><small>Imágenes IA</small></div><div><span class="wcc46-example-ic">▶</span><b id="wcc46Videos"></b><small>Videos IA de 5 s</small></div><div><span class="wcc46-example-ic">♫</span><b id="wcc46Voices"></b><small>Narraciones de hasta 1.000 caracteres</small></div></div><p class="wcc46-fineprint">Son alternativas, no acumulables. Precios de referencia: 4, 15 y 5 créditos respectivamente; las operaciones se cotizan antes de ejecutarse.</p></section><section class="wcc46-pane"><h3>Tu plan y sus límites</h3><p class="wcc46-sub">Permisos aplicados por el servidor</p><div id="wcc46Limits" class="wcc46-limits"></div></section></div></div>`;
  window.WorkticUnifiedCreditsV27355?.balances(el,d);
  text(el,'#wcc46Plan','Plan '+name);text(el,'#wcc46Available',fmt(credits)+' créditos');text(el,'#wcc46AvailableNote',d.enabled?'Saldo del plan y créditos comprados':'Generación IA pendiente de activación');text(el,'#wcc46Spent',fmt(used)+' créditos');text(el,'#wcc46SpendNote','de '+fmt(grant)+' del plan');text(el,'#wcc46Grant',fmt(grant)+' créditos');text(el,'#wcc46Period',String(d.period||'')+' · Renovación mensual');text(el,'#wcc46Bought',fmt(d.purchased_available)+' créditos');text(el,'#wcc46Percent',pct+'% utilizado');text(el,'#wcc46ProgressUnits',fmt(used)+' / '+fmt(grant));text(el,'#wcc46Remaining',fmt(d.plan_available));text(el,'#wcc46Reserved',fmt(reserve));text(el,'#wcc46Images',String(Math.floor(grant/4)));text(el,'#wcc46Videos',String(Math.floor(grant/15)));text(el,'#wcc46Voices',String(Math.floor(grant/5)));$('#wcc46ProgressBar',el).style.width=pct+'%';$('#wcc46SmallProgress',el).style.width=pct+'%';
  const banner=$('#wcc46Banner',el);banner.classList.toggle('wcc46-not-enabled',!d.enabled);banner.textContent=d.enabled?'✓ Los créditos se reservan al iniciar una generación autorizada y se liquidan según el resultado.':'◷ Tienes una asignación mensual de créditos. La generación con IA permanece protegida hasta que se habiliten y verifiquen los presupuestos del proveedor.';
  if(d.enabled){api('/api/studio/cost/status').then(v=>{if(current!==d)return;const b=$('#wcc46Banner',el);if(!b)return;const cents=Number(v.available_cents||0);if(cents<=0){b.classList.add('wcc46-not-enabled');b.textContent='◷ Presupuesto externo agotado o no configurado. Las generaciones IA permanecerán bloqueadas para proteger tu empresa.';}else{b.textContent='✓ Créditos activos y presupuesto externo disponible. Cada generación se verifica y reserva antes de consumir recursos.';}}).catch(()=>{if(current!==d)return;const b=$('#wcc46Banner',el);if(b){b.classList.add('wcc46-not-enabled');b.textContent='No fue posible verificar el presupuesto externo. Si el proveedor no confirma el cupo, la generación permanecerá bloqueada.';}})}
  const list=$('#wcc46Limits',el);
  const rules=[['Editor de imágenes y video',r.editor],['Generación de imágenes IA',r.image_ai],['Generación de videos IA',r.video_ai],['Narración IA',r.voice_ai],['Textos IA',r.text_ai]];
  for(const [label,yes] of rules){const line=document.createElement('div');const n=document.createElement('span');n.textContent=label;const status=document.createElement('b');status.className=yes?'wcc46-yes':'wcc46-no';status.textContent=yes?'✓ Permitido':'— No incluido';line.append(n,status);list.append(line)}
  const more=document.createElement('div');more.className='wcc46-limit-extra';more.textContent=`${fmt(r.max_projects||0)} proyectos · ${fmt(r.storage_mb||0)} MB de almacenamiento`;list.append(more);
  $('#wcc46BackToEditor',el).addEventListener('click',()=>document.querySelector('button[data-view="studio"]')?.click());$('#wcc46MovesButton',el).addEventListener('click',()=>openDialog('moves'));$('#wcc46BuyButton',el).addEventListener('click',()=>openDialog('topups'));
 }
 // V27.34.8: prominent, accessible dialogs instead of an invisible panel below the page fold.
 function ensureDialog(){
  if(popup&&popup.isConnected)return popup;
  popup=document.createElement('div');popup.id='wcc48Overlay';popup.className='wcc48-overlay';popup.hidden=true;
  popup.innerHTML='<section class="wcc48-dialog" role="dialog" aria-modal="true" aria-labelledby="wcc48Title" tabindex="-1"><div class="wcc48-header"><div><div class="wcc48-eyebrow">WORKTIC AI · BILLETERA DE CRÉDITOS</div><h2 id="wcc48Title"></h2></div><button type="button" id="wcc48Close" aria-label="Cerrar panel">✕</button></div><div class="wcc48-content" id="wcc48Content" aria-live="polite"></div></section>';
  popup.addEventListener('click',e=>{if(e.target===popup)closeDialog()});
  $('#wcc48Close',popup).addEventListener('click',closeDialog);
  document.addEventListener('keydown',e=>{
   if(!popup||popup.hidden)return;
   if(e.key==='Escape'){e.preventDefault();closeDialog();return}
   if(e.key!=='Tab')return;
   const focusable=$$('button:not([disabled]), input:not([disabled]), select:not([disabled]), a[href], [tabindex="0"]',popup).filter(x=>x.getClientRects().length);
   if(!focusable.length){e.preventDefault();$('.wcc48-dialog',popup).focus();return}
   const first=focusable[0],last=focusable[focusable.length-1];
   if(e.shiftKey&&document.activeElement===first){e.preventDefault();last.focus()}
   if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first.focus()}
  });
  document.body.appendChild(popup);
  return popup;
 }
 function closeDialog(){
  if(!popup||popup.hidden)return;
  popup.hidden=true;panel='overview';document.body.style.overflow=bodyOverflow===null?'':bodyOverflow;
  bodyOverflow=null;
  const restore=popupFocus;popupFocus=null;
  if(restore&&restore.isConnected&&typeof restore.focus==='function')restore.focus({preventScroll:true});
 }
 function openDialog(kind){
  if(kind!=='moves'&&kind!=='topups')return;
  panel=kind;const overlay=ensureDialog();
  if(overlay.hidden){popupFocus=document.activeElement;bodyOverflow=document.body.style.overflow;document.body.style.overflow='hidden'}
  overlay.hidden=false;
  populateDialog(kind);
  $('#wcc48Close',overlay)?.focus({preventScroll:true});
 }
 function populateDialog(kind){
  if(!popup||popup.hidden)return;
  const content=$('#wcc48Content',popup);const title=$('#wcc48Title',popup);
  content.replaceChildren();title.textContent=kind==='moves'?'Movimientos de créditos':'Recargar créditos';
  if(!current){showNotice(content,'Consultando tu billetera…');return}
  if(kind==='moves'){
   const head=document.createElement('div');head.className='wcc48-section-head';
   const desc=document.createElement('p');desc.textContent='Historial de operaciones de tu empresa. No se muestran transacciones de otros negocios.';
   const reload=document.createElement('button');reload.type='button';reload.textContent='↻ Actualizar';reload.addEventListener('click',()=>movements(content));
   head.append(desc,reload);content.append(head);
   const list=document.createElement('div');list.id='wcc46MovementsList';content.append(list);
   movements(content);return;
  }
  // Same compact purchase interface for active and inactive stores.
  // Never display long fake-card catalogs to a tenant without enabled purchasing.
  if(window.WorkticUnifiedCreditsV27355?.store)window.WorkticUnifiedCreditsV27355.store(content,catalog||{packages:[]},current);
  else if(actions?.buildStore)actions.buildStore(content,catalog||{packages:[]},current);
  else showNotice(content,'Cargando el catálogo de paquetes. Vuelve a intentar en unos segundos.');
 }
 function setup(){badge();walletMark();const old=$('#wccCommercialSection');if(old){const head=$('.wcc-wallet-head',old);if(head)head.hidden=true}
  document.addEventListener('click',ev=>{const x=ev.target.closest('button[data-view]');if(x?.dataset.view==='studio-credits'){setTimeout(walletMark,100)}});
  document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible'&&$('#studio-credits')?.classList.contains('active'))actions?.wallet()});
  // The shell's login may still be loading. Do not expose synthetic balances.
  setTimeout(async()=>{try{const data=await api('/api/commercial/wallet');updateBadge(data)}catch{}},1400)
 }
 window.WorkticWalletV27346={render,open:openWallet,refresh:()=>actions?.wallet(),statusError:(message)=>{const el=$('#wccWalletV2721');if(el)showNotice(el,message)}};
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',setup);else setup();
})();
