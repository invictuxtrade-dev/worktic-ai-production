/* Worktic AI V27.35.24: visual enhancement only. Existing login.js owns authentication. */
(function(){'use strict';
var swaps=[];
function translate(){var en=window.wtLang && window.wtLang()==='en';
  document.querySelectorAll('[data-wt24-en]').forEach(function(el){
    if(!el.hasAttribute('data-wt24-es'))el.setAttribute('data-wt24-es',el.innerHTML);
    el.innerHTML=en?el.getAttribute('data-wt24-en'):el.getAttribute('data-wt24-es');
  });
}
function init(){
  var open=document.getElementById('wt24TopRegister');
  var reg=document.getElementById('registerTab');
  if(open && reg)open.addEventListener('click',function(){reg.click();var card=document.querySelector('.wt24-card');if(card && window.matchMedia('(max-width:960px)').matches)card.scrollIntoView({behavior:'smooth',block:'start'});});
  document.querySelectorAll('.wt24-password-toggle').forEach(function(btn){btn.addEventListener('click',function(){
    var box=btn.closest('.wt24-field'),input=box && box.querySelector('input');if(!input)return;
    var visible=input.type==='password'; input.type=visible?'text':'password';btn.setAttribute('aria-pressed',String(visible));btn.setAttribute('aria-label',visible?'Ocultar contraseña':'Mostrar contraseña');
  });});
  translate();
  window.addEventListener('worktic:langchange',translate);
  var tabs=document.querySelectorAll('.wt24-tabs [role="tab"]');
  tabs.forEach(function(btn){btn.addEventListener('click',function(){tabs.forEach(function(t){t.setAttribute('aria-selected',String(t===btn));});});});
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();
