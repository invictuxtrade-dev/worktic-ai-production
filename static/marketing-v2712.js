(function(){
  'use strict';
  var els=document.querySelectorAll('.v2711-reveal');
  if('IntersectionObserver' in window){
    var io=new IntersectionObserver(function(entries){entries.forEach(function(e){if(e.isIntersecting){e.target.classList.add('is-visible');io.unobserve(e.target)}})},{threshold:.12});
    els.forEach(function(el){io.observe(el)});
  }else{els.forEach(function(el){el.classList.add('is-visible')})}
  var launcher=document.getElementById('wtWaLauncher'),panel=document.getElementById('wtWaPanel');
  if(launcher&&panel){
    launcher.addEventListener('click',function(e){e.stopPropagation();panel.hidden=!panel.hidden;launcher.setAttribute('aria-expanded',String(!panel.hidden))});
    document.addEventListener('click',function(e){if(panel.hidden||panel.contains(e.target)||launcher.contains(e.target))return;panel.hidden=true;launcher.setAttribute('aria-expanded','false')});
    document.addEventListener('keydown',function(e){if(e.key==='Escape'){panel.hidden=true;launcher.setAttribute('aria-expanded','false')}});
  }

  var mt=document.getElementById('mobileNavToggle'),mp=document.getElementById('mobileNavPanel'),mo=document.getElementById('mobileNavOverlay'),mx=document.getElementById('mobileNavClose');
  if(mt&&mp&&mo&&!mt.dataset.v2712Bound){
    mt.dataset.v2712Bound='1';
    var setMobile=function(open){mp.classList.toggle('open',open);mo.classList.toggle('show',open);document.body.classList.toggle('mobile-nav-lock',open);mt.setAttribute('aria-expanded',String(open));mp.setAttribute('aria-hidden',String(!open));};
    mt.addEventListener('click',function(e){e.preventDefault();e.stopPropagation();setMobile(true)});
    if(mx)mx.addEventListener('click',function(){setMobile(false)});
    mo.addEventListener('click',function(){setMobile(false)});
    mp.querySelectorAll('a').forEach(function(a){a.addEventListener('click',function(){setMobile(false)})});
  }
})();
