/* WorkticAI V27.34 - one public catalogue for landing and in-app checkout.
 * No duplicate prices, no guessed entitlements. Backend remains authoritative. */
(()=>{'use strict';
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const number=v=>Number(v||0).toLocaleString('es-CO');
const es=()=>document.documentElement.lang!=='en';
const words=(a,b)=>es()?a:b;
const size=mb=>mb>=1024?`${number(mb/1024)} GB`:`${number(mb)} MB`;
const cash=n=>Number(n||0).toLocaleString(es()?'es-CO':'en-US',{minimumFractionDigits:0,maximumFractionDigits:2});
const statusLine=(offer)=>offer.creative_commercial_active?`<span class="wts35-status-icon wts35-status-ready">✓</span><div><strong>${words('Créditos creativos disponibles','Creative credits available')}</strong><small>${words('Consulta los paquetes y el consumo de tu plan antes de generar.','Review packages and your plan limits before generating.')}</small></div>`:`<span class="wts35-status-icon">✦</span><div><strong>${words('Próximamente, más poder creativo','More creative possibilities are coming')}</strong><small>${words('Las recargas estarán disponibles después de su validación. El editor de tu plan sigue disponible.','Top-ups will become available after validation. Your plan editor remains available.')}</small></div>`;
const benefits=(p,offer,short=false)=>{
 const li=(label)=>`<li>${label}</li>`;
 const result=[
  li(`<b>${number(p.max_users)}</b> ${words('usuarios','users')} · <b>${number(p.max_channels)}</b> ${words('conexiones','connections')}`),
  li(`<b>${number(p.max_contacts)}</b> ${words('contactos CRM','CRM contacts')} · <b>${number(p.max_products)}</b> ${words('productos','products')}`),
  li(`<b>${number(p.max_ai_responses)}</b> ${words('respuestas IA/mes','AI replies/month')}`),
  li(`<b>${number(p.max_rules)}</b> ${words('reglas de automatización','automation rules')}`),
  li(`<b>${number(p.max_agents)}</b> ${words('agentes IA','AI agents')} · <b>${number(p.max_whatsapp)}</b> WhatsApp · <b>${number(p.max_telegram)}</b> Telegram · <b>${number(p.max_messenger)}</b> Messenger`),
  li(`Worktic Studio: ${p.studio_editor?words('editor de imágenes y videos','image and video editor'):words('no incluido','not included')}`),
  li(`${number(p.max_studio_projects)} ${words('proyectos Studio','Studio projects')} · ${esc(size(p.studio_storage_mb))} ${words('de medios','media storage')}`),
  li(offer.creative_commercial_active?`<b>${number(p.monthly_credits)} ${words('créditos creativos/mes','creative credits/month')}</b>`:words('Generación IA con créditos: pendiente de activación','Credit-based AI generation: pending activation')),
 ];
 if(!short){
   const mods=[['analytics','Analytics'],['ads','Ads Center'],['social','Social Hub'],['whatsapp_marketing','WhatsApp Marketing'],['meta_leads','Meta Lead Ads'],['automations',words('Workflows avanzados','Advanced workflows')]];
   for(const [code,label] of mods)result.push(li(`<span class="${p.modules?.[code]?'wts34-yes':'wts34-no'}">${p.modules?.[code]?'✓':'—'} ${esc(label)}</span>`));
   result.push(li(words('Servicios de Meta, Ads y proveedores externos sujetos a permisos y cargos propios','Meta, ads and third-party services may require permissions and separate charges')));
 }
 return result.join('');
};
window.WorkticOfferBenefitsV2734=(offer,code)=>{
 const p=offer?.plans?.find(x=>x.code===code);
 return p?benefits(p,offer,true):'';
};
window.WorkticOfferCheckoutV2734=(p,offer)=>{
 return `<div class="wts34-checkout-title">${words('Esto es lo que compras','What your plan includes')} · ${esc(p.name)}</div><ul class="wts34-checkout-list">${benefits(p,offer,true)}</ul><small class="wts34-checkout-disclosure">${words('Créditos mensuales no acumulables. El pago se aprueba tras la verificación. Los límites aplican por empresa y las funciones dependen de activación y permisos externos. El checkout calcula el importe definitivo.','Monthly credits do not roll over. Payment requires verification. Limits are per tenant; features may require activation and third-party permissions. Checkout confirms the final price.')}</small>${offer.creative_commercial_active?'':`<small class="wts35-checkout-note">${words('Los créditos creativos y recargas se habilitarán tras las pruebas; no están incluidos todavía en esta compra.','Creative credits and top-ups will launch after testing; they are not yet included in this purchase.')}</small>`}`;
};
const render=offer=>{
 const root=document.getElementById('plansGrid');if(!root||!document.getElementById('planes'))return;
 const year=(()=>{const annualBtn=document.getElementById('billingAnnual');if(annualBtn)return annualBtn.classList.contains('active');try{return localStorage.getItem('worktic_public_billing_cycle')==='annual'}catch{return false}})();
 const cards=offer.plans.map(p=>{
  const paid=p.code!=='free';const featured=p.code==='business';const price=year?p.annual_usdt:p.monthly_usdt;
  const href=paid?`/login.html?tab=register&plan=${encodeURIComponent(p.code)}&billing=${year?'annual':'monthly'}`:'/login.html?tab=register';
  return `<article class="card plan wts34-plan ${featured?'pop':''}" data-plan="${esc(p.code)}">${featured?`<span class="flag">${words('Más popular','Most popular')}</span>`:''}<h3>${esc(p.name)}</h3><p class="wts34-plan-desc">${esc(p.description)}</p><div class="price wts34-price">${paid?cash(price)+' USDT':'0 USDT'}<small>${paid?(year?words('/año','/year'):words('/mes','/month')):''}</small></div>${paid?`<p class="wts34-equivalent">${year?words(`Equivale a ${cash(p.annual_usdt/12)} USDT/mes · pago anual`,`Equivalent to ${cash(p.annual_usdt/12)} USDT/month · billed annually`):words('Facturado mensualmente','Billed monthly')}</p>`:''}<div class="wts34-entitle-title">${words('Incluye y limita','Included capacities')}</div><ul class="wts34-benefit-list">${benefits(p,offer,false)}</ul><a class="btn ${featured?'primary':''} plan-cta" href="${href}">${paid?words('Elegir '+esc(p.name),'Choose '+esc(p.name)):words('Crear cuenta Free','Start free')}</a>${!offer.creative_commercial_active?'<p class="wts34-mini">'+words('Las generaciones IA aún no se incluyen en la oferta activa.','AI generations are not included in the active offer yet.')+'</p>':''}</article>`;
 }).join('');
 root.innerHTML=cards;
 const cmp=document.getElementById('wts34Compare');if(cmp){
  const fields=[
   [words('Usuarios','Users'),'max_users'],[words('Conexiones totales','Connections'),'max_channels'],[words('Agentes IA','AI agents'),'max_agents'],['WhatsApp','max_whatsapp'],['Telegram','max_telegram'],['Messenger','max_messenger'],[words('Contactos CRM','CRM contacts'),'max_contacts'],[words('Productos / servicios','Products / services'),'max_products'],[words('Reglas','Automation rules'),'max_rules'],[words('Respuestas IA / mes','AI replies / month'),'max_ai_responses'],[words('Créditos creativos / mes','Creative credits / month'),'monthly_credits'],[words('Proyectos Studio','Studio projects'),'max_studio_projects'],[words('Almacenamiento Studio','Studio storage'),'studio_storage_mb'],['Studio Image IA','studio_image_ai'],['Studio Video IA','studio_video_ai'],['Ads Center','modules.ads'],['Analytics','modules.analytics'],['WhatsApp Marketing','modules.whatsapp_marketing'],['Social Hub','modules.social']
  ];
  const val=(p,key)=>{const v=key.split('.').reduce((o,k)=>o?.[k],p);if(typeof v==='boolean')return v?'✓':'—';if(key==='studio_storage_mb')return esc(size(v));return number(v)};
  cmp.innerHTML=`<table class="comparison-table"><thead><tr><th>${words('Capacidad','Capability')}</th>${offer.plans.map(p=>`<th>${esc(p.name)}</th>`).join('')}</tr></thead><tbody>${fields.map(([label,key])=>`<tr><td>${esc(label)}</td>${offer.plans.map(p=>`<td>${val(p,key)}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
 }
 const note=document.getElementById('wts34ActivationNotice');if(note){note.classList.toggle('is-ready',!!offer.creative_commercial_active);note.innerHTML=statusLine(offer)}
 const store=document.getElementById('wts34CreditPackages');if(store){store.replaceChildren();
  if(offer.creative_commercial_active&&offer.packages?.length){for(const pkg of offer.packages){const el=document.createElement('article');el.className='wts34-pack';el.innerHTML=`<b>${esc(pkg.name)}</b><strong>${number(pkg.credits)}</strong><small>${words('créditos','credits')} · ${cash(pkg.price_usdt)} USDT</small>`;store.append(el)}}
  else store.innerHTML=`<div class="wts35-service"><span class="wts35-service-icon">▧</span><b>Image AI</b><small>${words('Imágenes y campañas','Images and campaigns')}</small></div><div class="wts35-service"><span class="wts35-service-icon">▷</span><b>Video AI</b><small>${words('Clips para redes','Clips for social media')}</small></div><div class="wts35-service"><span class="wts35-service-icon">♫</span><b>Voice AI</b><small>${words('Narraciones y voz','Voiceovers and narration')}</small></div>`;
 }
};
async function init(){const root=document.getElementById('planes');if(!root)return;
 try{
  const resp=await fetch('/api/public/offer/v2734',{headers:{Accept:'application/json'},cache:'no-store',credentials:'same-origin'});
  if(!resp.ok)throw new Error('HTTP '+resp.status);
  const offer=await resp.json();if(!Array.isArray(offer.plans)||offer.plans.length!==4)throw new Error('incomplete offer');
  window.__wtCommercialOfferV2734=offer;render(offer);
  for(const id of ['billingMonthly','billingAnnual'])document.getElementById(id)?.addEventListener('click',()=>setTimeout(()=>render(offer),0));
  window.addEventListener('worktic:langchange',()=>setTimeout(()=>render(offer),0));
 }catch(e){
  document.getElementById('plansGrid').innerHTML=`<div class="wts34-unavailable" role="alert">${words('No podemos verificar los beneficios y precios ahora. Para evitar información incorrecta, la compra de planes está temporalmente deshabilitada desde esta página.','We cannot verify prices and plan rights right now. Purchases are temporarily unavailable from this page to avoid misleading information.')}</div>`;
  const table=document.getElementById('wts34Compare');if(table)table.textContent=words('Comparación temporalmente no disponible.','Comparison temporarily unavailable.');
 }
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();
