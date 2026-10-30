/* Worktic AI V27.35.25 — purely UI enhancements. Authentication & registration remain owned by /login.js. */
(function(){'use strict';
  function qs(s){return document.querySelector(s);}
  function lang(){try{if(typeof window.wtLang==='function')return window.wtLang()==='en'?'en':'es';}catch(e){}return document.documentElement.lang==='en'?'en':'es';}
  function applyLabels(){var en=lang()==='en';document.querySelectorAll('[data-wt24-en]').forEach(function(el){
     if(!el.hasAttribute('data-wt24-es'))el.setAttribute('data-wt24-es',el.innerHTML);
     el.innerHTML=en?el.getAttribute('data-wt24-en'):el.getAttribute('data-wt24-es');
   });
   var lbl=en?'EN':'ES'; document.querySelectorAll('.wt25-language-label').forEach(function(el){el.textContent=lbl;});
  }
  function init(){
   var remember=qs('#wt25RememberEmail'),email=qs('#loginForm input[name="email"]');
   if(remember&&email){
     try{var saved=localStorage.getItem('worktic-login-remember-email');if(saved){email.value=saved;remember.checked=true;}}catch(e){}
     // Store only the email, never the password/token. Original authentication remains untouched.
     qs('#loginForm').addEventListener('submit',function(){try{if(remember.checked)localStorage.setItem('worktic-login-remember-email',email.value.trim());else localStorage.removeItem('worktic-login-remember-email');}catch(e){}},false);
     remember.addEventListener('change',function(){if(!remember.checked){try{localStorage.removeItem('worktic-login-remember-email');}catch(e){}}});
   }
   var forgot=qs('#wt25HelpPassword'),info=qs('#wt25RecoveryInfo');
   if(forgot&&info)forgot.addEventListener('click',function(){info.hidden=false;info.textContent=lang()==='en'?'Password recovery is not yet configured. Contact your account administrator for assistance.':'La recuperación de contraseña aún no está configurada. Contacta al administrador de tu cuenta para recibir ayuda.';});
   applyLabels();window.addEventListener('worktic:langchange',applyLabels);
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();
