/* WorkticAI embedded web chat pilot. No AI replies or tracking cookies. */
(() => {
  const script = document.currentScript;
  if (!script) return;
  const mount = () => {
  const key = script.getAttribute('data-key');
  if (!key || !/^[a-zA-Z0-9_\-]{20,100}$/.test(key)) return;
  const api = new URL(script.src).origin + '/api/mobile/webchat/public';
  const storageKey = 'worktic-webchat-v1:' + key;
  let token = sessionStorage.getItem(storageKey) || '';
  let open = false;
  const el = (type, text) => { const node = document.createElement(type); if (text !== undefined) node.textContent = text; return node; };
  const addCSS = () => {
    const style = el('style');
    style.textContent = `#wtc-chat-btn{position:fixed;bottom:24px;right:24px;z-index:2147483646;border:0;border-radius:50%;height:56px;width:56px;background:linear-gradient(120deg,#337cff,#8c3ff8);color:#fff;font:30px Arial;box-shadow:0 8px 25px #0005;cursor:pointer}
      #wtc-chat-box{position:fixed;bottom:91px;right:22px;z-index:2147483646;width:min(350px,calc(100vw - 35px));height:min(510px,calc(100vh - 115px));box-sizing:border-box;background:#111a2c;color:#f6f8ff;border:1px solid #34466c;border-radius:20px;box-shadow:0 15px 45px #0008;display:flex;flex-direction:column;font:14px system-ui,sans-serif;overflow:hidden}
      #wtc-chat-box[hidden]{display:none}#wtc-chat-box header{padding:16px;background:linear-gradient(90deg,#337cff,#8c3ff8);font-weight:800}
      #wtc-chat-box form{padding:12px;display:flex;flex-direction:column;gap:8px}#wtc-chat-box input,#wtc-chat-box textarea{background:#202d44;border:1px solid #455a78;border-radius:10px;color:white;padding:11px;box-sizing:border-box;width:100%;font:inherit}
      #wtc-chat-box button{background:#337cff;color:white;border:0;border-radius:10px;padding:11px;cursor:pointer;font:inherit}
      #wtc-msgs{flex:1;padding:12px;overflow:auto}#wtc-msgs p{padding:10px;border-radius:12px;background:#202d44;white-space:pre-wrap;overflow-wrap:anywhere;margin:8px 0}#wtc-msgs p.out{background:#34256c;margin-left:12%}
      #wtc-status{color:#b8c4db;font-size:11px;min-height:15px}#wtc-privacy{font-size:11px;color:#d3dff0}`;
    document.head.append(style);
  };
  addCSS();
  const launcher = el('button','✦'); launcher.id='wtc-chat-btn';launcher.setAttribute('aria-label','Abrir chat WorkticAI');document.body.append(launcher);
  const box=el('section');box.id='wtc-chat-box';box.hidden=true; box.append(el('header','Chat con nuestro equipo'));
  const messages=el('div');messages.id='wtc-msgs';box.append(messages);
  const form=el('form');const status=el('div');status.id='wtc-status';
  const name=el('input');name.placeholder='Tu nombre';name.required=true;name.maxLength=100;
  const consent=el('label');consent.id='wtc-privacy';const checkbox=el('input');checkbox.type='checkbox';checkbox.required=true;checkbox.style.width='auto';consent.append(checkbox,document.createTextNode(' Acepto que la empresa procese mis mensajes para responder esta consulta.'));
  const editor=el('textarea');editor.placeholder='Escribe tu mensaje…';editor.maxLength=1600;editor.rows=2;
  const submit=el('button','Comenzar chat');submit.type='submit';
  form.append(name,consent,editor,submit,status);box.append(form);document.body.append(box);
  function current(){ name.hidden=!!token;name.required=!token;consent.hidden=!!token;checkbox.required=!token;submit.textContent=token?'Enviar mensaje':'Comenzar chat'; }
  current();
  async function request(path, method, data){
    const res=await fetch(api+path+'?key='+encodeURIComponent(key),{method,headers:{'Content-Type':'application/json', ...(token?{'X-Worktic-Session':token}:{})},body:data?JSON.stringify(data):undefined,credentials:'omit',cache:'no-store'});
    if(!res.ok) throw new Error((await res.text()).slice(0,160));return res.json();
  }
  async function refresh(){ if(!open||!token)return;try{const result=await request('/messages','GET');messages.replaceChildren();for(const m of result.messages.reverse()){const p=el('p',m.text);if(m.direction==='out')p.className='out';messages.append(p);}messages.scrollTop=messages.scrollHeight;}catch(e){status.textContent='No se pudo actualizar: '+e.message;} }
  launcher.onclick=()=>{open=!open;box.hidden=!open;if(open)refresh();};
  form.addEventListener('submit',async e=>{
    e.preventDefault();submit.disabled=true;status.textContent='Enviando…';
    try{
      if(!token){const result=await request('/start','POST',{name:name.value.trim(),consent:checkbox.checked});token=result.session;sessionStorage.setItem(storageKey,token);current();}
      if(editor.value.trim()) await request('/messages','POST',{text:editor.value.trim()});
      editor.value='';status.textContent='Mensaje enviado';await refresh();
    }catch(err){status.textContent='Error: '+err.message;}finally{submit.disabled=false;}
  });
  window.setInterval(()=>{if(open&&token)refresh();},5500);
  };
  if(document.readyState === 'loading') document.addEventListener('DOMContentLoaded',mount,{once:true});
  else mount();
})();
