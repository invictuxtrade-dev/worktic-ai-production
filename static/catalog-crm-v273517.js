/* V27.35.17 - Vista de productos asociados en el CRM; sin insertar IDs manuales. */
(() => {
  'use strict';
  const safe = x => String(x ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const req = async (url, options={}) => {
    const res = await fetch(url, {credentials:'same-origin', headers: {'Content-Type':'application/json'}, ...options});
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw Error(data.error||data.message||`Error HTTP ${res.status}`);
    return data;
  };
  function container(){ return document.querySelector('#oppProductsV273517'); }
  const money=(p)=>`${safe(p.currency)} ${Number(p.price||0).toLocaleString('es-CO')}`;
  window.workticLoadOpportunityProductsV273517=async (opportunityID)=>{
    const element=container(); if(!element)return;
    try {
      const [assigned, catalog] = await Promise.all([
        req('/api/catalog/opportunity-items?opportunity_id='+encodeURIComponent(opportunityID)),
        req('/api/catalog/commerce')
      ]);
      if(container()!==element)return;
      const products=(Array.isArray(catalog)?catalog:[]).filter(p=>p.active && p.available && !assigned.some(a=>Number(a.product_id)===Number(p.id)));
      element.innerHTML=`<header class="opp-products-header"><div><strong>Productos asociados</strong><p>Intereses comerciales de este cliente · ${assigned.length} artículo(s)</p></div></header>
        <div class="opp-products-list">${assigned.length?assigned.map(p=>`<div class="opp-product-row"><div><strong>${safe(p.name)}</strong><small>${p.quantity} × ${money(p)} · ${p.active?'Activo':'Inactivo'}</small></div><button type="button" data-remove-product="${Number(p.product_id)}" class="secondary" aria-label="Quitar ${safe(p.name)}">Quitar</button></div>`).join(''):'<p class="opp-product-empty">Aún no hay productos asociados.</p>'}</div>
        <div class="opp-products-add"><label>Agregar desde Catálogo<select id="oppProductSelect"><option value="">Seleccionar un producto o servicio</option>${products.map(p=>`<option value="${Number(p.id)}">${safe(p.name)} · ${money({currency:p.currency,price:p.effective_price})}</option>`).join('')}</select></label>
        <label>Cantidad<input id="oppProductQty" type="number" min="1" max="10000" value="1"></label><button type="button" id="oppProductAdd" ${products.length?'':'disabled'}>Agregar</button></div>
        <p class="opp-products-error" role="alert" hidden></p>`;
      const errorEl=element.querySelector('.opp-products-error');
      const showErr=e=>{errorEl.hidden=false;errorEl.textContent=e.message||String(e)};
      element.querySelector('#oppProductAdd').onclick=async e=>{
        const id=Number(element.querySelector('#oppProductSelect').value),qty=Number(element.querySelector('#oppProductQty').value);
        if(!id||!Number.isSafeInteger(qty)||qty<1||qty>10000){showErr('Selecciona un artículo y cantidad válida.');return;}
        e.currentTarget.disabled=true;
        try {await req('/api/catalog/opportunity-link',{method:'POST',body:JSON.stringify({product_id:id,opportunity_id:opportunityID,quantity:qty})});await window.workticLoadOpportunityProductsV273517(opportunityID);}catch(err){showErr(err);e.currentTarget.disabled=false;}
      };
      element.querySelectorAll('[data-remove-product]').forEach(b=>b.onclick=async()=>{
        if(!confirm('¿Quitar este producto de la oportunidad? No se modifica el catálogo ni el inventario.'))return;
        b.disabled=true;
        try {await req('/api/catalog/opportunity-items?opportunity_id='+encodeURIComponent(opportunityID)+'&product_id='+encodeURIComponent(b.dataset.removeProduct),{method:'DELETE'});await window.workticLoadOpportunityProductsV273517(opportunityID);}catch(err){showErr(err);b.disabled=false;}
      });
    } catch(err) { if(container()===element)element.innerHTML='<p class="opp-products-error">'+safe(err.message)+'</p>'; }
  };
})();
