/* WorkticAI V27.31 — read-only superadmin profitability scenarios.
   Explicit assumptions required. NO approved margin or production gate. */
(()=>{'use strict';
const create=(tag,cls,txt)=>{const n=document.createElement(tag);if(cls)n.className=cls;if(txt!==undefined)n.textContent=String(txt);return n};
const USD=c=>'$'+(Number(c||0)/100).toLocaleString('en-US',{minimumFractionDigits:2,maximumFractionDigits:2});
const fields=[
 ['usd_per_usdt','USD recibidos por 1 USDT (cotización observada)','0.998','number'],
 ['fx_source','Fuente de cotización, verificable','Exchange / fuente y referencia','text'],
 ['fx_asof','Fecha de cotización (últimos 7 días)','AAAA-MM-DD','date'],
 ['annual_discount_bps','Descuento anual (puntos básicos; 2000 = 20%)','2000','number'],
 ['partners_subscription_bps','Comisión sobre suscripción (5000 = 50%)','5000','number'],
 ['partners_topup_bps','Comisión específica recargas (1000 = 10%)','1000','number'],
 ['payment_processing_bps','Comisión pasarela / liquidación (300 = 3%)','300','number'],
 ['variable_cost_buffer_bps','Colchón por variación proveedor (2000 = 20%)','2000','number'],
 ['target_margin_bps','Margen objetivo (2000 = 20%)','2000','number'],
 ['max_supplier_credit_cents','Costo máximo estimado por crédito creativo (centavos USD)','3','number'],
 ['max_supplier_response_cents','Costo máximo estimado por respuesta IA (centavos USD)','1','number'],
 ['infra_per_tenant_month_cents','Infraestructura por empresa/mes estimada (centavos USD)','500','number'],
 ['other_per_tenant_month_cents','Soporte, terceros y otros por empresa/mes (centavos USD)','200','number']
];
function drawTable(data, keys){const outer=create('div','wfu-scroll'),t=create('table','wfu-table'),h=create('thead'),tr=create('tr');for(const [,n] of keys)tr.append(create('th','',n));h.append(tr);t.append(h);const b=create('tbody');for(const row of data||[]){const r=create('tr');for(const [key] of keys)r.append(create('td','',row[key]??'—'));b.append(r)}t.append(b);outer.append(t);return outer}
function install(){const admin=document.getElementById('admin');if(!admin||document.getElementById('wfuV2731'))return;
const section=create('section','wfu-wrap');section.id='wfuV2731';section.append(create('small','','V27.31 · MODELO FINANCIERO EN LABORATORIO'),create('h2','','Rentabilidad por plan y recargas'),create('p','wfu-muted','No son utilidades reales. Requiere cotización USDT/USD, costos máximos por unidad e infraestructura. No modifica pagos, comisiones, suscripciones ni precios.'));
const f=create('form','wfu-form');const inputs=create('div','wfu-grid');for(const [k,description,placeholder,type] of fields){const l=create('label','');l.append(create('span','',description));const input=create('input','');input.name=k;input.type=type;input.placeholder=placeholder;input.required=true;if(type==='number'){input.min='0';input.step=k==='usd_per_usdt'?'0.00001':'1'}l.append(input);inputs.append(l)}f.append(inputs);
const btn=create('button','wfu-button','Calcular escenarios de prueba');btn.type='submit';f.append(btn);section.append(f);const message=create('p','wfu-muted','Los importes son hipótesis que debes validar en staging.');section.append(message);const result=create('div','wfu-result');section.append(result);admin.append(section);
async function calculate(e){if(e)e.preventDefault();btn.disabled=true;result.replaceChildren();message.textContent='Calculando escenarios con las hipótesis proporcionadas…';try{const q=new URLSearchParams(new FormData(f));q.set('period',new Date().toISOString().slice(0,7));const r=await fetch('/api/admin/finance/v2731/unit-economics?'+q.toString(),{credentials:'same-origin',cache:'no-store'});if(!r.ok)throw Error('HTTP '+r.status);const data=await r.json();if(data.status!=='scenario_only_not_verified'){message.textContent='Datos incompletos o no verificables: '+(data.missing_inputs||[]).join(', ');return}
message.textContent='ESCENARIOS, NO MÁRGENES CERTIFICADOS. Lanzamiento público: NO APROBADO.';
result.append(create('h3','','Planes actuales · consumo del 100% de cuotas incluidas'));
result.append(drawTable((data.plan_scenarios||[]).map(x=>({'plan':x.code,'ciclo':x.cycle,'ingreso':USD(x.modeled_monthly_revenue_usd_cents),'partners':USD(x.modeled_partners_usd_cents),'coste':USD(x.modeled_included_credit_cost_usd_cents+x.modeled_response_cost_usd_cents+x.modeled_variable_cost_buffer_usd_cents),'infra':USD(x.modeled_infrastructure_usd_cents+x.modeled_other_costs_usd_cents),'contrib':USD(x.modeled_contribution_usd_cents),'margen':(x.modeled_margin_bps/100).toFixed(1)+'%'})),[['plan','Plan'],['ciclo','Ciclo'],['ingreso','Ingreso USD'],['partners','Partners'],['coste','IA + reserva'],['infra','Infra + otros'],['contrib','Contribución'],['margen','Margen modelado']]));
result.append(create('h3','','Paquetes de créditos · comisión separada'));
result.append(drawTable((data.credit_packages||[]).map(x=>({'paquete':x.code,'creditos':x.credits,'ingreso':USD(x.revenue_usd_cents),'costo':USD(x.supplier_reserve_usd_cents+x.cost_buffer_usd_cents),'partners':USD(x.partners_usd_cents),'resultado':USD(x.contribution_usd_cents),'margen':(x.margin_bps/100).toFixed(1)+'%'})),[['paquete','Paquete'],['creditos','Créditos'],['ingreso','Venta USD'],['costo','Proveedor + riesgo'],['partners','Partners'],['resultado','Contribución'],['margen','Margen modelado']]));
result.append(create('p','wfu-muted','Operaciones del mes sin tarifa: '+data.operations_with_unknown_supplier_price+' · Facturas sin atribuir: '+USD(data.verified_invoice_usd_cents_unallocated)+'. Ingresos en USDT convertidos solo para este escenario; la cotización indicada no está validada automáticamente. No equivale a liquidez recibida.'));
const note=create('p','wfu-warning','Bloqueos de lanzamiento: faltan facturas reales de proveedores, medición de infraestructura por empresa, pruebas PostgreSQL y conciliación completa de Partners, devoluciones y pagos.');result.append(note);
}catch(err){message.textContent='No se pudo ejecutar escenario: '+err.message}finally{btn.disabled=false}}
f.addEventListener('submit',calculate);
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',install);else install();
})();
