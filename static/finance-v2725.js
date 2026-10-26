/* WorkticAI V27.25 — superadmin-only audit. Provider costs are never shown to tenants. */
(()=>{'use strict';
const $=(s,p=document)=>p.querySelector(s);
const el=(tag,cls,text)=>{const n=document.createElement(tag);if(cls)n.className=cls;if(text!==undefined)n.textContent=String(text);return n};
const USD=c=>'$'+(Number(c||0)/100).toLocaleString('en-US',{minimumFractionDigits:2,maximumFractionDigits:2});
const req=async(url,data)=>{const resp=await fetch(url,{credentials:'same-origin',method:data?'POST':'GET',headers:data?{'Accept':'application/json','Content-Type':'application/json'}:{'Accept':'application/json'},body:data?JSON.stringify(data):undefined});if(!resp.ok)throw Error('HTTP '+resp.status+' · '+(await resp.text()).slice(0,240));return resp.json()};
const input=(name,label,type='text')=>{const wrap=el('label','wfa-label');wrap.append(el('span','',label));const i=el('input');i.name=name;i.type=type;i.required=type!=='checkbox';if(type==='checkbox')i.required=true;wrap.append(i);return wrap};
const select=(name,label,values)=>{const w=el('label','wfa-label');w.append(el('span','',label));const s=el('select');s.name=name;for(const v of values){const o=el('option');o.value=v;o.textContent=v;s.append(o)}w.append(s);return w};
const period=()=>new Date().toISOString().slice(0,7);
const vendors=['openai','luma','meta','render','redis','storage','messaging','other'];
function table(rows,keys){const t=el('table','wfa-table'),head=el('thead'),tr=el('tr');for(const [key,heading] of keys)tr.append(el('th','',heading));head.append(tr);t.append(head);const body=el('tbody');for(const item of rows||[]){const r=el('tr');for(const [key] of keys)r.append(el('td','',item[key]??'—'));body.append(r)}t.append(body);return t}
function build(){if($('#wfaV2725')||!$('#admin'))return;const section=el('section','wfa-wrap');section.id='wfaV2725';
 const top=el('div','wfa-top');const heading=el('div');heading.append(el('small','','V27.25 · STAGING FINANCIERO'),el('h2','','Cost Accounting & Reconciliation'),el('p','','Facturas comprobadas, costos asignados y estimaciones por separado. Solo superadministración.'));
 const refresh=el('button','wfa-btn','Actualizar conciliación');refresh.type='button';refresh.onclick=load;top.append(heading,refresh);section.append(top);
 const msg=el('p','wfa-note','Lectura financiera todavía no disponible.');msg.id='wfaMessage';section.append(msg);
 const result=el('div','');result.id='wfaResults';section.append(result);
 const formHolder=el('div','wfa-forms');
 const inv=el('form','wfa-form');inv.id='wfaInvoice';inv.append(el('h3','','Registrar factura verificada'),el('p','','Solo tras verificar el comprobante de proveedor. Importes en centavos USD, no USDT.'));
 inv.append(select('provider','Proveedor',vendors),input('invoice_ref','Referencia única de factura'),input('period','Mes (AAAA-MM)'),input('total_cents','Total facturado en centavos USD','number'),input('evidence_ref','Referencia interna del comprobante'),input('verified_source','Certifico verificación independiente','checkbox'));
 const saveInv=el('button','wfa-btn','Registrar factura');saveInv.type='submit';inv.append(saveInv);inv.onsubmit=async(e)=>{e.preventDefault();const d=Object.fromEntries(new FormData(inv));d.total_cents=Number(d.total_cents);d.verified_source=$('[name=verified_source]',inv).checked;try{await req('/api/admin/finance/v2725/invoices',d);inv.reset();notify('Factura registrada.');load()}catch(err){notify(err.message)}};
 const actual=el('form','wfa-form');actual.id='wfaActual';actual.append(el('h3','','Asignar costo verificado'),el('p','','Requiere factura registrada y referencia independiente por operación. Tenant 0 = costo sin asignar.'));
 actual.append(select('provider','Proveedor',vendors),input('invoice_ref','Referencia de factura'),input('source_ref','Referencia única del cargo'),input('tenant_id','ID empresa (0: sin asignar)','number'),input('period','Mes (AAAA-MM)'),input('operation','Tipo de operación (ej.: video)'),input('hold_id','Reserva de Studio (vacío si no aplica)'),input('actual_cents','Costo en centavos USD','number'),input('verified_source','Certifico origen comprobado','checkbox'));
 const save=el('button','wfa-btn','Asignar a factura');save.type='submit';actual.append(save);actual.onsubmit=async(e)=>{e.preventDefault();const d=Object.fromEntries(new FormData(actual));d.tenant_id=Number(d.tenant_id);d.actual_cents=Number(d.actual_cents);d.verified_source=$('[name=verified_source]',actual).checked;try{await req('/api/admin/finance/v2725/actuals',d);actual.reset();notify('Asignación registrada.');load()}catch(err){notify(err.message)}};
 formHolder.append(inv,actual);section.append(formHolder);$('#admin').append(section);
 $('[name=period]',inv).value=period();$('[name=period]',actual).value=period();$('[name=tenant_id]',actual).value='0';$('[name=hold_id]',actual).required=false;
 function notify(m){msg.textContent=m;msg.classList.toggle('wfa-error',/HTTP|Error|error/i.test(m))}
}
async function load(){const root=$('#wfaV2725');if(!root)return;const msg=$('#wfaMessage'),where=$('#wfaResults');msg.textContent='Cargando datos financieros…';where.replaceChildren();try{
 const month=period(),[summary,invoices,meters]=await Promise.all([
      req('/api/admin/finance/v2725/summary?period='+month),
      req('/api/admin/finance/v2725/invoices?period='+month),
      req('/api/admin/finance/v2726/metering?period='+month)
  ]);
 const stats=el('div','wfa-stats');for(const [name,value] of [['Facturas comprobadas',USD(summary.verified_invoices_usd_cents)],['Asignado a costos',USD(summary.allocated_usd_cents)],['Sin asignar',USD(summary.unallocated_usd_cents)],['Consumo estimado',USD(summary.estimated_operations_usd_cents)],['Ventas aprobadas',`${summary.sales_approved_usdt||0} USDT`]]){const box=el('div','wfa-stat');box.append(el('small','',name),el('strong','',value));stats.append(box)}where.append(stats);
 where.append(el('h3','','Facturas registradas'),table((invoices.invoices||[]).map(x=>({...x,total_cents:USD(x.total_cents),allocated_cents:USD(x.allocated_cents),unallocated_cents:USD(x.unallocated_cents)})),[['provider','Proveedor'],['invoice_ref','Referencia'],['total_cents','USD'],['allocated_cents','Asignado'],['unallocated_cents','Pendiente']]));
 const pricingUS=x=>'$'+(Number(x||0)/1000000).toLocaleString('en-US',{minimumFractionDigits:4,maximumFractionDigits:4});
 const observed=(meters.operational_meter||[]).map(x=>({
   ...x,estimated_micro_usd:x.priced_operations===x.operations?pricingUS(x.estimated_micro_usd):
     pricingUS(x.estimated_micro_usd)+' (parcial; faltan tarifas)',
   input_tokens:Number(x.input_tokens||0).toLocaleString('es-CO'),
   output_tokens:Number(x.output_tokens||0).toLocaleString('es-CO'),
   units:Number(x.units||0).toLocaleString('es-CO')
 }));
 where.append(el('h3','','V27.26 · Medición automática de operaciones'));
 where.append(el('p','wfa-note',meters.enabled?'Registro operativo activado. Los costos configurados son estimaciones, NO facturas verificadas.':'Registro desactivado: requiere WORKTIC_METER_V2726=1 en staging.'));
 where.append(table(observed,[['provider','Proveedor'],['operation','Operación'],['status','Estado'],['operations','Operaciones'],['input_tokens','Entrada'],['output_tokens','Salida'],['units','Unidades'],['estimated_micro_usd','Estimación USD']]));
 where.append(el('h3','','Estimaciones agrupadas por empresa'));
 where.append(table((meters.by_tenant||[]).map(x=>({...x,estimated_micro_usd:
   x.priced_operations===x.operations?pricingUS(x.estimated_micro_usd):
   pricingUS(x.estimated_micro_usd)+' (parcial)'})),[['tenant_id','Empresa'],['provider','Proveedor'],['operation','Operación'],['operations','Operaciones'],['estimated_micro_usd','Estimación USD']]));
 where.append(el('h3','','Cuotas internas sin costo confirmado'));
 where.append(table(meters.internal_unpriced||[],[['resource','Recurso'],['attempts','Ejecuciones'],['companies','Empresas']]));
 where.append(el('h3','','Eventos externos sin precio confirmado'));
 where.append(table(meters.external_unpriced||[],[['provider','Proveedor'],['operation','Operación'],['status','Estado'],['operations','Eventos'],['units','Unidades']]));
 where.append(el('p','wfa-note','Sin costo confirmado no significa gratis. Los importes reales necesitan facturas y conciliación V27.25.'));
 where.append(el('p','wfa-note','No es margen neto: faltan conciliación completa de proveedores, infraestructura, Partners, anualidades diferidas e impuestos. Los importes simulados no son facturas.'));
 msg.textContent=summary.record_creation_enabled?'Carga manual habilitada para laboratorio.':'Solo lectura: activación de importación deshabilitada por seguridad.';for(const b of root.querySelectorAll('.wfa-form button'))b.disabled=!summary.record_creation_enabled;
 }catch(e){msg.textContent='No disponible (requiere superadministrador y backend V27.25): '+e.message}}
function init(){build();document.addEventListener('click',e=>{if(e.target.closest?.('[data-view="admin"]'))setTimeout(load,550)});}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();
