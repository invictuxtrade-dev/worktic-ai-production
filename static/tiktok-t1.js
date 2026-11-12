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
 // T2: TikTok es una cuenta editorial; su estado se muestra en la tarjeta social premium.
 // Sin banner de conexión duplicado y sin componentes de mensajería en Mis Canales.
 async function refresh(){return readiness();}
 window.workticTikTokT1={connect,readiness,refresh};

})();
