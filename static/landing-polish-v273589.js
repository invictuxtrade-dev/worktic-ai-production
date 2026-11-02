/* Worktic AI V27.35.89 · Atracción de Clientes and premium landing editor polish.
   No database changes, no new forms, no auto-messages. Preserve routes / marketing APIs. */
(() => {
 'use strict';
 const $ = (selector,root=document) => root.querySelector(selector);
 const $$ = (selector,root=document) => [...root.querySelectorAll(selector)];
 const attraction = () => $('button[data-view="marketing"]');
 const landingRoute = () => $('button[data-view="landings"]');
 const marketing = () => $('#marketing');
 const landings = () => $('#landings');
 function markAttraction(){
   const growth=attraction();const landing=landingRoute();
   if(!growth||!landing)return;
   if(landings()?.classList.contains('active')){
     $$('#appSidebar button[data-view].active').forEach(b=>b.classList.remove('active'));
     growth.classList.add('active');
     growth.setAttribute('aria-current','page');
     landing.removeAttribute('aria-current');
   } else if(marketing()?.classList.contains('active')){
     growth.setAttribute('aria-current','page');
   } else growth.removeAttribute('aria-current');
 }
 function clickLanding(){
   landingRoute()?.click();
   queueMicrotask(markAttraction);
   requestAnimationFrame(markAttraction);
 }
 function clickGrowth(subview){
   attraction()?.click();
   // Existing Growth tabs have their own controller; defer to its click handler.
   if(subview)setTimeout(()=>{ $(`[data-growth="${subview}"]`)?.click();markAttraction();},0);
   else requestAnimationFrame(markAttraction);
 }
 function insertTabBar(){
   const growthArea=marketing(), landingArea=landings();
   if(!growthArea||!landingArea)return;
   const gTabs=$('.growth-tabs',growthArea);
   if(gTabs&&!$('#wtcaLandingTab',gTabs)){
     const b=document.createElement('button');b.type='button';b.id='wtcaLandingTab';b.textContent='Landing Pages';
     b.setAttribute('aria-label','Páginas de conversión en Atracción de Clientes');
     b.addEventListener('click',clickLanding);
     const forms=$('[data-growth="forms"]',gTabs);if(forms)forms.after(b);else gTabs.append(b);
   }
   if(!$('#wtcaLandingBreadcrumb',landingArea)){
     const bar=document.createElement('div');bar.className='wtca-attraction-tabs';bar.id='wtcaLandingBreadcrumb';
     const caption=document.createElement('span');caption.className='wtca-attraction-caption';caption.textContent='ATRACCIÓN DE CLIENTES /';bar.append(caption);
     for(const [title,action,active] of [
       ['Campañas',()=>clickGrowth('campaigns'),false],
       ['Formularios',()=>clickGrowth('forms'),false],
       ['Landing Pages',()=>{},true],
       ['Leads y atribución',()=>clickGrowth('leads'),false]
     ]){
       const b=document.createElement('button');b.type='button';b.textContent=title;
       if(active){b.classList.add('active');b.setAttribute('aria-current','page')}
       else b.addEventListener('click',action);
       bar.append(b);
     }
     landingArea.insertBefore(bar,landingArea.firstElementChild);
   }
   // Existing conversion-studio action should navigate the unified attraction area.
   const old=$('#wtcsGrowthLink');if(old){old.textContent='Ir a Landing Pages';old.onclick=clickLanding;}
   const head=$('#marketing .section-head h2');if(head){head.textContent='Atracción de Clientes';head.removeAttribute('data-i18n')}
   const desc=$('#marketing .section-head p');if(desc){desc.textContent='Campañas, formularios, landing pages y leads conectados en un mismo recorrido comercial.';desc.removeAttribute('data-i18n')}
 }
 // The previous dashboard reads an inaccessible lexical variable on some app builds and shows 0.
 // Render honest card counts; for metrics not exposed in HTML prefer a dash over fabricated zeros.
 function reconcileLandingCounters(){
   const cards=$$('#landingsGrid .landing-card');
   if(!cards.length)return;
   const total=$('#wtcsCountTotal');if(total)total.textContent=String(cards.length);
   // No guessing link status or tenant-level analytics from decorative cards.
   const live=$('#wtcsCountLive'),forms=$('#wtcsCountForms'),ready=$('#wtcsCountReady');
   const active=cards.filter(card=>{const text=card.textContent||'';return /\b(Activa|Publicada)\b/i.test(text)&&!/\bBorrador\b/i.test(text)});
   if(live)live.textContent=String(active.length);
   if(forms)forms.textContent='—';if(ready)ready.textContent='—';
   [forms,ready].forEach(node=>{if(node)node.title='Conexiones por verificar en los detalles de cada landing; no se muestran cifras estimadas.'});
 }
 function refineEditor(){
   const shell=$('.wtcs-upgraded');if(!shell||shell.dataset.wtcaRefined)return;
   shell.dataset.wtcaRefined='273589';
   const head=$('.wtl2-head-right',shell)||$('.wtl2-header',shell);
   const toggle=document.createElement('button');toggle.type='button';toggle.className='wtca-focus-btn';toggle.textContent='Ampliar edición';toggle.setAttribute('aria-pressed','false');toggle.title='Ocultar o mostrar la vista previa para tener más espacio';
   toggle.addEventListener('click',()=>{const focusing=shell.dataset.wtcaFocus!=='true';shell.dataset.wtcaFocus=focusing?'true':'false';toggle.textContent=focusing?'Mostrar vista previa':'Ampliar edición';toggle.setAttribute('aria-pressed',String(focusing));});
   head.prepend(toggle);
   const preview=$('.wtl2-preview-sticky',shell);
   if(preview&&!$('.wtca-preview-caption',preview)){
     const hint=document.createElement('p');hint.className='wtca-preview-caption';hint.textContent='Vista orientativa. En «Editar en sitio» verás y editarás la página completa.';
     const h=$('.wtl2-preview-head',preview);if(h)h.after(hint);else preview.prepend(hint);
   }
   // Keep the current step visible when switching sections; remove inherited scroll position.
   $$('[data-wtl2-step]',shell).forEach(b=>b.addEventListener('click',()=>{
     const editor=$('.wtl2-editor',shell);if(editor)editor.scrollTop=0;
   }));
   const save=$('#saveLanding',shell);if(save){save.setAttribute('aria-label','Guardar landing en Worktic AI');}
 }
 function install(){
   const menu=attraction();if(menu){const span=$('span',menu);if(span){span.textContent='Atracción de Clientes';span.removeAttribute('data-i18n')}}
   const hidden=landingRoute();if(hidden){hidden.hidden=true;hidden.setAttribute('aria-hidden','true');hidden.tabIndex=-1}
   insertTabBar();markAttraction();reconcileLandingCounters();
   // Event delegation: the old editor rebuilds its contents on every open.
   document.addEventListener('click',e=>{
      if(e.target.closest('button[data-view],#wtcaLandingTab'))requestAnimationFrame(()=>{markAttraction();insertTabBar()});
      if(e.target.closest('#newLanding,#landingsGrid button'))requestAnimationFrame(refineEditor);
   },true);
   const modal=$('#modalContent');if(modal){new MutationObserver(refineEditor).observe(modal,{childList:true,subtree:false})}
   const cards=$('#landingsGrid');if(cards){new MutationObserver(()=>{reconcileLandingCounters();}).observe(cards,{childList:true})}
   refineEditor();
 }
 window.WorkticAttractionV273589={openLandings:clickLanding,openGrowth:clickGrowth,version:'273589'};
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',install,{once:true});else install();
})();
