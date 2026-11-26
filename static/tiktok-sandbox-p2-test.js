/* Sandbox P2 test bench. Explicit manual actions only; no autopublishing. */
(()=>{'use strict';
const $=(id)=>document.getElementById(id);
const api='/api/social/tiktok/sandbox';let ready=false, canSubmit=false, hasCreator=false, privateAllowed=false;
let requestId='';let alreadySubmitted=false;
function output(obj){$('result').textContent=typeof obj==='string'?obj:JSON.stringify(obj,null,2)}
function errorText(err){return err instanceof Error?err.message:String(err)}
async function call(action,method='GET',data){
 const options={method,credentials:'same-origin',headers:{'Accept':'application/json'}};
 if(method==='POST'){options.headers['Content-Type']='application/json';options.headers['X-Worktic-TikTok-Sandbox']=action;options.body=JSON.stringify(data||{})}
 const response=await fetch(api+'?action='+encodeURIComponent(action),options);
 const body=await response.json().catch(()=>({error:'El servidor no devolvió JSON'}));
 if(!response.ok)throw new Error(body.error||'Error HTTP '+response.status);
 return body;
}
function updateButton(){
 const mode=$('mode').value;
 $('submit').disabled=!(ready && canSubmit && !alreadySubmitted && $('consent').checked && (mode==='inbox'||(hasCreator && privateAllowed)));
 $('submit').textContent=mode==='inbox'?'Enviar a bandeja TikTok':'Publicar prueba privada (Solo yo)';
}
async function refresh(){
 try{const data=await call('readiness');ready=!!data.connected;canSubmit=!!data.can_submit;
 $('status').textContent=`Sandbox: activo · Cuenta: ${data.connected?data.account_name+' (conectada)':'sin autorizar'} · Envíos de prueba: ${canSubmit?'habilitados':'desactivados'} · Permisos: ${data.granted_scopes||'—'}`;
 $('connect').disabled=false;
 }catch(err){ready=false;canSubmit=false;$('status').textContent='No disponible: '+errorText(err)}updateButton();
}
async function authorize(){try{const data=await call('authorize','POST',{});if(!data.authorization_url)throw new Error('Falta la URL OAuth');window.location.assign(data.authorization_url)}catch(err){output(errorText(err))}}
async function creator(){try{const data=await call('creator');hasCreator=true;privateAllowed=!!data.private_allowed;
 $('privacy').textContent='Cuenta: @'+(data.creator?.creator_username||'desconocida')+' · Privacidades: '+(data.creator?.privacy_level_options||[]).join(', ')+' · Solo yo: '+(privateAllowed?'permitido':'no permitido');
 output('creator_info cargado correctamente. No se ha enviado contenido.');
 }catch(err){hasCreator=false;privateAllowed=false;output(errorText(err))}updateButton();}
function newRequest(){alreadySubmitted=false;requestId=crypto.randomUUID();$('request').textContent=requestId;output('Nueva solicitud creada. No se ha enviado contenido.');updateButton()}
async function submit(){
 if(!confirm('Se enviará el MP4 exclusivamente a TU cuenta TikTok del Sandbox. En Direct Post será privado (Solo yo). ¿Confirmas?'))return;
 const payload={video_url:$('video_url').value.trim(),title:$('title').value.trim(),privacy_level:'SELF_ONLY',mode:$('mode').value,request_id:requestId,ai_generated:$('ai').checked,brand_organic:$('brand').checked,confirmed:$('consent').checked};
 alreadySubmitted=true;$('submit').disabled=true;
 try{const result=await call('submit','POST',payload);output(result)}catch(err){output('Error o respuesta no confirmada: '+errorText(err)+'\nNO vuelvas a enviar sin comprobar el estado.');}
 // Intentionally keep the same id and block reuse. Creating a new request requires a separate action.
 updateButton();
}
async function checkRequest(){if(!requestId){output('No hay solicitud para consultar');return}
 try{const r=await fetch(api+'?action=status&request_id='+encodeURIComponent(requestId),{credentials:'same-origin'});const data=await r.json();if(!r.ok)throw Error(data.error||'HTTP '+r.status);output(data)}catch(err){output(errorText(err))}}
$('refresh').onclick=refresh;$('connect').onclick=authorize;$('creator').onclick=creator;
$('mode').onchange=updateButton;$('consent').onchange=updateButton;
$('submit').onclick=submit;$('check').onclick=checkRequest;$('reset').onclick=()=>{if(confirm('¿Crear un nuevo identificador de solicitud? No reenvíes videos con estado desconocido.'))newRequest()};
$('video_url').onchange=()=>{$('preview').src=$('video_url').value.trim()};$('preview').src=$('video_url').value.trim();
const oauth=new URLSearchParams(location.search).get('oauth');if(oauth){output('Resultado OAuth: '+oauth+'. Consulta el estado para confirmar.');history.replaceState(null,'',location.pathname)}
newRequest();refresh();
})();
