/* Worktic AI V27.36.0: language sync, form validation remains native and server-side. */
(function(){'use strict';
function translate(){var en=false;try{en=(typeof window.wtLang==='function'&&window.wtLang()==='en')||document.documentElement.lang==='en';}catch(e){}
 document.body.classList.toggle('wt-legal-en',!!en);
}
function init(){translate();window.addEventListener('worktic:langchange',translate);var form=document.getElementById('registerForm');if(!form)return;
 form.addEventListener('submit',function(e){var t=form.querySelector('input[name="legal_accepted"]'),p=form.querySelector('input[name="privacy_authorized"]');if(!(t&&p&&t.checked&&p.checked)){e.preventDefault();e.stopImmediatePropagation();var error=document.getElementById('authError');if(error)error.textContent=document.body.classList.contains('wt-legal-en')?'Review and accept the terms and privacy authorization to create an account.':'Para crear la cuenta debes aceptar los Términos y autorizar el tratamiento informado de tus datos.';}},true);
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();
