/* Worktic AI · T2 · separación editorial/mensajería y desconexión segura por empresa.
   No altera OAuth; las confirmaciones son una acción explícita del usuario. */
(()=>{'use strict';
 const $=s=>document.querySelector(s);
 const notify=s=>window.toast?window.toast(s):window.alert(s);
 const LABELS={facebook:'Facebook',instagram:'Instagram',tiktok:'TikTok',youtube:'YouTube',linkedin:'LinkedIn',telegram:'Telegram'};
 function newDialog(title,build){
  const dlg=document.createElement('dialog');dlg.className='wt-t2-dialog';
  const shell=document.createElement('div');shell.className='wt-t2-dialog-inner';
  const heading=document.createElement('h2');heading.textContent=title;
  const content=document.createElement('div');content.className='wt-t2-dialog-content';
  const actions=document.createElement('div');actions.className='wt-t2-dialog-actions';
  shell.append(heading,content,actions);dlg.append(shell);document.body.append(dlg);
  dlg.addEventListener('close',()=>dlg.remove(),{once:true});
  build({dlg,content,actions});dlg.showModal();return dlg;
 }
 const button=(label,cls,onClick)=>{const x=document.createElement('button');x.type='button';x.textContent=label;x.className=cls;x.addEventListener('click',onClick);return x;};
 const info=(parent,text)=>{const p=document.createElement('p');p.textContent=text;parent.append(p);};
 window.workticConfirmUnlink=({name='esta cuenta',kind='social'}={})=>new Promise(resolve=>{
  let settled=false;
  const messages={
   social:'Se retirará esta autorización del espacio de trabajo. La cuenta dejará de aparecer en Redes Sociales y, si también figura en Mis Canales, en esa vista. No se borrarán conversaciones ni contenidos históricos. Las publicaciones pendientes quedarán pausadas. La revocación externa se realiza en los ajustes del proveedor.',
   social_both:'Este mismo perfil TikTok se utiliza en Redes Sociales y Mis Canales. Desconectarlo retirará su autorización de AMBAS vistas, sin borrar contenido ni historial. No deshabilita Chat Web, Llamadas IA ni otros canales.',
   channel:'Se detendrá la atención de este canal y se retirará su autorización almacenada. Las conversaciones, contactos y oportunidades históricas se conservarán.',
   email:'Se retirará únicamente este buzón de Mis Canales, aunque ahora requiera reconexión. Se eliminarán sus credenciales locales y no recibirá ni enviará mensajes desde Worktic AI. El historial de correos, conversaciones y contactos permanecerá; podrás volver a autorizarlo cuando quieras.',
   webchat:'Se desactivará la recepción y la IA del chat web. El canal podrá volver a configurarse y se conservarán las conversaciones anteriores.',
   voice:'Se desactivarán las nuevas llamadas de este canal. No se eliminarán los registros históricos.',
   instagram_agent:'Se quitará el agente asignado para atención por Instagram. La cuenta seguirá conectada en Redes Sociales.'
  };
  const dlg=newDialog('¿'+(kind==='instagram_agent'?'Quitar asignación IA':'Desconectar cuenta')+'?',({dlg,content,actions})=>{
   const strong=document.createElement('strong');strong.textContent=name;content.append(strong);
   info(content,messages[kind]||messages.social);
   const note=document.createElement('small');note.textContent='Esta acción requiere confirmación y no elimina el historial de clientes.';content.append(note);
   actions.append(button('Cancelar','wt-t2-cancel',()=>dlg.close()),button(kind==='instagram_agent'?'Quitar asignación':'Sí, desconectar','wt-t2-danger',()=>{if(!settled){settled=true;resolve(true);}dlg.close();}));
  });
  dlg.addEventListener('close',()=>{if(!settled){settled=true;resolve(false);}},{once:true});
 });
 window.workticReauthorizeSocial=async platform=>{
  if(!Object.hasOwn(LABELS,platform))return notify('Plataforma no reconocida');
  if(platform==='telegram'){
   $('#socialConnect2')?.click();const select=$('#scPlatform');if(select){select.value='telegram';select.dispatchEvent(new Event('change'));}return;
  }
  if(platform==='tiktok')return window.workticTikTokT1?.connect?.();
  try{
   const res=await window.api('/api/social/oauth/start?platform='+encodeURIComponent(platform));
   const url=new URL(res.authorization_url);
   if(url.protocol!=='https:'||!['facebook.com','www.facebook.com','www.instagram.com','instagram.com','www.linkedin.com','www.youtube.com','accounts.google.com','www.google.com'].includes(url.hostname)){
    throw new Error('El proveedor devolvió una URL de autorización inesperada');
   }
   location.assign(url.href);
  }catch(e){notify(e.message||'No fue posible iniciar la reconexión');}
 };
 window.workticOpenSocialAccountDetails=async id=>{
  try{
   const list=await window.api('/api/social/connections');
   const c=Array.isArray(list)?list.find(x=>Number(x.id)===Number(id)):null;
   if(!c)return notify('La conexión ya no está disponible. Actualiza la página.');
   newDialog('Administrar '+(LABELS[c.platform]||c.platform),({dlg,content,actions})=>{
    const facts=[['Cuenta',c.account_name],['Estado',c.status],['Tipo de acceso',c.provider_mode==='official'?'API oficial':'Entorno de pruebas'],['Permisos',c.scopes||'Sin permisos registrados'],['Última sincronización',c.last_sync_at||'Sin registro']];
    for(const [name,value] of facts){const line=document.createElement('div');line.className='wt-t2-fact';const n=document.createElement('span');n.textContent=name;const v=document.createElement('b');v.textContent=String(value||'—');line.append(n,v);content.append(line);}
    if(c.platform==='tiktok')info(content,'TikTok: cuenta de perfil mediante Login Kit. El envío de videos y los mensajes privados requieren permisos independientes.');
    actions.append(button('Cerrar','wt-t2-cancel',()=>dlg.close()),button('Probar conexión','wt-t2-primary',async e=>{
     e.currentTarget.disabled=true;try{await window.testSocialConnection(Number(c.id));dlg.close();}finally{e.currentTarget.disabled=false;}
    }),button('Reconectar','wt-t2-primary',()=>{dlg.close();window.workticReauthorizeSocial(c.platform);}));
   });
  }catch(e){notify('No se pudieron consultar los detalles: '+e.message);}
 };
 // Sustituye la confirmación antigua, manteniendo el endpoint y los permisos existentes.
 window.deleteSocialConnection=async id=>{
  try{
   const records=await window.api('/api/social/connections');
   const c=Array.isArray(records)?records.find(x=>Number(x.id)===Number(id)):null;
   if(!c)return notify('Cuenta no encontrada en esta empresa');
   if(!await window.workticConfirmUnlink({name:(LABELS[c.platform]||c.platform)+' · '+c.account_name,kind:'social'}))return;
   const out=await window.api('/api/social/connections?id='+encodeURIComponent(id),{method:'DELETE'});
   if(!out?.ok)throw new Error('No se confirmó la desconexión');
   notify('Conexión retirada. Se conservan los contenidos y el historial.');
   if(typeof window.loadSocialHub==='function')await window.loadSocialHub();
   await window.workticUnified?.refresh?.();
  }catch(e){notify('No se pudo desconectar: '+e.message);}
 };
 function mountSocialHeader(){
  const root=$('#social-connections');if(!root)return;
  const banner=$('#workticTikTokT1Panel');if(banner)banner.remove();
  if($('#workticT2SocialSummary'))return;
  const top=document.createElement('div');top.id='workticT2SocialSummary';top.className='wt-t2-social-summary';
  const i=document.createElement('span');i.className='wt-t2-symbol';i.textContent='◎';i.setAttribute('aria-hidden','true');
  const text=document.createElement('div');const title=document.createElement('b');title.textContent='Conexiones editoriales y sociales';
  const p=document.createElement('small');p.textContent='Publicaciones, programación y análisis. Los perfiles compatibles también se muestran en Mis Canales para la atención y agentes IA.';
  text.append(title,p);
  const count=document.createElement('strong');count.id='workticT2SocialCount';count.textContent='— cuentas';
  top.append(i,text,count);
  const head=root.querySelector('.section-head');if(head)head.after(top);
  const grid=$('#socialConnectionsGrid');if(grid){
   let scheduled=false;
   const sync=()=>{scheduled=false;const total=grid.querySelectorAll(':scope > article.connection-card').length;count.textContent=total+' '+(total===1?'cuenta visible':'cuentas visibles');};
   const obs=new MutationObserver(()=>{if(!scheduled){scheduled=true;queueMicrotask(sync);}});
   obs.observe(grid,{childList:true});sync();
  }
 }
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',mountSocialHeader,{once:true});else mountSocialHeader();
})();
