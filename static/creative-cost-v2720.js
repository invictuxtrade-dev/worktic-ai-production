/* WorkticAI V27.20 - conservative Studio cost estimates and superadmin dashboard.
   No customer credits are sold or debited here. Server guard is authoritative. */
(()=>{'use strict';
const fmt = v => (Number(v||0)/100).toLocaleString('en-US',{style:'currency',currency:'USD'});
const api = async(url)=>{const r=await fetch(url,{credentials:'same-origin',headers:{'Accept':'application/json'}});if(!r.ok)throw Error('HTTP '+r.status);return r.json()};
window.WorkticCostConfirm = async (action, extra={}) => {
  try {
    const q=new URLSearchParams({action,duration:String(extra.duration||0),chars:String(extra.chars||0)});
    const v=await api('/api/commercial/quote?'+q);
    if(!v.allowed_by_plan_and_cost){window.alert('Esta función no está incluida en tu plan o su costo excede el presupuesto autorizado.');return false;}
    return window.confirm(`Worktic Studio · Confirmación\n\nCréditos: ${v.credits}\nReserva estimada de proveedor: ${fmt(v.estimated_vendor_cost_cents)}\n\n${v.enabled?'Se descontarán créditos de tu billetera si la solicitud se envía al proveedor.':'Piloto sin cobros de créditos comerciales.'}\n¿Continuar?`);
  } catch (e) { window.alert('No se pudo verificar el costo estimado. Operación detenida.'); return false; }
};
function cell(label,value){const el=document.createElement('div');el.className='wcc-kpi';const s=document.createElement('span');s.textContent=label;const b=document.createElement('b');b.textContent=value;el.append(s,b);return el}
async function studioStatus(){
 const el=document.getElementById('wccStudioStatus');if(!el)return;
 try{const v=await api('/api/studio/cost/status'),b=v.budget;
  el.textContent=`Protección activa · ${fmt(v.available_cents)} de presupuesto externo estimado disponible este mes`;
  el.classList.toggle('wcc-empty',!v.available_cents);
 }catch{el.textContent='Presupuesto no disponible · Las generaciones externas permanecerán protegidas';el.classList.add('wcc-empty')}
}
async function adminStatus(){
 const el=document.getElementById('wccAdminData');if(!el)return;
 el.textContent='Consultando presupuestos…';
 try{
  const d=await api('/api/admin/cost-intelligence');el.replaceChildren();
  const budgets=(d.budgets||[]),tenants=budgets.filter(b=>b.tenant_id!==0),global=budgets.find(b=>b.tenant_id===0);
  const top=document.createElement('div');top.className='wcc-grid';
  top.append(cell('Mes auditado',d.period),cell('Presupuesto global',fmt(global?.limit_cents)),cell('Consumo estimado global',fmt(global?.estimated_spent_cents)),cell('Reservas pendientes',fmt(global?.reserved_cents)));
  el.append(top);
  const note=document.createElement('p');note.className='wcc-note';note.textContent='Importes estimados para control preventivo: no representan facturas verificadas, utilidad empresarial ni saldo comercial. Solo incluye generaciones externas de Worktic Studio.';el.append(note);
  const table=document.createElement('table');table.className='wcc-table';
  const th=document.createElement('thead'),hrow=document.createElement('tr');for(const label of ['Empresa ID','Límite','Reservado','Estimado','Disponible']){const h=document.createElement('th');h.textContent=label;hrow.append(h)}th.append(hrow);table.append(th);
  const body=document.createElement('tbody');for(const b of tenants){const tr=document.createElement('tr');for(const value of [b.tenant_id,fmt(b.limit_cents),fmt(b.reserved_cents),fmt(b.estimated_spent_cents),fmt(Math.max(0,b.limit_cents-b.reserved_cents-b.estimated_spent_cents))]){const td=document.createElement('td');td.textContent=String(value);tr.append(td)}body.append(tr)}table.append(body);el.append(table);
  const ops=(d.operations||[]);if(ops.length){const h=document.createElement('h3');h.textContent='Consumo por operación y proveedor';el.append(h);const ot=document.createElement('table');ot.className='wcc-table';const hr=document.createElement('tr');for(const x of ['Operación','Proveedor','Estado','Solicitudes','Costo estimado']){const th=document.createElement('th');th.textContent=x;hr.append(th)}const head=document.createElement('thead');head.append(hr);ot.append(head);const ob=document.createElement('tbody');for(const op of ops){const tr=document.createElement('tr');for(const v of [op.operation,op.provider,op.status,op.requests,fmt(op.estimated_cents)]){const td=document.createElement('td');td.textContent=String(v);tr.append(td)}ob.append(tr)}ot.append(ob);el.append(ot)}
  if(!tenants.length){const p=document.createElement('p');p.className='wcc-note';p.textContent='Sin generaciones registradas durante este mes. Esto es normal antes de tener clientes.';el.append(p)}
 }catch(e){el.textContent='No se pudieron consultar los datos. Esta sección requiere sesión de superadministrador. '+e.message}
}
function initialize(){
 const studio=document.querySelector('#studio .wts-app');if(studio&&!document.getElementById('wccStudioStatus')){const b=document.createElement('div');b.id='wccStudioStatus';b.className='wcc-status';b.textContent='Protección de gastos creativos';studio.prepend(b)}
 const admin=document.getElementById('admin');if(admin&&!document.getElementById('wccAdminData')){const section=document.createElement('article');section.className='wcc-admin';section.innerHTML='<div class="wcc-head"><div><span class="wcc-eyebrow">V27.20 · PROTECCIÓN FINANCIERA</span><h2>Creative Cost Intelligence</h2><p>Control preventivo de gasto externo por empresa y presupuesto global</p></div><button id="wccReload" type="button" class="secondary">Actualizar costos</button></div><div id="wccAdminData">Datos disponibles para superadministración</div>';admin.append(section);section.querySelector('#wccReload').addEventListener('click',adminStatus)}
 document.addEventListener('click',e=>{const b=e.target.closest('button[data-view]');if(!b)return;if(b.dataset.view==='studio')setTimeout(studioStatus,450);if(b.dataset.view==='admin')setTimeout(adminStatus,400)});
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',initialize);else initialize();
})();
