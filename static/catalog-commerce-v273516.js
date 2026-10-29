/* WorkticAI V27.35.16 — acciones comerciales sin alterar el diseño del backoffice. */
(() => {
  'use strict';
  const esc = text => String(text == null ? '' : text).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const request = async (path, opts={}) => {
    const r=await fetch(path,{credentials:'same-origin',headers:{'Content-Type':'application/json'},...opts});
    let json={};try{json=await r.json()}catch{}
    if(!r.ok)throw Error(json.error || json.message || `Error HTTP ${r.status}`);
    return json;
  };
  const notify = msg => {if(typeof window.toast === 'function') window.toast(msg);else alert(msg)};
  const modalDialog = html => { if(typeof window.modal === 'function')window.modal(html);else alert('Abre Catálogo desde la aplicación completa.');};
  const copy = async text => {try{await navigator.clipboard.writeText(text);notify('Información copiada')}catch{const a=document.createElement('textarea');a.value=text;document.body.append(a);a.select();document.execCommand('copy');a.remove();notify('Información copiada')}};
  const newButton = (text,fn) => {const b=document.createElement('button');b.type='button';b.textContent=text;b.className='secondary';b.addEventListener('click',fn);return b};
  window.catalogShareV273516=async function(productID=0){
    try{
      const d=await request('/api/catalog/share',{method:'POST',body:JSON.stringify({product_id:Number(productID)||0})});
      const grid=d.images?.length?`<div class="catalog-share-images">${d.images.slice(0,15).map(u=>`<img src="${esc(u)}" loading="lazy" alt="Imagen del catálogo">`).join('')}</div>`:'';
      modalDialog(`<div class="modal-heading"><span class="eyebrow">CATÁLOGO COMERCIAL</span><h2>${productID?'Compartir producto':'Compartir catálogo'}</h2><p>Copia la ficha con precios vigentes e imágenes. Los enlaces se pueden enviar desde tus canales.</p></div><div class="catalog-share-panel">${grid}<label>Mensaje para compartir<textarea id="catalogSharePayload" rows="9">${esc(d.text||'')}</textarea></label><div class="actions"><button type="button" id="catalogShareCopy">Copiar información</button><button type="button" id="catalogShareWhatsapp" class="secondary">Compartir por WhatsApp</button><button value="cancel" class="secondary">Cerrar</button></div><small>Compartir un enlace no equivale a enviar una foto como archivo multimedia nativo. Las transacciones requieren aprobación y verificación.</small></div>`);
      document.getElementById('catalogShareCopy')?.addEventListener('click',()=>copy(document.getElementById('catalogSharePayload').value));
      document.getElementById('catalogShareWhatsapp')?.addEventListener('click',()=>{
        const text=productID?(document.getElementById('catalogSharePayload')?.value||''):(d.share_url||document.getElementById('catalogSharePayload')?.value||'');
        window.open('https://wa.me/?text='+encodeURIComponent(text),'_blank','noopener,noreferrer');
      });
    }catch(e){notify(e.message)}
  };
  window.catalogLinkServiceV273516 = async function(id){
    const n=prompt('Duración del servicio en minutos (entre 5 y 1440):','30'); if(n===null)return;
    const minutes=Number(n);if(!Number.isInteger(minutes)||minutes<5||minutes>1440)return notify('Duración inválida');
    try{const out=await request('/api/catalog/service-link',{method:'POST',body:JSON.stringify({product_id:Number(id),duration_minutes:minutes})});notify('Servicio vinculado con Agenda #'+out.service_id)}catch(e){notify(e.message)}
  };
  window.catalogLinkOpportunityV273516=async function(id){
    const x=prompt('ID de la oportunidad CRM que deseas asociar:');if(x===null)return;
    const opp=Number(x);if(!Number.isSafeInteger(opp)||opp<1)return notify('ID inválido');
    try{await request('/api/catalog/opportunity-link',{method:'POST',body:JSON.stringify({product_id:Number(id),opportunity_id:opp,quantity:1})});notify('Producto asociado a la oportunidad')}catch(e){notify(e.message)}
  };
  window.catalogSaleV273516=async function(id){
    const q=prompt('Cantidad a registrar (sin descontar inventario hasta aprobar):','1');if(q===null)return;
    const qty=Number(q);if(!Number.isInteger(qty)||qty<1)return notify('Cantidad inválida');
    const ref=prompt('Referencia del comprobante de pago (no significa que esté verificado):');if(ref===null)return;
    try{const out=await request('/api/catalog/sales',{method:'POST',body:JSON.stringify({product_id:Number(id),quantity:qty,payment_reference:ref.trim()})});notify(`Pedido #${out.sale_id} pendiente de revisión. No se ha descontado inventario.`)}catch(e){notify(e.message)}
  };
  window.catalogSalesReviewV273516=async function(){
    try{
      const sales=await request('/api/catalog/sales');
      modalDialog(`<div class="modal-heading"><span class="eyebrow">CONTROL COMERCIAL</span><h2>Ventas y revisión</h2><p>Las referencias de pago no se verifican automáticamente. Solo un administrador puede aprobar tras comprobar el pago con su proveedor.</p></div><div class="catalog-sales-list">${sales.length?sales.map(s=>`<article><div><strong>Pedido #${s.id}</strong><small>Producto #${s.product_id} · ${s.quantity} × ${esc(s.currency)} ${s.unit_price}</small><small>${esc(s.status)} · ${esc(s.created_at)}</small></div>${s.status==='pending_review'?`<button type="button" data-review-sale="${s.id}">Revisar</button>`:''}</article>`).join(''):'<p>No hay pagos por revisar.</p>'}</div><div class="actions"><button value="cancel" class="secondary">Cerrar</button></div>`);
      document.querySelectorAll('[data-review-sale]').forEach(b=>b.addEventListener('click',async()=>{
        const id=Number(b.dataset.reviewSale);const confirmPayment=confirm('¿Verificaste el pago en tu proveedor bancario o blockchain? Aceptar=aprobar manualmente. Cancelar=rechazar o salir.');
        const decision=confirmPayment?'approve':'reject';if(!confirmPayment&&!confirm('¿Quieres rechazar este comprobante?'))return;
        const reason=prompt('Motivo obligatorio de la decisión:');if(!reason?.trim())return;
        try{await request('/api/catalog/sales/review',{method:'POST',body:JSON.stringify({sale_id:id,decision,manual_verification_confirmed:confirmPayment,reason:reason.trim()})});notify('Revisión registrada');window.catalogSalesReviewV273516();if(typeof window.loadProducts==='function')window.loadProducts()}catch(e){notify(e.message)}
      }));
    }catch(e){notify(e.message)}
  };
  async function refreshMetrics(){
    const grid=document.getElementById('productsGrid');if(!grid)return;
    try {
      const m=await request('/api/catalog/metrics');
      let row=document.getElementById('catalogMetricsV273516');
      if(!row){row=document.createElement('div');row.id='catalogMetricsV273516';row.className='catalog-metrics-row';grid.before(row)}
      row.innerHTML=[['Productos activos',m.products_active],['Agotados',m.out_of_stock],['Pagos por revisar',m.pending_review],['Ventas revisadas',m.approved_manual]].map(([label,value])=>`<div class="catalog-metric"><span>${esc(label)}</span><strong>${Number(value||0).toLocaleString('es-CO')}</strong></div>`).join('');
    }catch(e){/* No interrumpir el catálogo por un fallo de métricas. */}
  }
  function enrichCards(){
    const grid=document.getElementById('productsGrid');if(!grid)return;
    grid.querySelectorAll('.catalog-card').forEach(card=>{
      const edit=card.querySelector('button[onclick^="editProduct"]'); if(!edit)return;
      const match=edit.getAttribute('onclick')?.match(/"id"\s*:\s*(\d+)/);if(!match)return;
      const id=Number(match[1]);const target=card.querySelector('.actions');if(!target||target.dataset.commerce)return;target.dataset.commerce='1';
      target.append(newButton('Compartir',()=>window.catalogShareV273516(id)));
      target.append(newButton('Agregar a oportunidad',()=>window.catalogLinkOpportunityV273516(id)));
      if(edit.getAttribute('onclick')?.includes('"service"'))target.append(newButton('Vincular a Agenda',()=>window.catalogLinkServiceV273516(id)));
      target.append(newButton('Registrar pago',()=>window.catalogSaleV273516(id)));
    });
    if(!document.getElementById('catalogActionRowV273516')){
      const row=document.createElement('div');row.id='catalogActionRowV273516';row.className='catalog-commerce-actions';
      row.append(newButton('Compartir catálogo completo',()=>window.catalogShareV273516(0)),newButton('Revisión de ventas',window.catalogSalesReviewV273516));
      grid.before(row);
    }
  }
  function ready(){const grid=document.getElementById('productsGrid');if(!grid)return;
    const observer=new MutationObserver(()=>{if(grid.querySelector('.catalog-card:not(:has(.actions[data-commerce]))'))enrichCards()});
    observer.observe(grid,{childList:true});enrichCards();refreshMetrics();
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',ready);else ready();
})();
