/* Worktic AI V27.35.91 — unified Customer Attraction navigation & landing shell.
   Presentation only: no APIs, no forms, no workflow state, no messages. */
(() => {
  'use strict';
  const q = (s, root=document) => root.querySelector(s);
  const all = (s, root=document) => Array.from(root.querySelectorAll(s));
  const landing = () => q('#landings');
  const activeLanding = () => !!landing()?.classList.contains('active');
  const tabConfig = [
    ['Campañas', 'campaigns'], ['Calendario', 'calendar'],
    ['Creativos', 'creatives'], ['Formularios', 'forms'],
    ['Landing Pages', 'landings'], ['Leads y atribución', 'leads'],
    ['Meta Lead Ads', 'metaleads'], ['Meta Ads', 'meta']
  ];
  function openGrowth(tab) {
    const bridge = window.WorkticAttractionV273589;
    if (bridge && typeof bridge.openGrowth === 'function') bridge.openGrowth(tab);
    else {
      q('#appSidebar button[data-view="marketing"]')?.click();
      if (tab) requestAnimationFrame(() => q(`#marketing [data-growth="${tab}"]`)?.click());
    }
  }
  function openLandings() {
    const bridge=window.WorkticAttractionV273589;
    if (bridge && typeof bridge.openLandings === 'function') bridge.openLandings();
    else q('#appSidebar button[data-view="landings"]')?.click();
  }
  function ensureTabs() {
    const area=landing();if (!area) return;
    area.classList.add('wtcu-landing-unified');
    let bar=q('#wtcaLandingBreadcrumb',area);
    if(!bar){
      bar=document.createElement('div');bar.id='wtcaLandingBreadcrumb';
      area.insertBefore(bar,area.firstElementChild);
    }
    if (bar.dataset.wtcuReady!=='273591') {
      bar.className='growth-tabs wtcu-landing-tabs';
      bar.setAttribute('role','navigation');
      bar.setAttribute('aria-label','Secciones de Captación de Clientes');
      bar.replaceChildren();
      tabConfig.forEach(([label,key])=>{
        const btn=document.createElement('button');btn.type='button';
        btn.textContent=label;
        if(key==='landings'){
          btn.classList.add('active');btn.setAttribute('aria-current','page');
        } else {
          btn.addEventListener('click',()=>openGrowth(key));
        }
        bar.append(btn);
      });
      bar.dataset.wtcuReady='273591';
    }
    // Keep section title/actions and original landing data nodes intact.
    const head=q(':scope > .section-head',area);
    // La barra de pestañas precede la acción original de nueva landing.
    if (head && bar.nextElementSibling!==head) head.before(bar);
  }
  function reconcileHeader(){
    if (!activeLanding()) return;
    const title=q('#title');const subtitle=q('#subtitle');
    const expectedTitle = (window.wtTitle&&window.wtTitle('marketing','title')) || 'Captación de Clientes';
    if(title && title.textContent.trim()!==expectedTitle){
      title.textContent=expectedTitle;
    }
    const expectedSubtitle=(window.wtTitle&&window.wtTitle('landings','subtitle')) || 'Landing Pages · Diseña, conecta y publica páginas comerciales';
    if(subtitle && subtitle.textContent.trim()!==expectedSubtitle){
      subtitle.textContent=expectedSubtitle;
    }
    // The landing route is an internal tab of customer attraction for navigation.
    const main=q('#appSidebar button[data-view="marketing"]');
    if(main){
      main.classList.add('active');main.setAttribute('aria-current','page');
    }
    const nested=q('#appSidebar button[data-view="landings"]');
    if(nested){nested.classList.remove('active');nested.hidden=true;nested.setAttribute('aria-hidden','true')}
  }
  function sync(){ensureTabs();reconcileHeader()}
  function install(){
    sync();
    watchMarketingName();
    const area=landing();
    if(area){
      new MutationObserver(()=>requestAnimationFrame(sync))
        .observe(area,{attributes:true,attributeFilter:['class']});
    }
    // The legacy SPA controls the actual view. Reconcile after its click handler.
    document.addEventListener('click', e=>{
      if(e.target.closest('#appSidebar button[data-view],#wtcaLandingTab,.wtcu-landing-tabs button,#wtcsGrowthLink')){
        requestAnimationFrame(sync);
      }
    },true);
    // Some older renderers rewrite the page header when loading a view.
    const title=q('#title');if(title){
      new MutationObserver(()=>{if(activeLanding()) requestAnimationFrame(reconcileHeader)})
        .observe(title,{childList:true,characterData:true,subtree:true});
    }
  }

  // V27.35.95: Keep visible customer acquisition naming stable even if
  // legacy views re-render older names. Does not move buttons or alter routes.
  function expectedMarketingName(){
    const translated = typeof window.wtT==='function' ? window.wtT('nav.marketing') : null;
    if(translated) return translated;
    const language = typeof window.wtLang==='function' ? window.wtLang() : document.documentElement.lang;
    return String(language||'es').toLowerCase().startsWith('en') ? 'Customer Acquisition' : 'Captación de Clientes';
  }
  function setName(node,name){
    if(node && node.textContent.trim()!==name) node.textContent=name;
  }
  function reconcileMarketingName(){
    const name=expectedMarketingName();
    const nav=q('#appSidebar button[data-view="marketing"]');
    if(nav) setName(q('span[data-i18n="nav.marketing"]',nav) || q('span',nav),name);
    const marketing=q('#marketing');
    if(marketing){
      setName(q(':scope > .section-head h2',marketing),name);
      // Other (legacy) marketing renderers may create an extra namesake heading.
      all('h1,h2,h3',marketing).forEach(el=>{
        const heading=el.textContent.trim();
        if(heading==='Atracción de Clientes' || heading==='Captación de Leads') setName(el,name);
      });
    }
    if(window.__wtCurrentView==='marketing' || marketing?.classList.contains('active') || activeLanding()){
      setName(q('#title'),name);
    }
  }
  function watchMarketingName(){
    let pending=false;
    const schedule=()=>{
      if(pending)return;
      pending=true;
      requestAnimationFrame(()=>{ pending=false;reconcileMarketingName(); });
    };
    const nav=q('#appSidebar button[data-view="marketing"]');
    const marketing=q('#marketing');
    const head=marketing && q(':scope > .section-head',marketing);
    const title=q('#title');
    if(nav)new MutationObserver(schedule).observe(nav,{childList:true,characterData:true,subtree:true});
    if(head)new MutationObserver(schedule).observe(head,{childList:true,characterData:true,subtree:true});
    if(marketing)new MutationObserver(schedule).observe(marketing,{childList:true});
    if(title)new MutationObserver(schedule).observe(title,{childList:true,characterData:true,subtree:true});
    window.addEventListener('worktic:langchange',schedule);
    document.addEventListener('click',event=>{
      if(event.target.closest('#appSidebar button[data-view], #wtcaLandingBreadcrumb button'))schedule();
    },true);
    schedule();
  }
  window.WorkticAttractionShellV273591={openLandings,openGrowth,sync,reconcileMarketingName,version:'273595'};
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',install,{once:true});
  else install();
})();
