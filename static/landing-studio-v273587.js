/* Worktic AI Landing Conversion Studio V27.35.87
   Progressive enhancement of existing secure Landing Premium V2 editor.
   Uses the existing landing API, guard-based save, tenant-scoped form catalog,
   agent/workflow connections; never creates autonomous external messages. */
(() => {
  'use strict';
  const VERSION = '273587';
  const esc = v => String(v ?? '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
  const q = (s, root=document) => root.querySelector(s);
  const qa = (s, root=document) => Array.from(root.querySelectorAll(s));
  const get = async path => {
    const r = await fetch(path, {credentials:'same-origin',headers:{Accept:'application/json'}});
    if (!r.ok) throw new Error(`No fue posible consultar ${path} (${r.status})`);
    return r.json();
  };
  const safeArray = value => Array.isArray(value) ? value : [];
  const jsObject = value => {if (!value) return {}; if (typeof value==='object') return value; try {return JSON.parse(value)} catch {return {}}};
  const linkedWorkflows = (flows, formId) => safeArray(flows).filter(w => {
    const cfg = jsObject(w.trigger_config); return w.trigger_type==='form.submitted' && Number(cfg.form_id)===Number(formId) && Number(formId)>0;
  });
  const formatAgent = (w, agents) => {
    const graph = jsObject(w.graph), nodes = safeArray(graph.nodes);
    const agentNode = nodes.find(n => n.type==='ai_task' && n.config?.agent_id);
    const agent = safeArray(agents).find(a => String(a.id)===String(agentNode?.config?.agent_id));
    return agent?.name || (agentNode ? 'Agente no disponible' : 'Sin bloque IA');
  };
  const setValue = (id, value) => {
    const el = document.getElementById(id); if (!el) return;
    if (el.value !== String(value)) {el.value = String(value);el.dispatchEvent(new Event('input',{bubbles:true}));el.dispatchEvent(new Event('change',{bubbles:true}));}
  };
  const textOf = (id) => document.getElementById(id)?.value || '';
  const publishTab = () => {q('[data-wtl2-step="publish"]')?.click();};
  const showTab = step => {
    qa('[data-wtl2-step]').forEach(el => el.classList.toggle('active',el.dataset.wtl2Step===step));
    qa('.wtl2-step').forEach(el=>{el.hidden=el.id!=='wtl2-'+step});
    const shell=q('.wtl2-shell'); if(shell)shell.dataset.studioStep=step;
  };
  const formURL = f => `/f/${Number(f.tenant_id)}/${encodeURIComponent(f.slug)}`;
  const publicURL = item => `/l/${Number(item.tenant_id)}/${encodeURIComponent(item.slug)}`;
  const landingClean = forms => {
    const f=safeArray(forms).find(x=>Number(x.id)===Number(q('#lpForm')?.value||0));
    return !f || f.active===false ? null : f;
  };
  const createTab=(key,name,subtitle,html)=>{
    const nav=q('.wtl2-nav'), main=q('.wtl2-editor');if(!nav||!main)return;
    const b=document.createElement('button');b.type='button';b.dataset.wtl2Step=key;b.innerHTML=`<span class="num">${key==='conversion'?'5':'6'}</span><div><b>${esc(name)}</b><small>${esc(subtitle)}</small></div>`;
    nav.insertBefore(b,q('.wtl2-help',nav));
    const section=document.createElement('section');section.id='wtl2-'+key;section.className='wtl2-step wtcs-section';section.hidden=true;section.innerHTML=html;main.append(section);
    b.addEventListener('click',()=>showTab(key));
  };
  const visualInputs = [
    ['.hero-copy .eyebrow','lpBadge'],['.hero-copy h1','lpHeadline'],['.hero-copy .hero-lead','lpSubheadline'],
    ['.hero-actions a.btn:not(.secondary)','lpCTA'],['.brand span:last-child','lpBrandName'],
    ['.trust-line span:last-child','lpTrustText'],['.contact-copy h2','lpContactTitle'],
    ['.contact-copy p','lpContactSubtitle'],['.footer-inner > span','lpFooterText']
  ];
  const groups=[
    {selectors:['.benefits .benefit span:last-child'], id:'lpBenefits', kind:'lines'},
    {selectors:['.feature-grid .card h3','.feature-grid .card p'], id:'lpFeatures', kind:'pairs'},
    {selectors:['.testimonial-grid .card .quote','.testimonial-grid .card .author span:last-child'],id:'lpTestimonials',kind:'pairs'},
    {selectors:['.faq-list details summary','.faq-list details p'],id:'lpFAQ',kind:'pairs'}
  ];
  function makeEditableFrame(iframe, status) {
    const doc=iframe.contentDocument; if(!doc?.body)return;
    const css=doc.createElement('style');css.textContent=`
      [data-wtcs-field]{cursor:text!important;outline:2px dashed transparent;outline-offset:4px;border-radius:6px;transition:outline-color .18s}
      [data-wtcs-field]:hover{outline-color:#7c3aed!important}
      [data-wtcs-field]:focus{outline:2px solid #6c57ff!important;background:rgba(236,231,255,.36)!important}
      body::after{content:'MODO EDICIÓN · CAMBIOS SIN PUBLICAR';position:fixed;bottom:12px;left:12px;z-index:9000;background:#0e1735;color:white;border-radius:100px;padding:8px 14px;font:700 11px system-ui;pointer-events:none}
      html{scroll-behavior:auto!important}`;
    doc.head.append(css);
    doc.addEventListener('click',e=>{const target=e.target.closest('a,button,form');if(target&&!(target.hasAttribute('data-wtcs-field')))e.preventDefault()},true);
    doc.addEventListener('submit',e=>e.preventDefault(),true);
    for (const [selector,id] of visualInputs) {
      const el=doc.querySelector(selector);if(!el||!q('#'+id))continue;
      el.setAttribute('contenteditable','true');el.setAttribute('spellcheck','true');el.dataset.wtcsField=id;
      el.textContent=textOf(id);
      el.addEventListener('input',()=>setValue(id,el.textContent.trim()));
    }
    for(const group of groups){
      if(!q('#'+group.id))continue;
      if(group.kind==='lines'){
        const els=qa(group.selectors[0],doc); const lines=textOf(group.id).split('\n');
        els.forEach((el,i)=>{el.contentEditable='true';el.dataset.wtcsField=group.id;el.textContent=lines[i]||el.textContent;el.addEventListener('input',()=>setValue(group.id,els.map(e=>e.textContent.trim()).filter(Boolean).join('\n')))});
      } else {
        const first=qa(group.selectors[0],doc),second=qa(group.selectors[1],doc);
        const values=textOf(group.id).split('\n').map(s=>s.split('|').map(x=>x.trim()));
        const sync=()=>setValue(group.id,first.map((el,i)=>[el.textContent.replace(/[“”]/g,'').trim(),second[i]?.textContent.trim()||''].join(' | ')).filter(s=>s.replace(/[|\s]/g,'')).join('\n'));
        first.forEach((el,i)=>{el.contentEditable='true';el.dataset.wtcsField=group.id;if(values[i]?.[0])el.textContent=values[i][0];el.addEventListener('input',sync)});
        second.forEach((el,i)=>{el.contentEditable='true';el.dataset.wtcsField=group.id;if(values[i]?.[1])el.textContent=values[i][1];el.addEventListener('input',sync)});
      }
    }
    const controls=q('#wtcsEditControls');if(controls)controls.hidden=false;
    if(status)status.textContent='Edición sobre la página · haz clic en cualquier texto resaltado';
  }
  function draftHTML(){
    const benefit=(textOf('lpBenefits')||'').split('\n').filter(Boolean).map(x=>`<span class="wtcs-dbenefit">✓ ${esc(x)}</span>`).join('');
    const accent=q('#lpAccent')?.value||'#6858ff';const photo=textOf('lpHero');
    const photoEl=photo?`<img src="${esc(photo)}" alt="Portada" style="max-width:100%;width:100%;border-radius:24px;box-shadow:0 18px 46px #070d3030">`:'<div class="place">Arrastra una imagen desde la pestaña Multimedia</div>';
    return `<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>body{margin:0;font-family:system-ui;background:#f7f8ff;color:#121633}nav{padding:26px 7%;background:white;display:flex;justify-content:space-between;align-items:center}nav b{font-size:21px}.cta{background:${esc(accent)};color:white;padding:13px 23px;border-radius:14px;font-weight:800}.hero{padding:65px 7%;display:grid;grid-template-columns:1.1fr 1fr;gap:45px;align-items:center;background:radial-gradient(circle at 80% 0,#e9d7ff 0,transparent 45%),#f8f7ff}h1{font-size:clamp(36px,5.5vw,68px);letter-spacing:-.055em;line-height:1.05;margin:10px 0 22px}.tag{color:${esc(accent)};font-weight:800;letter-spacing:.14em;text-transform:uppercase}.lead{font-size:18px;color:#646b82;line-height:1.6}.place{height:280px;background:linear-gradient(135deg,#e8ecff,#f9eaff);border-radius:25px;display:grid;place-items:center;text-align:center;padding:15px;color:#69738a}section{padding:30px 7%;display:flex;gap:12px;flex-wrap:wrap}.wtcs-dbenefit{padding:17px 22px;background:white;border-radius:14px;box-shadow:0 8px 28px #080f2b10}footer{padding:30px 7%;background:white}@media(max-width:700px){.hero{grid-template-columns:1fr;padding:48px 6%}nav{padding:18px 6%}}[contenteditable]:hover{outline:2px dashed ${esc(accent)};outline-offset:4px}[contenteditable]:focus{outline:2px solid ${esc(accent)};outline-offset:4px}</style></head><body><nav><b class="brand"><span></span><span>${esc(textOf('lpBrandName')||textOf('lpName')||'Tu marca')}</span></b><span class="cta">${esc(textOf('lpCTA'))}</span></nav><div class="hero"><div class="hero-copy"><div class="eyebrow tag">${esc(textOf('lpBadge'))}</div><h1>${esc(textOf('lpHeadline')||'Tu propuesta empieza aquí')}</h1><p class="hero-lead lead">${esc(textOf('lpSubheadline'))}</p><div class="hero-actions"><a class="cta btn">${esc(textOf('lpCTA'))}</a></div><div class="trust-line"><span></span><span>${esc(textOf('lpTrustText'))}</span></div></div>${photoEl}</div><section class="benefits">${benefit.split('</span>').join('</span>')}</section><footer class="footer-inner"><strong>Worktic AI</strong> <span>${esc(textOf('lpFooterText'))}</span></footer></body></html>`;
  }
  function showEditorFrame(item) {
    const frame=q('#wtcsFrame'),status=q('#wtcsFrameStatus');if(!frame)return;
    status.textContent='Preparando el editor visual…';frame.onload=()=>{
      try{makeEditableFrame(frame,status)}catch(e){status.textContent='No se pudo habilitar edición. Usa las pestañas de contenido.'}
    };
    if(item.id && item.published && item.slug && item.tenant_id){frame.removeAttribute('srcdoc');frame.src=publicURL(item)+'?edit_preview='+VERSION;}
    else{frame.removeAttribute('src');frame.srcdoc=draftHTML()}
  }
  async function conversionPanel(item) {
    const status=q('#wtcsFormStatus'), select=q('#wtcsForm'), channels=q('#wtcsChannelStatus');if(!status||!select)return;
    status.textContent='Consultando formularios y vínculos de conversión…';
    const results=await Promise.allSettled([get('/api/marketing/forms'),get('/api/automations/workflows'),get('/api/agents'),get('/api/marketing/landings/channels')]);
    if(results[0].status!=='fulfilled') {status.textContent='No se pudieron consultar los formularios. Reintenta sin publicar.';return}
    const forms=safeArray(results[0].value), flows=results[1].status==='fulfilled'?safeArray(results[1].value):[], agents=results[2].status==='fulfilled'?safeArray(results[2].value?.agents):[];
    const list=results[3].status==='fulfilled'?safeArray(results[3].value?.channels):[];
    select.replaceChildren(new Option('Sin formulario — solo página informativa','0'));
    forms.forEach(f=>select.add(new Option(`${f.name||f.slug}${f.active===false?' (inactivo)':''}`,String(f.id))));
    select.value=String(q('#lpForm')?.value||0);
    if(select.selectedIndex<0)select.value='0';
    const update=()=>{
      const fid=Number(select.value||0),f=forms.find(v=>Number(v.id)===fid);
      setValue('lpForm',fid);
      const warn=q('#wtcsFormWarning');
      if(warn)warn.textContent=!fid?'Conecta un formulario para captar leads y enviarlos al CRM.':f?.active===false?'El formulario está inactivo. Actívalo antes de publicar.':'El formulario se incrustará en la landing publicada; sus envíos crearán contactos y oportunidades según las reglas del CRM.';
      if(q('#wtcsFormLink')){q('#wtcsFormLink').hidden=!f; if(f)q('#wtcsFormLink').href=formURL(f)}
      const linked=linkedWorkflows(flows,fid);
      const linkedList=q('#wtcsAutomationList');
      if(linkedList)linkedList.innerHTML=!fid?'<div class="wtcs-mini-note">Primero vincula un formulario.</div>': linked.length ? linked.map(w=>`<div class="wtcs-linked"><span class="wtcs-dot ${w.status==='active'?'on':''}"></span><div><b>${esc(w.name)}</b><small>${esc(w.status==='active'?'Activo':'No activo')} · Agente: ${esc(formatAgent(w,agents))}</small></div></div>`).join(''):'<div class="wtcs-mini-note">No hay automatización específica para este formulario. Puedes crearla desde Automatizaciones.</div>';
    };
    select.addEventListener('change',update);update();
    status.textContent=`${forms.length} formulario(s) disponible(s) en esta empresa`;
    if(channels){channels.textContent=list.length?`${list.length} canal(es) con enlace público. Selecciónalos en la pestaña Publicar para incluirlos en la landing.`:'No hay canales con enlace público conectados. El formulario funciona sin WhatsApp ni Chat Web.'}
    const navAgents=q('#wtcsAgentNote');if(navAgents)navAgents.textContent='El agente clasificador se vincula mediante el workflow activo. Para atención conversacional, primero configura y asigna un canal Chat Web en Canales; esta landing no activa agentes por sí sola.';
  }
  function enhance(item={}) {
    const shell=q('.wtl2-shell');if(!shell||shell.dataset.wtcsInstalled)return; shell.dataset.wtcsInstalled=VERSION;
    shell.classList.add('wtcs-upgraded');
    const title=q('.wtl2-header h2');if(title)title.textContent=`${item.id?'Editar':'Crear'} página de conversión`;
    const subtitle=q('.wtl2-header p');if(subtitle)subtitle.textContent='Diseña, conecta y publica una experiencia completa para captar oportunidades de negocio.';
    createTab('conversion','Conversión','Formularios, agentes y seguimiento',`
      <div class="wtcs-top"><span class="wtcs-kicker">05 · MOTOR COMERCIAL</span><h3>Conecta cada visita con una oportunidad</h3><p>Utiliza un formulario existente, verifica el flujo automático y ofrece canales reales de atención sin duplicar configuraciones.</p></div>
      <div class="wtcs-panel"><div class="wtcs-panel-head"><span class="wtcs-step-number">01</span><div><h4>Formulario de captación</h4><p>Reutiliza el mismo formulario en tantas páginas como necesites.</p></div></div>
        <label class="wtcs-field">Formulario asociado<select id="wtcsForm"><option>Consultando catálogo...</option></select></label>
        <div id="wtcsFormWarning" class="wtcs-mini-note"></div><div id="wtcsFormStatus" class="wtcs-status"></div>
        <div class="wtcs-inline-actions"><a target="_blank" rel="noopener" id="wtcsFormLink" hidden class="wtcs-text-button">Abrir formulario independiente</a><button type="button" id="wtcsCreateForm">Administrar formularios</button></div></div>
      <div class="wtcs-panel"><div class="wtcs-panel-head"><span class="wtcs-step-number">02</span><div><h4>Agente IA y automatización</h4><p>Conoce quién clasificará el lead cuando envíe el formulario.</p></div></div><div id="wtcsAutomationList" class="wtcs-linked-list"></div><p id="wtcsAgentNote" class="wtcs-mini-note"></p><button type="button" id="wtcsGoAutomation">Abrir Automatizaciones</button></div>
      <div class="wtcs-panel"><div class="wtcs-panel-head"><span class="wtcs-step-number">03</span><div><h4>Canales y conversación</h4><p>Conecta únicamente canales reales; no se activan envíos automáticos desde aquí.</p></div></div><div id="wtcsChannelStatus" class="wtcs-mini-note"></div><button type="button" id="wtcsGoChannels">Seleccionar canales para la página</button></div>
      <div class="wtcs-panel wtcs-progress"><div><span>1 · Atraer</span><b>Landing</b></div><span>→</span><div><span>2 · Captar</span><b>Formulario</b></div><span>→</span><div><span>3 · Calificar</span><b>IA</b></div><span>→</span><div><span>4 · Convertir</span><b>CRM</b></div></div>`);
    createTab('inplace','Editar en sitio','Vista real · PC y móvil',`
      <div class="wtcs-top"><span class="wtcs-kicker">06 · EDICIÓN VISUAL</span><h3>Diseña directamente sobre tu página</h3><p>Haz clic en títulos, botones, beneficios y textos para modificarlos sin tocar código. Los cambios solo se publican al guardar.</p></div>
      <div class="wtcs-frame-toolbar"><div class="wtcs-dots"><i></i><i></i><i></i></div><span id="wtcsFrameStatus">Vista lista</span><div class="wtcs-device-actions"><button type="button" class="active" data-wtcs-size="desktop">Escritorio</button><button type="button" data-wtcs-size="mobile">Móvil</button></div></div>
      <div class="wtcs-frame-shell"><iframe id="wtcsFrame" title="Editor en sitio — vista previa sin envíos" sandbox="allow-same-origin" referrerpolicy="same-origin"></iframe></div>
      <div id="wtcsEditControls" class="wtcs-inline-actions" hidden><span>Edición segura: enlaces y envíos desactivados en esta vista.</span><button type="button" id="wtcsOpenClassic">Editar secciones y multimedia</button></div>`);
    qa('[data-wtl2-step]').forEach(b=>b.addEventListener('click',()=>{shell.dataset.studioStep=b.dataset.wtl2Step;if(b.dataset.wtl2Step==='inplace')showEditorFrame(item)}));
    q('#wtcsGoChannels')?.addEventListener('click',publishTab);
    q('#wtcsOpenClassic')?.addEventListener('click',()=>q('[data-wtl2-step="sections"]')?.click());
    q('#wtcsGoAutomation')?.addEventListener('click',()=>{
      q('#modal')?.close();q('button[data-view="rules"]')?.click();
    });
    q('#wtcsCreateForm')?.addEventListener('click',()=>{
      q('#modal')?.close();q('button[data-view="marketing"]')?.click();q('[data-growth="forms"]')?.click();
    });
    qa('[data-wtcs-size]').forEach(b=>b.addEventListener('click',()=>{
      qa('[data-wtcs-size]').forEach(x=>x.classList.toggle('active',x===b));
      const frame=q('#wtcsFrame');if(frame){const mobile=b.dataset.wtcsSize==='mobile';frame.style.setProperty('width',mobile?'390px':'100%','important');frame.style.setProperty('max-width',mobile?'390px':'100%','important');frame.style.setProperty('flex',mobile?'0 0 390px':'1 1 100%','important')}
    }));
    conversionPanel(item).catch(e=>{if(q('#wtcsFormStatus'))q('#wtcsFormStatus').textContent='No se pudieron consultar las conexiones: '+e.message});
  }
  function dashboard(){
    const area=q('#landings'); if(!area)return;
    if(!q('#wtcsHub')){
      const hub=document.createElement('div');hub.id='wtcsHub';hub.innerHTML=`<div class="wtcs-hub-hero"><div class="wtcs-hub-copy"><span class="wtcs-kicker">WORKTIC · CONVERSION STUDIO</span><h2>Crea páginas que convierten visitas en clientes</h2><p>Diseña experiencias premium, conecta tus formularios, incorpora canales y automatiza el seguimiento desde el CRM.</p><div class="wtcs-hero-pills"><span>Diseño sin código</span><span>Formularios reutilizables</span><span>Automatizaciones IA</span></div></div><div class="wtcs-hub-glass"><span>Tu recorrido comercial</span><div><b>01</b><span>Página premium</span></div><div><b>02</b><span>Formulario conectado</span></div><div><b>03</b><span>Alicia / CRM</span></div><div><b>04</b><span>Seguimiento y venta</span></div></div></div><div class="wtcs-hub-stats"><div><b id="wtcsCountTotal">—</b><span>Landing pages</span></div><div><b id="wtcsCountLive">—</b><span>Publicadas</span></div><div><b id="wtcsCountForms">—</b><span>Con formulario</span></div><div><b id="wtcsCountReady">—</b><span>Listas para captar</span></div></div><div class="wtcs-hub-subtitle"><h3>Mis páginas comerciales</h3><p>Personaliza, previsualiza y publica sin perder la conexión con tus prospectos.</p></div>`;
      const head=q('.section-head',area); if(head)head.after(hub); else area.prepend(hub);
    }
    const title=q('#landings .section-head h2');if(title){title.textContent='Landing Pages · Conversion Studio';title.removeAttribute('data-i18n')}
    const desc=q('#landings .section-head p');if(desc){desc.textContent='Páginas de alto impacto conectadas a formularios, campañas, agentes y oportunidades.';desc.removeAttribute('data-i18n')}
    const grid=q('#landingsGrid');if(!grid)return;
    const items=typeof allLandings!=='undefined'?safeArray(allLandings):[];
    const total=q('#wtcsCountTotal'), live=q('#wtcsCountLive'), forms=q('#wtcsCountForms'), ready=q('#wtcsCountReady');
    if(total)total.textContent=items.length;if(live)live.textContent=items.filter(x=>x.published).length;
    if(forms)forms.textContent=items.filter(x=>Number(x.form_id)>0).length;
    if(ready)ready.textContent=items.filter(x=>x.published&&Number(x.form_id)>0).length;
  }
  function relabelGrowth(){
    const tab=q('[data-growth="forms"]');if(tab){tab.textContent='Formularios';tab.removeAttribute('data-i18n')}
    const title=q('#growth-forms h2');if(title){title.textContent='Formularios de captación';title.removeAttribute('data-i18n')}
    const description=q('#growth-forms .section-head p');if(description){description.textContent='Crea formularios reutilizables para captar prospectos. Puedes integrarlos en cualquier landing page y activar su seguimiento automático.';description.removeAttribute('data-i18n')}
    const button=q('#newLeadForm');if(button){button.textContent='Nuevo formulario';button.removeAttribute('data-i18n')}
    if(!q('#wtcsGrowthLink')&&q('#growth-forms .section-head')){const a=document.createElement('button');a.type='button';a.className='secondary';a.id='wtcsGrowthLink';a.textContent='Abrir Landing Pages';a.onclick=()=>q('button[data-view="landings"]')?.click();q('#growth-forms .section-head').append(a)}
    const modalTitle=q('#modalContent h2');if(modalTitle&&modalTitle.textContent.trim()==='Nuevo formulario / landing')modalTitle.textContent='Nuevo formulario de captación';
  }
  function install(){
    if(typeof openLandingEditor==='function'&&!window.__wtcsWrapped){
      const original=openLandingEditor;
      openLandingEditor=function(item={}){original(item);enhance(item)};
      window.__wtcsWrapped=true;
    }
    dashboard();relabelGrowth();
    const observer=new MutationObserver(()=>{dashboard();relabelGrowth()});
    // Observe only container children: avoid recursive loops from our own text updates.
    const landingGrid=q('#landingsGrid');if(landingGrid)observer.observe(landingGrid,{childList:true});
    const formsGrid=q('#leadFormsGrid');if(formsGrid)observer.observe(formsGrid,{childList:true});
    document.addEventListener('click',e=>{if(e.target?.closest('#newLeadForm'))setTimeout(relabelGrowth,0)},true);
  }
  window.WTLandingStudioV273587={linkedWorkflows,formatAgent,version:VERSION,enhance};
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',install,{once:true});else install();
})();
