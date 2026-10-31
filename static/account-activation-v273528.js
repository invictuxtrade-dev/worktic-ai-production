/* WorkticAI V27.35.28 · Self-hosted visual challenge + email verification UI.
   Loads only for newly enabled registrations; no changes to original login logic. */
(function () {
  'use strict';
  const form = document.getElementById('registerForm');
  const login = document.getElementById('loginForm');
  const card = document.querySelector('.auth-card');
  const tabs = document.querySelector('.tabs');
  if (!form || !login || !card || !tabs) return;
  const icons = {
    chat: '<rect x="3" y="3" width="18" height="15" rx="4"/><path d="m7 18-2 4v-5"/><path d="M8 9h8m-8 4h5"/>',
    spark: '<path d="m12 2 2.7 7.3L22 12l-7.3 2.7L12 22l-2.7-7.3L2 12l7.3-2.7z"/>',
    shield: '<path d="M12 2 4 6v5c0 5 3 8 8 11 5-3 8-6 8-11V6z"/><path d="m9 12 2 2 4-4"/>',
    calendar: '<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M8 3v4m8-4v4M3 10h18"/>',
    bolt: '<path d="m13 2-9 11h7l-1 9 10-12h-7z"/>',
    heart: '<path d="M20.8 5a5.5 5.5 0 0 0-7.8 0L12 6 11 5a5.5 5.5 0 1 0-7.8 7.8L12 22l8.8-9.2A5.5 5.5 0 0 0 20.8 5z"/>'
  };
  const es = {chat:'Chat',spark:'Estrella',shield:'Escudo',calendar:'Calendario',bolt:'Rayo',heart:'Corazón'};
  const en = {chat:'Chat',spark:'Star',shield:'Shield',calendar:'Calendar',bolt:'Lightning',heart:'Heart'};
  let active = false, challenge = null, selected = -1, busy = false, mode = 'pending';
  // UI cooldown is restored across refreshes. PostgreSQL enforces the 60-second limit.
  const cooldownKey = 'wt29-resend-until';
  let resendUntil = 0, cooldownTimer = null;
  try { resendUntil = Math.max(0, Number(sessionStorage.getItem(cooldownKey)) || 0); } catch (_) {}
  const formatCountdown = seconds => (isEnglish() ? 'Resend link in ' : 'Reenviar enlace en ') + seconds + 's';
  function updateCooldown() {
    const resendButton = document.getElementById('wt28ResendButton');
    if (!resendButton) return;
    const remaining = Math.max(0, Math.ceil((resendUntil - Date.now()) / 1000));
    resendButton.disabled = busy || remaining > 0 || mode === 'success' || mode === 'verifying';
    resendButton.textContent = remaining > 0 ? formatCountdown(remaining) : text('Reenviar enlace','Resend link');
    if (!remaining && cooldownTimer !== null) {
      clearInterval(cooldownTimer); cooldownTimer = null;
      try { sessionStorage.removeItem(cooldownKey); } catch (_) {}
    }
  }
  function startCooldown(seconds = 60) {
    resendUntil = Math.max(resendUntil, Date.now() + seconds * 1000);
    try { sessionStorage.setItem(cooldownKey, String(resendUntil)); } catch (_) {}
    updateCooldown();
    if (cooldownTimer === null) cooldownTimer = setInterval(updateCooldown, 1000);
  }
  const isEnglish = () => document.documentElement.lang === 'en';
  const label = k => (isEnglish() ? en : es)[k] || k;
  const text = (esText, enText) => isEnglish() ? enText : esText;
  const captcha = document.createElement('section');
  captcha.className = 'wt28-captcha'; captcha.hidden = true;
  captcha.innerHTML = '<div class="wt28-captcha__head"><span class="wt28-captcha__mark" aria-hidden="true">✦</span><div><strong id="wt28CaptchaTitle">Verificación inteligente</strong><small id="wt28CaptchaHint"></small></div></div>'+
    '<div class="wt28-captcha__tiles" id="wt28Tiles" role="group" aria-label="Elige el símbolo solicitado"></div>'+
    '<div class="wt28-captcha__footer"><small id="wt28CaptchaStatus" aria-live="polite"></small><button type="button" class="wt28-captcha__refresh" id="wt28Refresh">↻ Nuevo desafío</button></div>';
  const website = document.createElement('label'); website.className = 'wt28-honey'; website.setAttribute('aria-hidden','true');
  website.innerHTML = 'Página web <input tabindex="-1" autocomplete="off" name="website" type="text">';
  const button = form.querySelector('button[type="submit"]');
  if (!button) return;
  button.insertAdjacentElement('beforebegin', captcha);
  form.append(website);
  const panel = document.createElement('section'); panel.id='wt28Activation'; panel.className='wt28-activation'; panel.hidden=true;
  panel.innerHTML = '<div class="wt28-activation__icon" aria-hidden="true">✉</div><h3 id="wt28Title"></h3><p id="wt28Description"></p>'+
    '<form class="wt28-activation__form" id="wt28ResendForm"><label for="wt28ResendEmail">Correo electrónico</label><input id="wt28ResendEmail" type="email" autocomplete="email" required maxlength="254"><button type="submit" id="wt28ResendButton">Reenviar enlace</button></form>'+
    '<div class="wt28-activation__status" id="wt28Status" hidden></div><div class="wt28-activation__error" id="wt28Error" role="alert" hidden></div>'+
    '<button class="wt28-activation__back" id="wt28Back" type="button">Volver al acceso</button>';
  form.insertAdjacentElement('afterend', panel);
  const resendLogin = document.createElement('button'); resendLogin.type = 'button'; resendLogin.className='wt28-resend-link';
  resendLogin.textContent='¿No has activado tu cuenta? Reenviar correo'; resendLogin.hidden = true;
  login.insertAdjacentElement('afterend',resendLogin);
  const $ = id => document.getElementById(id);
  const endpoint = (path, body) => fetch(path, {method:'POST',credentials:'same-origin',cache:'no-store',headers:{'Content-Type':'application/json','Accept':'application/json'},body:JSON.stringify(body)}).then(async r=>{
    let data={};try{data=await r.json()}catch(_){}if(!r.ok)throw Error(data.error||text('No fue posible completar la solicitud','Request failed'));return data;
  });
  const showPanel = (next,email) => {
    mode=next;panel.hidden=false;form.hidden=true;login.hidden=true;tabs.hidden=true;resendLogin.hidden=true;
    if (typeof email==='string') $('wt28ResendEmail').value=email;
    $('wt28Status').hidden=true;$('wt28Error').hidden=true;
    $('wt28ResendForm').hidden=(next==='success'||next==='verifying');
    $('wt28Back').textContent=next==='success'?text('Ingresar a Worktic AI','Sign in to Worktic AI'):text('Volver al acceso','Back to sign in');
    panel.querySelector('.wt28-activation__icon').textContent=next==='success'?'✓':'✉';
    const titles = {pending:text('Revisa tu correo','Check your email'),success:text('¡Cuenta activada!','Account activated!'),verifying:text('Activando tu cuenta…','Activating your account…'),resend:text('Reenviar activación','Resend verification')};
    const descriptions = {pending:text('Hemos enviado un enlace de activación. Es válido durante 24 horas. Si no activas tu cuenta dentro de 72 horas desde el registro, el registro provisional se eliminará.','We have sent a 24-hour activation link. The provisional registration expires after 72 hours.'),success:text('Tu correo está verificado. Ya puedes ingresar a Worktic AI. ¡Bienvenido!','Your email is verified. You can now sign in.'),verifying:text('Estamos comprobando tu enlace seguro.','Checking your secure link.'),resend:text('Escribe el correo de tu registro para solicitar otro enlace.','Enter your registration email to request another link.')};
    $('wt28Title').textContent=titles[next];$('wt28Description').textContent=descriptions[next];
    if(next==='pending')startCooldown(60);else updateCooldown();
  };
  const showError = msg => { $('wt28Error').hidden=false;$('wt28Error').textContent=msg; };
  $('wt28Back').onclick = () => { panel.hidden=true;tabs.hidden=false;login.hidden=false;form.hidden=true;resendLogin.hidden=!active;const t=$('loginTab');if(t)t.click(); };
  resendLogin.onclick=()=>showPanel('resend',login.querySelector('[name="email"]')?.value||'');
  $('wt28ResendForm').addEventListener('submit',async e=>{
    e.preventDefault();if(busy||Date.now()<resendUntil)return;busy=true;updateCooldown();
    try{await endpoint('/api/auth/email/resend',{email:$('wt28ResendEmail').value.trim()});$('wt28Error').hidden=true;$('wt28Status').hidden=false;$('wt28Status').textContent=text('Solicitud recibida. Si tu cuenta está pendiente, revisa tu correo y la carpeta de spam.','Request received. If your account is pending, check your inbox and spam folder.');startCooldown(60);}
    catch(err){showError(err.message)}finally{busy=false;updateCooldown()}
  });
  const loadCaptcha=async()=>{
    challenge=null;selected=-1; $('wt28Tiles').replaceChildren();$('wt28CaptchaStatus').textContent='';
    try{
      const r=await fetch('/api/auth/captcha/challenge',{cache:'no-store'});if(!r.ok)throw Error('challenge unavailable');challenge=await r.json();
      $('wt28CaptchaTitle').textContent=text('Verificación inteligente','Smart verification');
      $('wt28CaptchaHint').textContent=text('Selecciona: ','Select: ')+label(challenge.target);
      challenge.tiles.forEach((png,i)=>{
        const tile=document.createElement('button');tile.type='button';tile.className='wt28-tile';tile.setAttribute('aria-pressed','false');
        tile.setAttribute('aria-label',text('Opción ','Option ')+(i+1));
const img=document.createElement('img');
img.src=png;
img.alt='';
const span=document.createElement('span');
span.textContent=text('Opción ','Option ')+(i+1);
tile.append(img,span);
        tile.addEventListener('click',()=>{selected=i;[...$('wt28Tiles').children].forEach((b,j)=>{b.classList.toggle('is-selected',i===j);b.setAttribute('aria-pressed',i===j?'true':'false')});$('wt28CaptchaStatus').textContent=text('Selección lista ✓','Selection ready ✓')});
        $('wt28Tiles').append(tile)
      });
    }catch(_){$('wt28CaptchaStatus').textContent=text('Desafío no disponible. Intenta recargar.','Challenge unavailable. Refresh to retry.')}
  };
  $('wt28Refresh').onclick=loadCaptcha;
  document.getElementById('registerTab')?.addEventListener('click',function(){if(active&&!challenge)loadCaptcha()});
  window.addEventListener('submit',async e=>{
    if (!active || e.target!==form)return;
    e.preventDefault();e.stopImmediatePropagation();if(busy)return;
    const email=form.querySelector('[name="email"]')?.value.trim()||'';
    const notice=document.getElementById('authError');if(notice)notice.textContent='';
    if(!challenge||selected<0){$('wt28CaptchaStatus').textContent=text('Resuelve el desafío antes de continuar.','Complete the challenge to continue.');return}
    busy=true;button.disabled=true;
    const data=Object.fromEntries(new FormData(form).entries());data.captcha_id=challenge.id;data.captcha_choice=selected;
    try{
      const r=await endpoint('/api/auth/register',data);
      if(r.pending_verification){showPanel('pending',email)}else{throw Error(text('No se recibió confirmación de registro pendiente.','Pending verification not confirmed.'))}
    }catch(err){if(notice)notice.textContent=err.message;await loadCaptcha()}
    finally{busy=false;button.disabled=false}
  },true);
  (async()=>{
    const fragment = (location.hash||'').match(/^#activate=([^&]+)$/);
    let token='';if(fragment){try{token=decodeURIComponent(fragment[1])}catch(_){}history.replaceState(null,'',location.pathname+location.search)}
    try{const r=await fetch('/api/auth/email/status',{cache:'no-store'});const d=await r.json();active=!!d.enabled;}catch(_){active=false}
    if(!active)return;
    if(resendUntil>Date.now()){updateCooldown();cooldownTimer=setInterval(updateCooldown,1000);}
    captcha.hidden=false;resendLogin.hidden=false;
    if(form.hidden!==true)loadCaptcha();
    if(token){showPanel('verifying');try{await endpoint('/api/auth/email/verify',{token});showPanel('success')}catch(err){showPanel('resend');showError(err.message)}}
  })();
})();
