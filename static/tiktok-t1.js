/* Worktic AI TikTok T1 — Login Kit y perfil, sin mensajeria/publicacion prematura */
(()=>{'use strict';
 const $=s=>document.querySelector(s);
 const say=s=>window.toast?window.toast(s):alert(s);
 let busy=false;
 async function readiness(){
  const r=await fetch('/api/social/tiktok/t1/readiness',{credentials:'same-origin',cache:'no-store'});
  if(!r.ok)throw Error('No fue posible consultar TikTok ('+r.status+')');
  return r.json();
 }
 async function connect(){
  if(busy)return;
  busy=true;
  try{
   const d=await readiness();
   if(!d.login_ready){say(d.message||'TikTok T1 no configurado');return;}
   const x=await window.api('/api/social/oauth/start?platform=tiktok');
   const url=new URL(x.authorization_url);
   if(url.origin!=='https://www.tiktok.com'||url.pathname!=='/v2/auth/authorize/')throw Error('URL de autorización TikTok inválida');
   window.location.assign(url.href);
  }catch(e){say(e.message||'Error conectando TikTok')}finally{busy=false;}
 }
 function createPanel(){
  const root=$('#social-connections');if(!root||$('#workticTikTokT1Panel'))return;
  const el=document.createElement('article');el.id='workticTikTokT1Panel';el.className='wt-tt-t1-panel';
  el.innerHTML=`<div class="wt-tt-t1-symbol" aria-hidden="true">♪</div><div class="wt-tt-t1-info"><span class="wt-tt-t1-eyebrow">TIKTOK · LOGIN KIT</span><h3>Conecta tu cuenta oficial TikTok</h3><p>Tu perfil se vinculará únicamente a esta empresa. Publicación, mensajes directos y agentes IA requieren permisos oficiales adicionales.</p><small id="workticTikTokT1Status" role="status">Consultando integración...</small></div><div class="wt-tt-t1-buttons"><button type="button" id="workticTikTokT1Connect">Conectar TikTok</button><button type="button" class="secondary" id="workticTikTokT1Refresh">Verificar configuración</button></div>`;
  const grid=$('#socialConnectionsGrid');root.insertBefore(el,grid);
  $('#workticTikTokT1Connect').addEventListener('click',connect);
  $('#workticTikTokT1Refresh').addEventListener('click',refresh);
  refresh();
 }
 async function refresh(){
  const label=$('#workticTikTokT1Status');const btn=$('#workticTikTokT1Connect');if(!label||!btn)return;
  try{
   const [ready,accounts]=await Promise.all([readiness(),window.api('/api/social/connections')]);
   const mine=Array.isArray(accounts)?accounts.filter(a=>a.platform==='tiktok'&&a.provider_mode==='official'):[];
   label.textContent=mine.length?`Cuenta(s): ${mine.map(a=>a.account_name+' ('+a.status+')').join(', ')} · ${ready.message}`:ready.message;
   btn.disabled=!ready.login_ready;
   btn.textContent=mine.length?'Conectar otra cuenta TikTok':'Conectar TikTok';
  }catch(e){label.textContent=e.message||'No se pudo verificar TikTok';btn.disabled=true;}
 }
 window.workticTikTokT1={connect,readiness,refresh};
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',createPanel);else createPanel();
 document.querySelector('aside nav button[data-view="socialhub"]')?.addEventListener('click',()=>setTimeout(refresh,200));
})();
