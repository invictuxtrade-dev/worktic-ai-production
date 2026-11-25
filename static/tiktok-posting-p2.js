/* Worktic AI TikTok P2 — guarded manual Content Posting, not Autopilot.
   OFF until approved TikTok scopes and WORKTIC_TIKTOK_POSTING_P2_ENABLED=1.
*/
(()=>{
'use strict';
const path='/api/social/tiktok/p2';
const $=id=>document.getElementById(id);
const escape=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const tiktokList=()=>{
 try{return (socialConnections||[]).filter(c=>c.platform==='tiktok'&&c.status==='connected'&&c.provider_mode==='official')}catch{return []}
};
const hasScope=(scopes,name)=>String(scopes||'').split(/[\s,]+/).includes(name);
const announce=text=>{$('tikP2Message').textContent=text};
let creator=null;
let currentPublishId='';
let mode='direct';
let videoDuration=0;
let checking=false;

async function getJSON(url){return api(url)}

async function beginOAuth(){
 if(!confirm('TikTok solicitará autorización adicional para video.upload y video.publish. Tu cuenta actual seguirá conectada si cancelas. ¿Continuar?'))return;
 try{const a=await getJSON('/api/social/oauth/start?platform=tiktok&mode=posting');if(!a.authorization_url)throw Error('No se recibió la URL OAuth');window.location.assign(a.authorization_url)}catch(e){announce(e.message||'No se pudo iniciar autorización')}
}

function updateUI(){
 mode=$('tikP2Mode').value;
 $('tikP2Direct').hidden=mode!=='direct';
 $('tikP2InboxNote').hidden=mode!=='inbox';
 $('tikP2BrandChoices').hidden=!$('tikP2Disclosure').checked;
 if(mode==='direct')refreshCreator();else {creator=null;announce('Envío a TikTok: el creador deberá abrir la notificación en TikTok y completar allí la publicación.');}
}

async function refreshCreator(){
 creator=null;
 $('tikP2Privacy').innerHTML='<option value="">Cargando opciones del creador...</option>';
 const id=Number($('tikP2Account').value||0);if(!id)return;
 const account=tiktokList().find(c=>Number(c.id)===id);
 if(!account||!hasScope(account.scopes,'video.publish')){
  $('tikP2Privacy').innerHTML='<option value="">Debes autorizar video.publish</option>';
  announce('Cuenta con Login Kit solamente. Usa «Autorizar publicación» después de que TikTok apruebe el producto.');return;
 }
 try{
  creator=await getJSON(`${path}?action=creator&connection_id=${encodeURIComponent(id)}`);
  $('tikP2Creator').textContent=`Cuenta destinataria: @${creator.creator_username} (${creator.creator_nickname})`;
  const sel=$('tikP2Privacy');sel.innerHTML='<option value="">Elige privacidad (obligatorio)</option>';
  const names={PUBLIC_TO_EVERYONE:'Todos',MUTUAL_FOLLOW_FRIENDS:'Amigos',FOLLOWER_OF_CREATOR:'Seguidores',SELF_ONLY:'Solo yo'};
  for(const p of creator.privacy_level_options||[]){const o=document.createElement('option');o.value=p;o.textContent=names[p]||p;sel.appendChild(o)}
  [['tikP2Comments','comment_disabled'],['tikP2Duet','duet_disabled'],['tikP2Stitch','stitch_disabled']].forEach(([el,field])=>{const ch=$(el);ch.checked=false;ch.disabled=!!creator[field];ch.closest('label').title=ch.disabled?'Esta interacción no está disponible para el creador':''});
  const max=Number(creator.max_video_post_duration_sec||0);
  announce(`Opciones del creador cargadas. ${max?`Duración máxima: ${max} segundos.`:''} Elige privacidad e interacciones manualmente.`);
 }catch(e){$('tikP2Privacy').innerHTML='<option value="">No se pudieron cargar opciones</option>';announce(e.message||'Fallo consultando creator_info')}
}

function validateConsent(){
 if(!$('tikP2Consent').checked)throw Error('Confirma expresamente el envío y las condiciones de música de TikTok');
 if(!$('tikP2Title').value.trim())throw Error('Escribe el texto del video');
 if(!$('tikP2VideoURL').value.startsWith('https://'))throw Error('Necesitas un video disponible por HTTPS');
 if($('tikP2Disclosure').checked&&!$('tikP2Own').checked&&!$('tikP2Branded').checked)throw Error('Indica si promocionas tu negocio o una marca de terceros');
 if(!$('tikP2Disclosure').checked&&($('tikP2Own').checked||$('tikP2Branded').checked))throw Error('Activa la divulgación comercial cuando indiques una marca');
 if(mode==='direct'){
  if(!creator)throw Error('Primero carga las opciones del creador desde TikTok');
  if(!$('tikP2Privacy').value)throw Error('Selecciona explícitamente la privacidad');
  if($('tikP2Branded').checked&&$('tikP2Privacy').value==='SELF_ONLY')throw Error('El contenido de terceros no puede configurarse Solo yo');
  if(videoDuration<=0)throw Error('No se pudo verificar la duración del video. Utiliza un archivo MP4 público accesible para vista previa.');
  if(creator.max_video_post_duration_sec && videoDuration>creator.max_video_post_duration_sec)throw Error('El video supera la duración admitida por esta cuenta TikTok');
 }
}

async function submit(){
 const button=$('tikP2Send');if(button.disabled)return;
 try{
  validateConsent();button.disabled=true;button.textContent='Enviando a TikTok...';
  const payload={
   connection_id:Number($('tikP2Account').value),mode:mode,
   video_url:$('tikP2VideoURL').value.trim(),title:$('tikP2Title').value.trim(),
   privacy_level:mode==='direct'?$('tikP2Privacy').value:'',
   allow_comment:mode==='direct'&&$('tikP2Comments').checked,
   allow_duet:mode==='direct'&&$('tikP2Duet').checked,
   allow_stitch:mode==='direct'&&$('tikP2Stitch').checked,
   brand_organic:$('tikP2Disclosure').checked&&$('tikP2Own').checked,
   brand_content:$('tikP2Disclosure').checked&&$('tikP2Branded').checked,
   ai_generated:$('tikP2AIGenerated').checked,confirmed:true
  };
  const res=await api(`${path}?action=submit`,{method:'POST',headers:{'X-Worktic-TikTok-Consent':'p2'},body:JSON.stringify(payload)});
  currentPublishId=res.publish_id||'';
  $('tikP2Result').hidden=false;
  $('tikP2Result').textContent=`Solicitud recibida por TikTok: ${currentPublishId}. Estado: EN PROCESO. No vuelvas a enviarlo; utiliza «Consultar estado».${res.tracking_saved?'':' Atención: el historial local no se guardó; comprueba el estado dentro de TikTok antes de intentar otra vez.'}`;
  $('tikP2Check').disabled=!res.tracking_saved;
  $('tikP2Send').hidden=true;
 }catch(e){announce(e.message||'No se pudo enviar; si TikTok recibió algo, verifica antes de reintentar')}
 finally{button.disabled=false;button.textContent='Confirmar y enviar a TikTok'}
}
async function check(){
 if(checking||!currentPublishId)return;
 checking=true;
 try{const r=await getJSON(`${path}?action=status&publish_id=${encodeURIComponent(currentPublishId)}`);let msg=`Estado TikTok: ${r.status}`;
  if(r.status==='SEND_TO_USER_INBOX')msg='Video recibido en TikTok. Abre la notificación en la bandeja de TikTok para terminar la publicación.';
  if(r.status==='PUBLISH_COMPLETE')msg='TikTok confirmó la publicación.';
  if(r.status==='FAILED')msg=`TikTok informó un fallo: ${r.fail_reason||'sin detalle'}`;
  $('tikP2Result').textContent=msg;
 }catch(e){announce(e.message||'No se pudo consultar estado')}finally{checking=false}
}

async function open(){
 const networks=[...document.querySelectorAll('.platform-option input:checked')].map(x=>x.value);
 if(networks.length!==1||networks[0]!=='tiktok')return alert('Para confirmar la exportación a TikTok debes seleccionar solo TikTok. Puedes guardar publicaciones de otras redes por separado.');
 const accounts=tiktokList();
 if(!accounts.length)return alert('Conecta una cuenta oficial de TikTok antes de continuar.');
 const title=$('spMaster')?.value?.trim()||'';const mediaInput=($('spMedia')?.value||'').trim();let video='';try{video=new URL(mediaInput,location.origin).href}catch{video=mediaInput}
 if(!video)return alert('Selecciona primero un video en el Composer.');
 let readiness;try{readiness=await getJSON(`${path}?action=readiness`)}catch(e){return alert(e.message||'No pudimos comprobar la disponibilidad de TikTok')}
 if(!readiness.enabled)return alert('Content Posting API sigue pendiente de aprobación para Worktic AI. Puedes guardar borradores, pero todavía no enviar a TikTok.');
 creator=null;currentPublishId='';mode='direct';videoDuration=0;
 modal(`<div class="tik-p2-modal"><h2>Exportar video a TikTok</h2><p>Revisa el video, los permisos y elige qué publicar. TikTok requiere tu confirmación expresa en cada envío.</p>
 <label>Cuenta de TikTok<select id="tikP2Account">${accounts.map(c=>`<option value="${Number(c.id)}">${escape(c.account_name)}</option>`).join('')}</select></label>
 <p id="tikP2Creator" class="muted">Cargando creador...</p>
 <button type="button" class="secondary" id="tikP2Authorize">Autorizar publicación en TikTok</button>
 <label>Modalidad<select id="tikP2Mode"><option value="direct">Publicar directamente</option><option value="inbox">Enviar a TikTok para editar y publicar allí</option></select></label>
 <label>Descripción y hashtags (editable)<textarea rows="3" id="tikP2Title">${escape(title)}</textarea></label>
 <label>URL HTTPS del video<input type="url" id="tikP2VideoURL" value="${escape(video)}"></label>
 <video id="tikP2Preview" preload="metadata" muted controls playsinline style="width:100%;max-height:180px;background:#161620;border-radius:12px"></video>
 <p class="muted" id="tikP2Duration">Comprobando duración...</p>
 <div id="tikP2Direct"><label>Privacidad (sin opción predeterminada)<select id="tikP2Privacy"><option value="">Elige privacidad...</option></select></label>
 <div class="tik-p2-checks"><label><input type="checkbox" id="tikP2Comments"> Permitir comentarios</label><label><input type="checkbox" id="tikP2Duet"> Permitir Dúos</label><label><input type="checkbox" id="tikP2Stitch"> Permitir Pegar</label></div></div>
 <p id="tikP2InboxNote" hidden>El video se enviará a la bandeja de TikTok para editar y completar la publicación desde la aplicación.</p>
 <label><input type="checkbox" id="tikP2Disclosure"> Este contenido promociona un negocio, producto o servicio</label>
 <div id="tikP2BrandChoices" hidden><label><input type="checkbox" id="tikP2Own"> Mi propia marca (contenido promocional)</label><label><input type="checkbox" id="tikP2Branded"> Marca de terceros (colaboración pagada)</label></div>
 <label><input type="checkbox" id="tikP2AIGenerated"> El video fue generado con IA</label>
 <p>Al publicar, aceptas la <a href="https://www.tiktok.com/legal/page/global/music-usage-confirmation/en" target="_blank" rel="noopener">confirmación de uso de música de TikTok</a> y, si corresponde, la <a href="https://www.tiktok.com/legal/page/global/branded-content-policy/en" target="_blank" rel="noopener">política de contenido de marca</a>.</p>
 <label><input type="checkbox" id="tikP2Consent"> Confirmo que revisé este video y autorizo expresamente su envío a mi cuenta TikTok</label>
 <p id="tikP2Message" role="status" class="muted"></p><p id="tikP2Result" role="status" hidden></p>
 <div class="actions"><button type="button" id="tikP2Send">Confirmar y enviar a TikTok</button><button type="button" class="secondary" id="tikP2Check" disabled>Consultar estado</button><button type="button" class="secondary" id="tikP2Cancel">Cerrar</button></div></div>`);
 $('tikP2Account').addEventListener('change',()=>{updateUI()});$('tikP2Mode').addEventListener('change',updateUI);
 $('tikP2Authorize').onclick=beginOAuth;$('tikP2Send').onclick=submit;$('tikP2Check').onclick=check;
 $('tikP2Cancel').onclick=()=>document.querySelector('#modal')?.close();
 $('tikP2Disclosure').onchange=()=>{if(!$('tikP2Disclosure').checked){$('tikP2Own').checked=false;$('tikP2Branded').checked=false}updateUI()};
 $('tikP2VideoURL').onchange=refreshVideo;
 $('tikP2Branded').onchange=()=>{if($('tikP2Branded').checked&&$('tikP2Privacy').value==='SELF_ONLY'){$('tikP2Privacy').value='';announce('La colaboración pagada no admite Solo yo. Elige otra privacidad.') }};
 refreshVideo();updateUI();
}
function refreshVideo(){
 videoDuration=0;const el=$('tikP2Preview');el.src=$('tikP2VideoURL').value.trim();el.onloadedmetadata=()=>{videoDuration=el.duration||0;$('tikP2Duration').textContent=`Duración del video: ${Math.ceil(videoDuration)} segundos`};el.onerror=()=>{$('tikP2Duration').textContent='No se pudo cargar la vista previa de este video; comprueba su URL pública.'};
}

// Capture before the legacy Composer's onclick handler, without altering other providers.
document.addEventListener('click',e=>{
 const button=e.target.closest?.('#spSave');if(!button)return;
 const action=document.getElementById('spAction')?.value;
 if(!['publish','schedule'].includes(action))return;
 const platforms=[...document.querySelectorAll('.platform-option input:checked')].map(x=>x.value);
 if(!platforms.includes('tiktok'))return;
 e.preventDefault();e.stopImmediatePropagation();
 if(action==='schedule')return alert('TikTok requiere una confirmación explícita en cada publicación. Por ahora guarda el borrador y expórtalo manualmente cuando quieras publicar.');
 open();
},true);

// An informational hint appears only within the social Composer.
const observer=new MutationObserver(()=>{
 const el=document.getElementById('socialComposerMount');if(!el||document.getElementById('tikP2ComposerHint'))return;
 const actions=el.querySelector('.composer-actions');if(!actions)return;
 const hint=document.createElement('p');hint.id='tikP2ComposerHint';hint.className='muted';hint.textContent='TikTok: la exportación requiere permiso de publicación, selección manual de privacidad y confirmación por cada video; no sale por Autopilot.';actions.parentElement.append(hint);
});
const composerRoot=document.getElementById('socialComposerMount');
if(composerRoot)observer.observe(composerRoot,{subtree:true,childList:true});
})();
