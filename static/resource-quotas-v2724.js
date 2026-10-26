/* WorkticAI V27.24 — quota overview (staging). Never changes budgets. */
(()=>{'use strict';
  const make=(t,c,txt)=>{const n=document.createElement(t);if(c)n.className=c;if(txt!==undefined)n.textContent=String(txt);return n};
  const names={products:'Productos en catálogo',contacts:'Contactos CRM',appointments:'Citas activas',professionals:'Profesionales',services:'Servicios de agenda',analytics_events:'Eventos Analytics / mes',ads_syncs:'Sincronizaciones Ads / mes'};
  let inFlight=false;
  async function reload(){
    const node=document.getElementById('wrqV2724');if(!node||inFlight)return;
    inFlight=true;const list=node.querySelector('.wrq-body');list.replaceChildren(make('p','wrq-muted','Consultando los límites del plan…'));
    try{
      const resp=await fetch('/api/consumption/v2724/resources',{credentials:'same-origin',headers:{Accept:'application/json'}});
      if(!resp.ok)throw Error('HTTP '+resp.status);
      const d=await resp.json();list.replaceChildren();
      list.append(make('p','wrq-note',d.guard_active?'Los límites de las rutas integradas están activos.':'Vista de laboratorio: los controles V27.24 están desactivados.'));
      for(const [key,title] of Object.entries(names)){
        const item=make('div','wrq-item');
        const cap=d.limits?.[key];const used=(key==='analytics_events'||key==='ads_syncs')?d.monthly_meters?.[key]:d.current_resources?.[key];
        item.append(make('span','',title),make('strong','',cap===null||cap===undefined?'No configurado':
          (Number(used||0).toLocaleString('es-CO')+' / '+(Number(cap)<0?'Sin límite':Number(cap).toLocaleString('es-CO')))));
        list.append(item);
      }
      list.append(make('p','wrq-note','Los recursos reflejan el estado al consultar; Ads y Analytics muestran intentos del mes, no gastos facturados.'));
    }catch(e){list.replaceChildren(make('p','wrq-muted','No fue posible consultar cuotas: '+e.message));}
    finally{inFlight=false}
  }
  function start(){
    const studio=document.getElementById('studio');if(!studio)return;
    if(document.getElementById('wrqV2724'))return;
    const node=make('section','wrq-panel');node.id='wrqV2724';
    const header=make('div','wrq-top');
    const headings=make('div','');headings.append(make('span','wrq-eyebrow','WORKTICAI · V27.24'),make('h3','','Mis límites y consumo'));
    const button=make('button','wrq-reload','Actualizar');button.type='button';button.addEventListener('click',reload);header.append(headings,button);
    node.append(header,make('div','wrq-body'));studio.append(node);
    document.addEventListener('click',evt=>{const nav=evt.target.closest?.('[data-view]');if(nav?.dataset.view==='studio')setTimeout(reload,100)});
    reload();
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',start);else start();
})();
