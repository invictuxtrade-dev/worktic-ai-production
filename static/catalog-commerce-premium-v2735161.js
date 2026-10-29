/* WorkticAI V27.35.16.1 — Catálogo comercial, CRM y pagos: formularios premium.
   Mantiene V27.35.16 como motor de negocio y las mismas API de servidor. */
(() => {
  'use strict';
  const esc = (v) => String(v == null ? '' : v).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const icon = (name) => {
    const paths = {
      briefcase:'<rect x="3" y="7" width="18" height="14" rx="2"/><path d="M8 7V5a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M3 13h18"/>',
      cart:'<circle cx="9" cy="20" r="1"/><circle cx="19" cy="20" r="1"/><path d="M2 3h2l3 12h13l2-9H5"/>',
      check:'<path d="m5 12 4 4L19 6"/>',
      search:'<circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/>',
      shield:'<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z"/><path d="m9 12 2 2 4-4"/>',
      upload:'<path d="M12 16V3m-5 5 5-5 5 5"/><path d="M4 16v5h16v-5"/>',
      receipt:'<path d="M4 3h16v18l-4-2-4 2-4-2-4 2V3Z"/><path d="M8 8h8M8 12h8"/>',
      close:'<path d="M18 6 6 18M6 6l12 12"/>',
    };
    return `<svg aria-hidden="true" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${paths[name] || paths.check}</svg>`;
  };
  const json = async (path, options={}) => {
    const res=await fetch(path,{credentials:'same-origin',headers:{'Content-Type':'application/json'},...options});
    const data=await res.json().catch(()=>({}));
    if (!res.ok) throw new Error(data.error||data.message||`Error HTTP ${res.status}`);
    return data;
  };
  const notice = (message) => typeof window.toast === 'function' ? window.toast(message) : alert(message);
  const money = (amount,cur) => `${esc(cur||'COP')} ${Number(amount||0).toLocaleString('es-CO',{maximumFractionDigits:2})}`;
  const attr = val=>esc(val);
  let dlg = null;
  function close() { if(dlg){ dlg.close();dlg.remove();dlg=null; } }
  function show({eyebrow,title,subtitle,body,footer}){
    close();dlg=document.createElement('dialog');dlg.id='workticCatalogUxModal';dlg.className='catalog-ux-dialog';
    dlg.innerHTML=`<div class="catalog-ux-frame"><header class="catalog-ux-header"><div><small>${esc(eyebrow)}</small><h2>${esc(title)}</h2><p>${esc(subtitle||'')}</p></div><button type="button" class="catalog-ux-close" aria-label="Cerrar ventana" data-cx-close>${icon('close')}</button></header><div class="catalog-ux-scroll">${body}</div><footer class="catalog-ux-footer">${footer||'<button type="button" class="catalog-ux-secondary" data-cx-close>Cerrar</button>'}</footer></div>`;
    document.body.append(dlg);
    dlg.querySelectorAll('[data-cx-close]').forEach(b=>b.addEventListener('click',close));
    dlg.addEventListener('cancel',e=>{e.preventDefault();close()});
    dlg.addEventListener('click',e=>{if(e.target===dlg)close()});
    dlg.showModal();
    return dlg;
  }
  function error(message){
    const x=dlg?.querySelector('.catalog-ux-error');
    if(x){x.textContent=message;x.hidden=false;} else notice(message);
  }
  function setBusy(button,v){if(button){button.disabled=v;button.dataset.originalLabel ||= button.innerHTML;button.innerHTML=v?'Procesando…':button.dataset.originalLabel;}}
  let catalogCache = null;
  async function catalog(){
    const data=await json('/api/catalog/commerce');
    catalogCache=Array.isArray(data)?data:[];
    return catalogCache;
  }
  async function product(id){
    const item=(await catalog()).find(p=>Number(p.id)===Number(id));
    if(!item)throw Error('No se encontró el artículo en esta empresa');
    return item;
  }
  function productStrip(p){
    return `<div class="catalog-ux-product">${p.image_url?`<img src="${attr(p.image_url)}" alt="Portada de ${attr(p.name)}">`:`<div class="catalog-ux-placeholder">${icon('cart')}</div>`}<div><strong>${esc(p.name)}</strong><span>${p.type==='service'?'Servicio':'Producto'} · ${money(p.effective_price??p.price,p.currency)}</span><small>${p.unlimited_stock||p.type==='service'?'Disponibilidad según servicio':`${Number(p.stock||0)} unidades en inventario`}</small></div></div>`;
  }
  const opportunityText = (o) => [o.title,o.contact_name,o.owner,o.stage,o.contact_phone].filter(Boolean).join(' ');
  async function opportunities(){
    const values=await json('/api/crm/opportunities');
    if(!Array.isArray(values))throw Error('No se pudo leer la lista de oportunidades');
    return values.filter(o=>Number(o.id)>0);
  }
  function options(items,includeEmpty=true){
    return `${includeEmpty?'<option value="">Sin asociar por ahora</option>':''}${items.map(o=>`<option value="${Number(o.id)}">${esc(o.title||'Oportunidad')} · ${esc(o.contact_name||'Sin contacto')} · ${esc(o.stage||'')}</option>`).join('')}`;
  }
  window.catalogLinkOpportunityV273516=async (id)=>{
    try{
      const [p,opps]=await Promise.all([product(id),opportunities()]);
      let selected=0;
      show({eyebrow:'CRM Y CATÁLOGO',title:'Vincular a una oportunidad',subtitle:'Selecciona la oportunidad por nombre o cliente. No necesitas conocer ningún ID.',body:`${productStrip(p)}<label class="catalog-ux-label">Buscar oportunidad<input id="cxSearch" type="search" placeholder="Cliente, oportunidad, responsable o etapa" autocomplete="off" aria-label="Buscar oportunidades"></label><p class="catalog-ux-hint" id="cxCount">${opps.length} oportunidades encontradas</p><div id="cxOppList" class="catalog-ux-options" role="radiogroup" aria-label="Oportunidades del CRM"></div><div class="catalog-ux-fieldrow"><label>Cantidad de artículos<input id="cxQty" type="number" min="1" max="10000" value="1" required></label></div><p class="catalog-ux-error" hidden></p>`,footer:'<button class="catalog-ux-secondary" type="button" data-cx-close>Cancelar</button><button class="catalog-ux-primary" type="button" id="cxSave" disabled>Vincular oportunidad</button>'});
      const list=dlg.querySelector('#cxOppList');
      const save=dlg.querySelector('#cxSave');
      const render=(term='')=>{
        const filtered=opps.filter(o=>opportunityText(o).toLocaleLowerCase('es').includes(term.toLocaleLowerCase('es'))).slice(0,100);
        dlg.querySelector('#cxCount').textContent=`${filtered.length} resultado(s) ${opps.length>100?'· mostrando hasta 100':''}`;
        list.innerHTML=filtered.length?filtered.map(o=>`<label class="catalog-ux-choice ${selected===Number(o.id)?'selected':''}"><input type="radio" name="cxOpportunity" value="${Number(o.id)}" ${selected===Number(o.id)?'checked':''}><span class="catalog-ux-choice-body"><strong>${esc(o.title||'Oportunidad sin título')}</strong><small>${esc(o.contact_name||'Sin contacto asociado')} · ${esc(o.stage||'Sin etapa')}${o.owner?' · '+esc(o.owner):''}</small></span><span class="catalog-ux-choice-mark">${icon('check')}</span></label>`).join(''):'<div class="catalog-ux-empty">No hay oportunidades con esa búsqueda. Puedes crear una en CRM → Oportunidades.</div>';
      };
      render();
      dlg.querySelector('#cxSearch').addEventListener('input',e=>render(e.target.value.trim()));
      list.addEventListener('change',e=>{if(e.target.name==='cxOpportunity'){selected=Number(e.target.value);save.disabled=!selected;render(dlg.querySelector('#cxSearch').value.trim());}});
      save.addEventListener('click',async()=>{
        const qty=Number(dlg.querySelector('#cxQty').value);
        if(!selected||!Number.isSafeInteger(qty)||qty<1||qty>10000)return error('Selecciona una oportunidad y una cantidad válida.');
        setBusy(save,true);
        try{await json('/api/catalog/opportunity-link',{method:'POST',body:JSON.stringify({product_id:Number(id),opportunity_id:selected,quantity:qty})});close();notice('Producto vinculado a la oportunidad del CRM.');}catch(e){error(e.message);setBusy(save,false);}
      });
    }catch(e){notice(e.message);}
  };
  function statusLabel(s){return ({pending_review:'Pendiente de revisión',approved_manual:'Aprobada manualmente',rejected:'Rechazada'})[s]||s;}
  async function uploadEvidence(id,file){
    if(!file)return;
    if(file.size>4*1024*1024)throw Error('El comprobante supera el máximo de 4 MB');
    if(!['image/jpeg','image/png','image/webp','application/pdf'].includes(file.type))throw Error('Solo se permiten imágenes JPG, PNG, WEBP o PDF');
    const f=new FormData();f.set('sale_id',String(id));f.set('proof',file);
    const res=await fetch('/api/catalog/sales/evidence',{method:'POST',credentials:'same-origin',body:f});
    const d=await res.json().catch(()=>({}));
    if(!res.ok)throw Error(d.error||'No se pudo adjuntar el comprobante');
    return d;
  }
  window.catalogSaleV273516=async(id)=>{
    try{
      const [p,opps]=await Promise.all([product(id),opportunities()]);
      show({eyebrow:'REGISTRO COMERCIAL',title:'Registrar venta y comprobante',subtitle:'Registra un pedido para revisión. El sistema no confirma pagos automáticamente.',body:`${productStrip(p)}<div class="catalog-ux-step"><b><span>1</span> Datos del pedido</b><div class="catalog-ux-fieldrow"><label>Cantidad <em>Obligatorio</em><input id="cxSaleQty" type="number" min="1" max="10000" step="1" value="1" required></label><label>Oportunidad CRM <em>Opcional</em><select id="cxSaleOpp">${options(opps)}</select></label></div><div class="catalog-ux-total"><span>Importe estimado</span><strong id="cxSaleTotal">${money(p.effective_price??p.price,p.currency)}</strong></div></div><div class="catalog-ux-step"><b><span>2</span> Comprobante de pago</b><label>Referencia de transferencia o transacción <em>Obligatorio</em><input id="cxSaleRef" type="text" maxlength="180" minlength="8" placeholder="Referencia o hash de 8 a 180 caracteres" required></label><label>Adjuntar comprobante <em>Opcional</em><input id="cxSaleProof" type="file" accept="image/jpeg,image/png,image/webp,application/pdf"><small>Foto o PDF, máximo 4 MB. Archivo privado para revisión administrativa.</small></label></div><div class="catalog-ux-warning">${icon('shield')} El comprobante y su referencia son evidencia aportada, no un pago verificado. La venta quedará pendiente; no se descuenta inventario hasta su aprobación.</div><p class="catalog-ux-error" hidden></p>`,footer:'<button class="catalog-ux-secondary" type="button" data-cx-close>Cancelar</button><button class="catalog-ux-primary" type="button" id="cxRegister">Registrar para revisión</button>'});
      const qty=dlg.querySelector('#cxSaleQty'),ref=dlg.querySelector('#cxSaleRef'),save=dlg.querySelector('#cxRegister'),proof=dlg.querySelector('#cxSaleProof');
      qty.addEventListener('input',()=>{const n=Number(qty.value);dlg.querySelector('#cxSaleTotal').textContent=money(Math.max(0,n)*Number(p.effective_price??p.price),p.currency)});
      save.addEventListener('click',async()=>{
        const n=Number(qty.value),r=ref.value.trim(),file=proof.files?.[0];
        if(!Number.isSafeInteger(n)||n<1||n>10000)return error('Indica una cantidad válida entre 1 y 10.000.');
        if(p.type==='product'&&!p.unlimited_stock&&n>Number(p.stock||0))return error('No hay inventario suficiente para esa cantidad.');
        if(r.length<8||r.length>180)return error('La referencia del pago debe contener entre 8 y 180 caracteres.');
        if(file&&(file.size>4*1024*1024||!['image/jpeg','image/png','image/webp','application/pdf'].includes(file.type)))return error('Adjunta solo JPG, PNG, WEBP o PDF de máximo 4 MB.');
        setBusy(save,true);
        let d;
        try{d=await json('/api/catalog/sales',{method:'POST',body:JSON.stringify({product_id:Number(id),opportunity_id:Number(dlg.querySelector('#cxSaleOpp').value||0),quantity:n,payment_reference:r})});}
        catch(e){error(e.message);setBusy(save,false);return;}
        let uploaded=false,uploadError='';
        if(file){try{await uploadEvidence(d.sale_id,file);uploaded=true;}catch(e){uploadError=e.message;}}
        show({eyebrow:'REGISTRO COMPLETADO',title:`Pedido #${d.sale_id} pendiente de revisión`,subtitle:'El pago no ha sido aprobado; el inventario permanece igual.',body:`<div class="catalog-ux-success">${icon('check')} Pedido guardado correctamente</div><div class="catalog-ux-total"><span>Importe registrado por servidor</span><strong>${money(d.total,d.currency)}</strong></div><p>${file?(uploaded?'Comprobante almacenado de forma privada.':'El pedido fue registrado, pero el comprobante NO se pudo adjuntar: '+esc(uploadError)):'No se adjuntó comprobante. Puedes subirlo desde Revisión de ventas.'}</p><div class="catalog-ux-warning">Revisa el pago con tu proveedor antes de aprobarlo.</div>${file&&!uploaded?'<button type="button" class="catalog-ux-secondary" id="cxRetryUpload">Reintentar adjunto</button>':''}`,footer:'<button type="button" class="catalog-ux-primary" data-cx-close>Entendido</button>'});
        if(file&&!uploaded)dlg.querySelector('#cxRetryUpload').addEventListener('click',async(e)=>{const b=e.currentTarget;setBusy(b,true);try{await uploadEvidence(d.sale_id,file);b.replaceWith(Object.assign(document.createElement('p'),{textContent:'Comprobante adjuntado correctamente.'}));}catch(err){notice(err.message);setBusy(b,false);}});
        if(typeof window.loadProducts==='function')window.loadProducts();
      });
    }catch(e){notice(e.message);}
  };
  window.catalogSalesReviewV273516=async()=>{
    try{
      const [sales,items]=await Promise.all([json('/api/catalog/sales'),catalog()]);
      const byID=new Map(items.map(p=>[Number(p.id),p]));
      const status=s=>`<span class="catalog-ux-status ${esc(s)}">${esc(statusLabel(s))}</span>`;
      show({eyebrow:'VENTAS Y AUDITORÍA',title:'Pedidos y revisión de pagos',subtitle:'Administra referencias y comprobantes, con revisión humana y trazabilidad.',body:`<div class="catalog-ux-warning">${icon('shield')} Ninguna captura o hash verifica por sí solo una transferencia. Comprueba el pago directamente con el proveedor.</div><div class="catalog-ux-filters"><label>Filtrar estado<select id="cxSalesFilter"><option value="pending_review">Pendientes</option><option value="all">Todas</option><option value="approved_manual">Aprobadas</option><option value="rejected">Rechazadas</option></select></label></div><div id="cxSalesList" class="catalog-ux-sales-list"></div><p class="catalog-ux-error" hidden></p>`,footer:'<button type="button" class="catalog-ux-secondary" data-cx-close>Cerrar</button>'});
      const list=dlg.querySelector('#cxSalesList');
      const render=()=>{
        const filter=dlg.querySelector('#cxSalesFilter').value;
        const filtered=sales.filter(s=>filter==='all'||s.status===filter);
        list.innerHTML=filtered.length?filtered.map(s=>`<article class="catalog-ux-sale"><div class="catalog-ux-sale-row"><div><strong>Pedido #${Number(s.id)} · ${esc(byID.get(Number(s.product_id))?.name||'Producto #'+s.product_id)}</strong><small>${Number(s.quantity)} unidad(es) · ${money(s.total,s.currency)} · ${esc(s.created_at||'')}</small></div>${status(s.status)}</div><div class="catalog-ux-muted">Referencia: ${esc(s.payment_reference)}</div><div class="catalog-ux-sale-actions">${s.has_evidence?`<a class="catalog-ux-secondary" href="/api/catalog/sales/evidence?sale_id=${Number(s.id)}" target="_blank" rel="noopener noreferrer">${icon('receipt')} Descargar comprobante</a>`:'<span class="catalog-ux-muted">Sin archivo adjunto</span>'}${s.status==='pending_review'?`<button type="button" class="catalog-ux-secondary" data-cx-upload="${Number(s.id)}">${icon('upload')} Adjuntar</button><button type="button" class="catalog-ux-primary" data-cx-review="${Number(s.id)}">Revisar</button>`:''}</div></article>`).join(''):'<div class="catalog-ux-empty">No hay pedidos en este estado.</div>';
      };
      render();dlg.querySelector('#cxSalesFilter').addEventListener('change',render);
      list.addEventListener('click',async(e)=>{
        const upload=e.target.closest('[data-cx-upload]');
        const review=e.target.closest('[data-cx-review]');
        if(upload){
          const saleID=Number(upload.dataset.cxUpload);const inp=document.createElement('input');inp.type='file';inp.accept='image/jpeg,image/png,image/webp,application/pdf';inp.addEventListener('change',async()=>{if(!inp.files?.[0])return;setBusy(upload,true);try{await uploadEvidence(saleID,inp.files[0]);const sale=sales.find(s=>Number(s.id)===saleID);if(sale)sale.has_evidence=true;render();notice('Comprobante adjuntado');}catch(err){error(err.message);setBusy(upload,false);}});inp.click();return;
        }
        if(review){
          const saleID=Number(review.dataset.cxReview),sale=sales.find(s=>Number(s.id)===saleID);
          if(!sale)return;
          show({eyebrow:'REVISIÓN ADMINISTRATIVA',title:`Revisar pedido #${saleID}`,subtitle:'Aprobar es una declaración de verificación manual, no automática.',body:`<div class="catalog-ux-total"><span>Total del pedido</span><strong>${money(sale.total,sale.currency)}</strong></div><p>Referencia: <strong>${esc(sale.payment_reference)}</strong></p>${sale.has_evidence?`<a href="/api/catalog/sales/evidence?sale_id=${saleID}" class="catalog-ux-secondary" target="_blank" rel="noopener noreferrer">Descargar comprobante</a>`:'<div class="catalog-ux-warning">No hay comprobante adjunto. Verifica el pago directamente en tu proveedor.</div>'}<label class="catalog-ux-label">Decisión<select id="cxReviewDecision"><option value="approve">Aprobar manualmente</option><option value="reject">Rechazar</option></select></label><label class="catalog-ux-label">Motivo de la decisión <em>Obligatorio</em><textarea id="cxReviewReason" rows="3" placeholder="Ej. Transacción confirmada en el proveedor con fecha y monto, o referencia inválida"></textarea></label><label class="catalog-ux-check"><input id="cxVerified" type="checkbox"> <span>He verificado personalmente el pago con el proveedor (obligatorio para aprobar).</span></label><p class="catalog-ux-error" hidden></p>`,footer:'<button class="catalog-ux-secondary" type="button" data-cx-close>Cancelar</button><button class="catalog-ux-primary" type="button" id="cxSaveReview">Registrar decisión</button>'});
          const decision=dlg.querySelector('#cxReviewDecision'),check=dlg.querySelector('#cxVerified'),button=dlg.querySelector('#cxSaveReview');
          decision.addEventListener('change',()=>{check.closest('label').hidden=decision.value==='reject';});
          button.addEventListener('click',async()=>{
            const reason=dlg.querySelector('#cxReviewReason').value.trim();
            if(!reason)return error('Indica el motivo de la decisión.');
            if(decision.value==='approve'&&!check.checked)return error('Debes verificar el pago antes de aprobar.');
            setBusy(button,true);
            try{await json('/api/catalog/sales/review',{method:'POST',body:JSON.stringify({sale_id:saleID,decision:decision.value,reason,manual_verification_confirmed:decision.value==='approve'&&check.checked})});close();notice('Decisión registrada en auditoría');if(typeof window.loadProducts==='function')window.loadProducts();}
            catch(err){error(err.message);setBusy(button,false);}
          });
        }
      });
    }catch(e){notice(e.message);}
  };
})();
