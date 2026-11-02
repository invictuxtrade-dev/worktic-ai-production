(()=>{
let workflows=[], current=null, graph={nodes:[],edges:[]}, selected=null;
// V27.35.47 — active agents are loaded only from the tenant-scoped Agents API.
let wtAgentsV273547=[],wtAgentsReadyV273547=false,wtAgentsLoadingV273547=null,wtAgentsErrorV273547='',wtAgentsAttemptedV273547=false;
async function loadAutomationAgentsV273547(force=false){
 if(wtAgentsReadyV273547&&!force)return wtAgentsV273547;
 if(wtAgentsLoadingV273547)return wtAgentsLoadingV273547;
 wtAgentsAttemptedV273547=true;
 wtAgentsLoadingV273547=(async()=>{try{
   const data=await api('/api/agents');
   if(!data||!Array.isArray(data.agents))throw Error('No se pudo consultar el catálogo de agentes');
   wtAgentsV273547=data.agents.filter(a=>a.status==='active'&&Number.isSafeInteger(Number(a.id))&&Number(a.id)>0).map(a=>({id:String(a.id),name:String(a.name||'Agente')}));
   wtAgentsReadyV273547=true;wtAgentsErrorV273547='';
  }catch(e){wtAgentsReadyV273547=false;wtAgentsErrorV273547=String(e?.message||'Error de consulta')}
  finally{wtAgentsLoadingV273547=null}
  if(selected&&graph.nodes.some(n=>n.id===selected&&n.type==='ai_task'))renderProps();
  return wtAgentsV273547;
 })();return wtAgentsLoadingV273547;
}
function renderAutomationAgentV273547(n){
 const id=String(n.config?.agent_id||'');
 const missing=id&&!wtAgentsV273547.some(a=>a.id===id);
 return `<label>Agente IA de esta empresa<select data-cfg="agent_id"><option value="">Selecciona un agente activo</option>${missing?`<option value="${safe(id)}" selected disabled>Agente anterior no disponible</option>`:''}${wtAgentsV273547.map(a=>`<option value="${safe(a.id)}"${a.id===id?' selected':''}>${safe(a.name)}</option>`).join('')}</select></label>`+
 `<button type="button" class="secondary mini" id="wtAutoReloadAgentsV273547">Actualizar agentes</button>`+
 `<p class="muted" style="font-size:12px">${wtAgentsErrorV273547?safe(wtAgentsErrorV273547):wtAgentsReadyV273547?'Solo aparecen agentes activos de esta empresa.':'Consultando agentes...'}</p>`+
 `<label>Tipo de resultado<select data-cfg="output_mode"><option value="text"${n.config?.output_mode!=='classification'?' selected':''}>Texto / recomendación</option><option value="classification"${n.config?.output_mode==='classification'?' selected':''}>Clasificación (intención, prioridad y resumen)</option></select></label>`+
 `<p class="muted" style="font-size:12px">Para usarlo en una condición posterior, selecciona el campo <b>ai.intent</b>, <b>ai.priority</b>, <b>ai.summary</b> o <b>ai.reply</b>. El agente no envía mensajes ni cambia CRM por sí mismo.</p>`;
}

const $a=s=>document.querySelector(s);
const safe=s=>esc(String(s??''));
const nodeTypes={trigger:'Disparador',condition:'Condición',delay:'Espera',send_whatsapp:'WhatsApp',ai_task:'Agente IA',tag_contact:'Etiquetar contacto',move_pipeline:'Mover pipeline',assign_owner:'Asignar responsable',create_task:'Crear tarea',webhook:'Webhook'};
const opportunityStagesV273535=['Nuevo','Contactado','Interesado','Calificado','Cotización','Negociación','Ganado','Perdido'];
function opportunityStageSelectV273535(key,caption,value='',optional=false){
  const options=(optional?['']:[]).concat(opportunityStagesV273535);
  return `<label>${caption}<select data-stage-field="${key}">${options.map(s=>`<option value="${safe(s)}"${s===value?' selected':''}>${safe(s||'Cualquier etapa')}</option>`).join('')}</select></label>`;
}
// V27.35.44 — form trigger: names are fetched from this tenant's forms catalog.
// V27.35.85: share the pending request between builder and list activation.
// A failed refresh must NOT authorize activation with a stale catalog.
let wtFormsV273544=[];
let wtFormsLoadedV273544=false;
let wtFormsLoadingV273544=false;
let wtFormsPromiseV273585=null;
let wtFormsErrorV273585='';
async function loadFormsForAutomationsV273544(force=false){
  if(wtFormsLoadedV273544&&!force)return wtFormsV273544;
  if(wtFormsPromiseV273585)return wtFormsPromiseV273585;
  wtFormsLoadingV273544=true;
  if(force)wtFormsLoadedV273544=false;
  wtFormsPromiseV273585=(async()=>{
    try{
      const items=await api('/api/marketing/forms');
      if(!Array.isArray(items))throw new Error('Respuesta inválida de formularios');
      wtFormsV273544=items.filter(x=>Number(x.id)>0&&x.active!==false)
        .map(x=>({value:String(x.id),label:String(x.name||x.slug||('Formulario '+x.id))}));
      wtFormsLoadedV273544=true;
      wtFormsErrorV273585='';
    }catch(e){
      wtFormsLoadedV273544=false;
      wtFormsErrorV273585=String(e?.message||'Error consultando formularios');
      console.warn('No se pudieron cargar formularios para automatizaciones:',wtFormsErrorV273585);
    }finally{
      wtFormsLoadingV273544=false;
      wtFormsPromiseV273585=null;
    }
    if($a('#wfTrigger')?.value==='form.submitted')renderTriggerSettingsV273535();
    return wtFormsV273544;
  })();
  return wtFormsPromiseV273585;
}
function renderTriggerSettingsV273535(){
  const control=$a('#wfTrigger');if(!control)return;
  let panel=$a('#wtAutoStageFiltersV273535');
  if(!panel){panel=document.createElement('div');panel.id='wtAutoStageFiltersV273535';panel.style.cssText='margin-top:8px;grid-column:1/-1;display:grid;grid-template-columns:repeat(auto-fit,minmax(145px,1fr));gap:12px';control.parentElement.insertAdjacentElement('afterend',panel)}
  const type=control.value;
  if(type==='form.submitted'&&!wtFormsLoadedV273544&&!wtFormsLoadingV273544)void loadFormsForAutomationsV273544();
  const options=type==='opportunity.stage_changed'?[{key:'from_stage',label:'Etapa anterior',choices:opportunityStagesV273535},{key:'stage',label:'Nueva etapa',choices:opportunityStagesV273535}]
    :type==='message.received'?[{key:'channel',label:'Canal de origen',choices:[['whatsapp_cloud','WhatsApp Business Cloud'],['messenger','Facebook Messenger'],['web','Chat Web']]}]
    :type==='social.comment'?[{key:'platform',label:'Red del comentario',choices:[['facebook','Facebook'],['instagram','Instagram'],['youtube','YouTube']]}]
    :type==='form.submitted'?[{key:'form_id',label:'Formulario de origen',choices:wtFormsV273544.map(x=>[x.value,x.label])}]:[];
  panel.style.display=options.length?'grid':'none';
  if(!options.length){panel.replaceChildren();return}
  current=current||{trigger_config:{}};
  const conf=current.trigger_config||{};
  panel.innerHTML=options.map(o=>{
    const choices=o.choices.map(item=>Array.isArray(item)?item:[item,item]);
    if(o.key==='form_id'&&conf.form_id&&!choices.some(x=>x[0]===String(conf.form_id)))choices.unshift([String(conf.form_id),'Formulario guardado #'+conf.form_id+' (no disponible)']);
    return `<label>${safe(o.label)}<select data-trigger-field="${safe(o.key)}"><option value="">${o.key==='form_id'&&wtIsCaptacionGraphV273569(graph)?'Selecciona un formulario (obligatorio)':'Todos / cualquiera'}</option>${choices.map(([value,label])=>`<option value="${safe(value)}"${String(conf[o.key]||'')===value?' selected':''}>${safe(label)}</option>`).join('')}</select></label>`;
  }).join('');
  if(type==='form.submitted'){
    const note=document.createElement('div');
    note.style.cssText='grid-column:1/-1;font-size:12px;color:#66718e;display:flex;gap:10px;align-items:center;flex-wrap:wrap';
    const detail=document.createElement('span');
    detail.textContent=wtFormsLoadingV273544?'Consultando formularios de esta empresa…':
      !wtFormsLoadedV273544?'No se pudo cargar el catálogo de formularios. '+(wtFormsErrorV273585||''):
      wtFormsV273544.length?'Formularios disponibles: '+wtFormsV273544.length:'Esta empresa todavía no tiene formularios disponibles.';
    const refresh=document.createElement('button');refresh.type='button';refresh.className='secondary mini';
    refresh.textContent='Actualizar formularios';refresh.disabled=wtFormsLoadingV273544;
    refresh.onclick=()=>void loadFormsForAutomationsV273544(true);
    note.append(detail,refresh);panel.appendChild(note);
  }
  panel.querySelectorAll('[data-trigger-field]').forEach(select=>select.addEventListener('change',()=>{
    current.trigger_config=current.trigger_config||{};
    if(select.value)current.trigger_config[select.dataset.triggerField]=select.value;
    else delete current.trigger_config[select.dataset.triggerField];
  }));
}

// V27.35.40 — Configuración guiada: canales disponibles y validación previa.
let wtWaCloudV273540=[];
let wtWaCloudLoadedV273540=false;
let wtWaCloudAttemptedV273540=false;
let wtWaCloudErrorV273540='';
let wtWaCloudPromiseV273540=null;
async function loadWaCloudV273540(force=false){
  if(wtWaCloudLoadedV273540&&!force)return wtWaCloudV273540;
  if(wtWaCloudPromiseV273540)return wtWaCloudPromiseV273540;
  wtWaCloudAttemptedV273540=true;
  wtWaCloudPromiseV273540=(async()=>{
    try{
      const r=await api('/api/channels/connections');
      if(!r||!Array.isArray(r.connections))throw new Error('La API no devolvió conexiones válidas');
      wtWaCloudV273540=r.connections.filter(c=>c.type==='whatsapp_cloud'&&Number(c.id)>0)
        .map(c=>({id:String(c.id),name:String(c.name||'WhatsApp Cloud'),status:String(c.status||'')}));
      wtWaCloudErrorV273540='';wtWaCloudLoadedV273540=true;
    }catch(e){wtWaCloudLoadedV273540=false;wtWaCloudErrorV273540=String(e?.message||'Error de conexión')}
    finally{wtWaCloudPromiseV273540=null}
    if(selected&&graph.nodes.some(n=>n.id===selected&&n.type==='send_whatsapp'))renderProps();
    return wtWaCloudV273540;
  })();
  return wtWaCloudPromiseV273540;
}
function renderWaCloudSelectV273540(n){
  const value=String(n.config?.connection_id||'');
  const available=wtWaCloudV273540.filter(c=>c.status==='connected');
  const missing=value&&!available.some(c=>c.id===value);
  let options='<option value="">Selecciona una conexión conectada</option>';
  if(missing)options+=`<option value="${safe(value)}" selected disabled>Anterior no disponible (${safe(value)})</option>`;
  options+=available.map(c=>`<option value="${safe(c.id)}"${c.id===value?' selected':''}>${safe(c.name)}</option>`).join('');
  return `<label>Conexión de WhatsApp Business Cloud<select id="wtWaCloudSelectV273540" data-cfg="connection_id">${options}</select></label>`+
    `<p class="muted" style="font-size:12px">${wtWaCloudErrorV273540?'No se pudieron consultar los canales: '+safe(wtWaCloudErrorV273540):wtWaCloudLoadedV273540?available.length?'Solo se muestran canales conectados de esta empresa.':'No hay conexiones Cloud activas. Configúralas en Canales.':'Consultando canales configurados…'}</p>`+
    '<button type="button" class="secondary mini" id="wtReloadWaV273540">Actualizar conexiones</button>';
}
function wtActivationIssuesV273540(g){
  const issues=[];
  if(!g||!Array.isArray(g.nodes)||!Array.isArray(g.edges))return ['El diagrama está incompleto.'];
  if(g.nodes.length<2)return ['Agrega una acción después de Inicio.'];
  const known=new Set(g.nodes.map(n=>n.id));
  const start=g.nodes.find(n=>n.type==='trigger');
  if(!start)return ['No se encontró el disparador Inicio.'];
  const reached=new Set();
  function visit(id){if(!id||reached.has(id))return;reached.add(id);g.edges.filter(e=>e.from===id).forEach(e=>visit(e.to))}
  visit(start.id);
  for(const n of g.nodes){
    if(!reached.has(n.id))issues.push(`Conecta el bloque «${n.label||n.type}» desde Inicio.`);
    const c=n.config||{},name=n.label||n.type;
    if(n.type==='send_whatsapp'){
      const conn=String(c.connection_id||'');
      if(!conn)issues.push(`Selecciona la conexión WhatsApp del bloque «${name}».`);
      else if(!wtWaCloudLoadedV273540)issues.push('Actualiza la lista de canales antes de activar WhatsApp.');
      else if(wtWaCloudLoadedV273540&&!wtWaCloudV273540.some(x=>x.id===conn&&x.status==='connected'))issues.push(`La conexión WhatsApp de «${name}» no está conectada.`);
      if(!String(c.message||'').trim())issues.push(`Completa el mensaje del bloque «${name}».`);
    }
    if(n.type==='condition'&&!String(c.field||'').trim())issues.push(`Selecciona el campo de «${name}».`);
    if(n.type==='tag_contact'&&!String(c.tag||'').trim())issues.push(`Escribe la etiqueta de «${name}».`);
    if(n.type==='move_pipeline'&&!opportunityStagesV273535.includes(String(c.stage||'')))issues.push(`Selecciona una etapa válida en «${name}».`);
    if(n.type==='assign_owner'&&(!/^[1-9]\d*$/.test(String(c.owner_user_id||''))||!wtSalesUsersReadyV273545||!wtSalesUsersV273545.some(u=>u.id===String(c.owner_user_id))))issues.push(`Selecciona un responsable activo para «${name}».`);
    if(n.type==='create_task'){
      const title=String(c.title||'').trim(), hours=Number(c.due_hours);
      if(title.length<3||title.length>180)issues.push(`El título de «${name}» debe tener entre 3 y 180 caracteres.`);
      if(!Number.isSafeInteger(hours)||hours<1||hours>720)issues.push(`«${name}» necesita un vencimiento de 1 a 720 horas.`);
      if(c.owner_user_id&&(!wtSalesUsersReadyV273545||!wtSalesUsersV273545.some(u=>u.id===String(c.owner_user_id))))issues.push(`Responsable no disponible en «${name}».`);
    }
    if(n.type==='ai_task'){
     const aid=String(c.agent_id||'');
     if(!aid||!wtAgentsReadyV273547||!wtAgentsV273547.some(a=>a.id===aid))issues.push(`Selecciona un agente IA activo en «${name}».`);
     if(c.output_mode&&!['text','classification'].includes(c.output_mode))issues.push(`Modo IA inválido en «${name}».`);
     if(String(c.prompt||'').length>2000)issues.push(`Instrucción IA demasiado larga en «${name}».`);
    }
    if(n.type==='webhook'&&!/^https:\/\//i.test(String(c.url||'')))issues.push(`El webhook «${name}» necesita una URL HTTPS.`);
    if(n.type==='delay'&&(!Number.isSafeInteger(Number(c.seconds))||Number(c.seconds)<0||Number(c.seconds)>2592000))issues.push(`La espera «${name}» debe estar entre 0 segundos y 30 días.`);
  }
  if(g.edges.some(e=>!known.has(e.from)||!known.has(e.to)))issues.push('El diagrama contiene conexiones sin destino válido.');
  return issues;
}
function wtShowActivationIssuesV273540(issues){
  if(!issues.length)return true;
  toast('Antes de activar: '+issues.slice(0,3).join(' · ')+(issues.length>3?` (+${issues.length-3} más)` :''));
  return false;
}

// V27.35.45 — Users come from this tenant's existing CRM Sales Hub.
let wtSalesUsersV273545=[];
let wtSalesUsersReadyV273545=false;
let wtSalesUsersAttemptedV273545=false;
let wtSalesUsersErrorV273545='';
let wtSalesUsersPromiseV273545=null;
async function loadSalesHubUsersV273545(force=false){
  if(wtSalesUsersReadyV273545&&!force)return wtSalesUsersV273545;
  if(wtSalesUsersPromiseV273545)return wtSalesUsersPromiseV273545;
  wtSalesUsersAttemptedV273545=true;
  wtSalesUsersPromiseV273545=(async()=>{
    try{
      const data=await api('/api/crm/sales-hub');
      if(!data||!Array.isArray(data.users))throw new Error('Catálogo de responsables no disponible');
      wtSalesUsersV273545=data.users.filter(u=>Number.isSafeInteger(Number(u.id))&&Number(u.id)>0)
        .map(u=>({id:String(u.id),name:String(u.name||'Usuario'),role:String(u.role||'')}));
      wtSalesUsersReadyV273545=true;wtSalesUsersErrorV273545='';
    }catch(e){wtSalesUsersReadyV273545=false;wtSalesUsersErrorV273545=String(e?.message||'Error de consulta')}
    finally{wtSalesUsersPromiseV273545=null}
    const node=graph.nodes.find(n=>n.id===selected);
    if(node&&['assign_owner','create_task'].includes(node.type))renderProps();
    return wtSalesUsersV273545;
  })();
  return wtSalesUsersPromiseV273545;
}
function renderSalesHubOwnerV273545(node,required){
  const selectedID=String(node.config?.owner_user_id||'');
  const stale=selectedID&&!wtSalesUsersV273545.some(u=>u.id===selectedID);
  const empty=required?'Selecciona un responsable':'Usar responsable de la oportunidad';
  const staleOption=stale?`<option value="${safe(selectedID)}" selected disabled>Anterior no disponible (#${safe(selectedID)})</option>`:'';
  return `<label>Responsable<select data-cfg="owner_user_id"><option value="">${empty}</option>${staleOption}${wtSalesUsersV273545.map(u=>`<option value="${safe(u.id)}"${u.id===selectedID?' selected':''}>${safe(u.name)} · ${safe(u.role)}</option>`).join('')}</select></label>`+
    `<p class="muted" style="font-size:12px">${wtSalesUsersErrorV273545?'No se pudo consultar responsables: '+safe(wtSalesUsersErrorV273545):wtSalesUsersReadyV273545?'Solo usuarios activos de esta empresa.':'Consultando responsables…'}</p>`+
    '<button type="button" id="wtReloadOwnersV273545" class="secondary mini">Actualizar responsables</button>';
}
function automationTaskSettingsV273545(n){
  const hours=n.config?.due_hours??24;
  return `<label>Título de la tarea<input data-cfg="title" maxlength="180" value="${safe(n.config?.title||'')}" placeholder="Ej. Llamar al cliente"></label>`+
    `<label>Vence dentro de (horas)<input type="number" min="1" max="720" step="1" data-cfg="due_hours" value="${safe(hours)}"></label>`+
    renderSalesHubOwnerV273545(n,false)+
    '<p class="muted" style="font-size:12px">Crea una tarea real en Oportunidades → Seguimiento. El evento debe identificar una oportunidad. Si no eliges responsable, se usa el asignado a esa oportunidad.</p>';
}
// V27.35.39 — Editor de conexiones. Edges are the source of truth;
// never rebuild a branched graph from the array order.
function ensureAutomationEditorStylesV273539(){
  if(document.getElementById('wtAutomationEditorStylesV273539'))return;
  const style=document.createElement('style');style.id='wtAutomationEditorStylesV273539';
  style.textContent=`
    #automationCanvas .wt-auto-overview{font-size:12px;color:#66718e;margin:0 0 14px;line-height:1.45}
    #automationCanvas .auto-node{position:relative;margin-bottom:14px;cursor:pointer;transition:border-color .15s ease,box-shadow .15s ease}
    #automationCanvas .auto-node.selected{box-shadow:0 0 0 2px rgba(99,102,241,.22)}
    #automationCanvas .wt-auto-links{display:flex;flex-wrap:wrap;gap:5px;margin-top:12px;padding-top:10px;border-top:1px solid #e8eaf5}
    #automationCanvas .wt-auto-route{font-size:11px;padding:5px 8px;border-radius:9px;background:#f2f4ff;color:#34416d;word-break:break-word}
    #automationCanvas .wt-auto-route.yes{background:#e5f8ee;color:#176140}
    #automationCanvas .wt-auto-route.no{background:#fff1e6;color:#935019}
    #automationCanvas .wt-auto-route.missing{background:#fff5ee;color:#99561d}
    #automationCanvas .wt-auto-orphan{border:1px dashed #efad81 !important}
    #nodeProperties .wt-route-settings{border-top:1px solid #e8eaf5;padding-top:12px;margin-top:15px}
    #nodeProperties .wt-route-note{font-size:12px;color:#66718e;line-height:1.5;margin:6px 0 12px}
    #nodeProperties .wt-route-settings label{display:block;margin:10px 0}
    #nodeProperties .wt-route-settings select{max-width:100%;width:100%}
    #nodeProperties .wt-route-action{display:flex;align-items:center;gap:6px;margin-top:6px;flex-wrap:wrap}
    #nodeProperties .wt-route-action select{flex:1;min-width:125px}
    #nodeProperties .wt-route-action button{white-space:nowrap}
  `;
  document.head.appendChild(style);
}
function automationNodeNameV273539(id){const n=graph.nodes.find(x=>x.id===id);return n?(n.label||nodeTypes[n.type]||n.type):'Finalizar flujo'}
function automationOutgoingV273539(id){return graph.edges.filter(e=>e.from===id)}
function automationReachableV273539(){
  const start=graph.nodes.find(n=>n.type==='trigger');const found=new Set();
  function go(id){if(!id||found.has(id))return;found.add(id);for(const edge of automationOutgoingV273539(id))go(edge.to)}
  if(start)go(start.id);return found;
}
function automationOrderedNodesV273539(){
  const start=graph.nodes.find(n=>n.type==='trigger'),visited=new Set(),list=[];
  function visit(id){if(!id||visited.has(id))return;const n=graph.nodes.find(x=>x.id===id);if(!n)return;visited.add(id);list.push(n);
    const order={yes:0,next:1,no:2};for(const e of automationOutgoingV273539(id).slice().sort((a,b)=>(order[a.branch||'next']??1)-(order[b.branch||'next']??1)))visit(e.to)
  }
  if(start)visit(start.id);for(const n of graph.nodes)visit(n.id);return list;
}
function automationHasCycleV273539(edges){
  const seen=new Set(),stack=new Set();const byId=new Map();
  for(const e of edges){if(!byId.has(e.from))byId.set(e.from,[]);byId.get(e.from).push(e.to)}
  function walk(id){if(stack.has(id))return true;if(seen.has(id))return false;seen.add(id);stack.add(id);for(const to of byId.get(id)||[])if(walk(to))return true;stack.delete(id);return false}
  return graph.nodes.some(n=>walk(n.id));
}
function automationSetRouteV273539(from,branch,to){
  const node=graph.nodes.find(n=>n.id===from);
  if(!node||!['next','yes','no'].includes(branch))return {ok:false,error:'Ruta desconocida'};
  if(node.type!=='condition'&&branch!=='next')return {ok:false,error:'Solo una condición admite Sí/No'};
  if(to&&(!graph.nodes.some(n=>n.id===to&&n.type!=='trigger')||to===from))return {ok:false,error:'Destino inválido'};
  let edges=graph.edges.map(e=>({...e}));
  if(node.type==='condition'){
    // Editing a legacy linear condition changes its success output into YES.
    edges=edges.map(e=>e.from===from&&(e.branch==='next'||!e.branch)?{...e,branch:'yes'}:e);
  }
  edges=edges.filter(e=>!(e.from===from&&(e.branch||'next')===branch));
  if(to)edges.push({from,to,branch});
  if(automationHasCycleV273539(edges))return {ok:false,error:'Esa conexión formaría un ciclo'};
  graph.edges=edges;return {ok:true};
}
function automationAddNodeV273539(type,fromId='',branch='next'){
  if(!current)newWorkflow();
  if(!Object.prototype.hasOwnProperty.call(nodeTypes,type)||type==='trigger')return null;
  const stamp=Date.now().toString(36);let id=type+'_'+stamp,idx=0;
  while(graph.nodes.some(n=>n.id===id)){idx++;id=type+'_'+stamp+'_'+idx}
  const n={id,type,label:nodeTypes[type]||type,config:{}};
  if(type==='delay')n.config.seconds=5;
  if(type==='condition')n.config={field:'score',operator:'equals',value:'80'};
  if(type==='send_whatsapp')n.config={connection_id:'',to:'',message:'Hola, gracias por contactarnos.'};
  if(type==='tag_contact')n.config={tag:'interesado'};
  if(type==='move_pipeline')n.config={stage:'Contactado'};
  if(type==='ai_task')n.config={agent_id:'',output_mode:'classification',prompt:'Analiza este lead y recomienda el siguiente paso.'};
  if(type==='create_task')n.config={title:'Hacer seguimiento comercial',due_hours:24,owner_user_id:''};
  if(type==='assign_owner')n.config={owner_user_id:''};
  const prev=graph.nodes.find(x=>x.id===fromId)||graph.nodes[graph.nodes.length-1];
  let old='';
  if(prev){
    if(prev.type==='condition'){
      branch=branch==='no'?'no':'yes';
      const existing=automationOutgoingV273539(prev.id).find(e=>e.branch===branch||(branch==='yes'&&(!e.branch||e.branch==='next')));
      old=existing?.to||'';
      graph.edges=graph.edges.filter(e=>e!==existing);
      if(branch==='yes')graph.edges=graph.edges.filter(e=>e.from!==prev.id||!(e.branch==='next'||!e.branch));
    }else{
      branch='next';const existing=automationOutgoingV273539(prev.id).find(e=>e.branch==='next'||!e.branch);
      old=existing?.to||'';graph.edges=graph.edges.filter(e=>e!==existing);
    }
  }
  graph.nodes.push(n);
  if(prev)graph.edges.push({from:prev.id,to:id,branch});
  if(old)graph.edges.push({from:id,to:old,branch:'next'});
  selected=id;renderCanvas();renderProps();return id;
}
function automationRouteControlsV273539(n){
  const routeFor=branch=>{const outgoing=automationOutgoingV273539(n.id);return outgoing.find(e=>e.branch===branch||(!e.branch&&branch==='next')||(n.type==='condition'&&branch==='yes'&&(!e.branch||e.branch==='next')))?.to||''};
  const targetChoices=(branch)=>`<option value="">Finalizar aquí / Sin conexión</option>`+graph.nodes.filter(x=>x.id!==n.id&&x.type!=='trigger').map(x=>`<option value="${safe(x.id)}"${routeFor(branch)===x.id?' selected':''}>${safe(x.label||nodeTypes[x.type])}</option>`).join('');
  const row=(branch,label)=>`<label>${label}<select data-auto-route="${branch}">${targetChoices(branch)}</select></label>`;
  const note='<p class="wt-route-note">Las flechas determinan qué ejecuta el motor. Una ruta sin conexión finaliza el recorrido. No se permiten ciclos.</p>';
  if(n.type==='condition'){
    return `<div class="wt-route-settings"><strong>Ramas de la condición</strong>${note}${row('yes','Si cumple (Sí) →')}${row('no','Si no cumple (No) →')}<div class="wt-route-action"><select id="wtAutoBranchNewType" aria-label="Nuevo bloque en rama No">${Object.keys(nodeTypes).filter(t=>t!=='trigger').map(t=>`<option value="${safe(t)}">${safe(nodeTypes[t])}</option>`).join('')}</select><button class="secondary mini" type="button" id="wtAutoAddNoBranch">+ Crear en No</button></div></div>`;
  }
  return `<div class="wt-route-settings"><strong>Siguiente paso</strong>${note}${row('next','Continuar hacia →')}</div>`;
}

// V27.35.66 — Plantilla comercial: formulario → IA → tareas → espera.
// Datos solamente; permanece borrador hasta seleccionar agente y responsable.
function wtCaptacionDraftV273566(){
  return {
    name:'Captación inteligente · formulario a seguimiento',
    description:'Selecciona el formulario, agente IA y responsable. Las tareas usan la oportunidad vinculada al lead; no se envían mensajes.',
    status:'draft',
    trigger_type:'form.submitted',
    trigger_config:{},
    graph:{
      nodes:[
        {id:'wt566_inicio',type:'trigger',label:'Nuevo formulario',config:{}},
        {id:'wt566_ai',type:'ai_task',label:'Clasificar prospecto',config:{agent_id:'',output_mode:'classification',prompt:'Clasifica la intención comercial de este prospecto. Devuelve intent, priority (alta, media o baja) y summary. No inventes información.'}},
        {id:'wt566_prioridad',type:'condition',label:'¿Prioridad alta?',config:{field:'ai.priority',operator:'equals',value:'alta'}},
        {id:'wt566_responsable',type:'assign_owner',label:'Asignar responsable',config:{owner_user_id:''}},
        {id:'wt566_tarea',type:'create_task',label:'Primera tarea comercial',config:{title:'Contactar al prospecto {{name}}',due_hours:24,owner_user_id:''}},
        {id:'wt566_espera',type:'delay',label:'Esperar seguimiento',config:{seconds:3600}},
        {id:'wt566_seguimiento',type:'create_task',label:'Segunda tarea comercial',config:{title:'Revisar seguimiento de {{name}}',due_hours:24,owner_user_id:''}},
        {id:'wt566_prioritario',type:'tag_contact',label:'Marcar prioritario',config:{tag:'prioridad-alta'}},
        {id:'wt566_normal',type:'tag_contact',label:'Seguimiento normal',config:{tag:'seguimiento-normal'}}
      ],
      edges:[
        {from:'wt566_inicio',to:'wt566_ai',branch:'next'},
        {from:'wt566_ai',to:'wt566_prioridad',branch:'next'},
        {from:'wt566_prioridad',to:'wt566_prioritario',branch:'yes'},
        {from:'wt566_responsable',to:'wt566_tarea',branch:'next'},
        {from:'wt566_tarea',to:'wt566_espera',branch:'next'},
        {from:'wt566_espera',to:'wt566_seguimiento',branch:'next'},
        {from:'wt566_prioritario',to:'wt566_responsable',branch:'next'},
        {from:'wt566_prioridad',to:'wt566_normal',branch:'no'}
      ]
    }
  };
}
// Reutiliza la barra existente de plantillas; sin cambios de HTML ni CSS.
function wtInstallCaptacionButtonV273566(){
  if(document.querySelector('[data-template="captacion_ia"]'))return;
  const anchor=document.querySelector('[data-template]');
  if(!anchor||!anchor.parentElement)return;
  const button=document.createElement('button');
  button.type='button';
  button.className=anchor.className;
  button.dataset.template='captacion_ia';
  button.textContent='Captación IA + seguimiento';
  button.title='Plantilla editable sin envíos automáticos de WhatsApp';
  button.onclick=()=>applyTemplate('captacion_ia');
  anchor.parentElement.appendChild(button);
}

window.loadRules=async function(){bindTabs(); wtInstallCaptacionButtonV273566(); await loadWorkflowList(); void loadFormsForAutomationsV273544();};
function bindTabs(){if($a('#wfTrigger'))$a('#wfTrigger').onchange=()=>{if(current){current.trigger_type=$a('#wfTrigger').value;current.trigger_config={}}renderTriggerSettingsV273535();renderCanvas()};document.querySelectorAll('[data-auto-tab]').forEach(b=>b.onclick=()=>{document.querySelectorAll('[data-auto-tab],.auto-panel').forEach(x=>x.classList.remove('active'));b.classList.add('active');$a('#auto-'+b.dataset.autoTab).classList.add('active');if(b.dataset.autoTab==='history')loadExec();});if($a('#newWorkflow'))$a('#newWorkflow').onclick=()=>newWorkflow();if($a('#saveWorkflow'))$a('#saveWorkflow').onclick=saveWorkflow;if($a('#testWorkflow'))$a('#testWorkflow').onclick=testWorkflow;document.querySelectorAll('[data-add-node]').forEach(b=>b.onclick=()=>addNode(b.dataset.addNode));document.querySelectorAll('[data-template]').forEach(b=>b.onclick=()=>applyTemplate(b.dataset.template));}
async function loadWorkflowList(){workflows=await api('/api/automations/workflows');renderList();renderStats();}
function renderStats(){const total=workflows.length,active=workflows.filter(x=>x.status==='active').length,runs=workflows.reduce((a,x)=>a+(x.runs||0),0),ok=workflows.reduce((a,x)=>a+(x.successes||0),0);$a('#automationStats').innerHTML=[[total,'Flujos'],[active,'Activos'],[runs,'Ejecuciones'],[runs?Math.round(ok/runs*100)+'%':'—','Éxito']].map(x=>`<div class="stat"><b>${x[0]}</b><span>${x[1]}</span></div>`).join('')}
function renderList(){const el=$a('#workflowList');el.innerHTML=workflows.length?workflows.map(w=>`<article class="workflow-card"><div class="wf-top"><div><span class="eyebrow">${safe(w.trigger_type)}</span><h3>${safe(w.name)}</h3></div><span class="wf-status ${safe(w.status)}">${safe(w.status)}</span></div><p>${safe(w.description||'Sin descripción')}</p><div class="wf-metrics"><div><b>${w.runs||0}</b><small>Runs</small></div><div><b>${w.successes||0}</b><small>OK</small></div><div><b>${w.failures||0}</b><small>Error</small></div></div><div class="actions"><button onclick="editWorkflow(${w.id})">Editar</button><button class="secondary" onclick="toggleWorkflow(${w.id})">${w.status==='active'?'Pausar':'Activar'}</button><button class="danger" onclick="deleteWorkflow(${w.id})">Eliminar</button></div></article>`).join(''):'<div class="empty-state"><b>Aún no hay workflows</b><p>Crea uno desde cero o usa una plantilla.</p></div>'}
window.editWorkflow=id=>{const stored=workflows.find(x=>x.id===id);if(!stored)return;current=JSON.parse(JSON.stringify(stored));graph=current.graph||{nodes:[],edges:[]};if(typeof graph==='string')graph=JSON.parse(graph);$a('#wfName').value=current.name||'';$a('#wfDesc').value=current.description||'';$a('#wfStatus').value=current.status||'draft';$a('#wfTrigger').value=current.trigger_type||'manual';renderTriggerSettingsV273535();renderCanvas();switchTab('builder')};
const wtActivationBusyV273585=new Set();
window.toggleWorkflow=async id=>{
  const w=workflows.find(x=>x.id===id);
  if(!w||wtActivationBusyV273585.has(id))return;
  wtActivationBusyV273585.add(id);
  const updated={...w,status:w.status==='active'?'paused':'active'};
  try{
    if(updated.status==='active'){
      const candidate=typeof w.graph==='string'?JSON.parse(w.graph):w.graph;
      // Always await the catalog on direct activation from the list.
      // It also handles a request already in flight from the builder.
      if(wtIsCaptacionGraphV273569(candidate))await loadFormsForAutomationsV273544(true);
      const issue=wtCaptacionFormIssueV273569(candidate,w.trigger_config);
      if(issue){toast(issue);return;}
      if(candidate?.nodes?.some(n=>n.type==='send_whatsapp'))await loadWaCloudV273540();
      if(candidate?.nodes?.some(n=>['assign_owner','create_task'].includes(n.type)))await loadSalesHubUsersV273545(true);
      if(candidate?.nodes?.some(n=>n.type==='ai_task'))await loadAutomationAgentsV273547(true);
      if(!wtShowActivationIssuesV273540(wtActivationIssuesV273540(candidate)))return;
    }
    await api('/api/automations/workflows',{method:'PUT',body:JSON.stringify(updated)});
    toast('Estado actualizado');await loadWorkflowList();
  }catch(e){toast(e?.message||'No se pudo actualizar el estado')}
  finally{wtActivationBusyV273585.delete(id)}
};
window.deleteWorkflow=async id=>{if(!confirm('¿Eliminar este workflow?'))return;await api('/api/automations/workflows?id='+id,{method:'DELETE'});loadWorkflowList()};
function newWorkflow(){current={name:'Nuevo flujo',description:'',status:'draft',trigger_type:'manual',trigger_config:{}};selected=null;graph={nodes:[{id:'trigger_'+Date.now(),type:'trigger',label:'Inicio',config:{}}],edges:[]};$a('#wfName').value=current.name;$a('#wfDesc').value='';$a('#wfStatus').value='draft';$a('#wfTrigger').value='manual';renderTriggerSettingsV273535();renderCanvas();renderProps();switchTab('builder')}
function switchTab(t){const b=document.querySelector(`[data-auto-tab="${t}"]`);if(b)b.click()}
function addNode(type){
  if(!current)newWorkflow();
  const origin=selected||graph.nodes[graph.nodes.length-1]?.id||'';
  automationAddNodeV273539(type,origin,'yes');
}
function renderCanvas(){
  ensureAutomationEditorStylesV273539();
  const c=$a('#automationCanvas');if(!c)return;
  const reached=automationReachableV273539();
  const nodes=automationOrderedNodesV273539();
  const arrows=n=>{
    const outgoing=automationOutgoingV273539(n.id);
    const branches=n.type==='condition'?['yes','no']:['next'];
    return `<div class="wt-auto-links">${branches.map(branch=>{
      const e=outgoing.find(x=>x.branch===branch||(branch==='yes'&&n.type==='condition'&&(!x.branch||x.branch==='next'))||(branch==='next'&&!x.branch));
      const label=branch==='yes'?'SÍ':branch==='no'?'NO':'SIGUE';
      const target=e?.to?automationNodeNameV273539(e.to):'Fin del recorrido';
      return `<span class="wt-auto-route ${branch}${!e?' missing':''}">↳ ${label} · ${safe(target)}</span>`;
    }).join('')}</div>`;
  };
  c.innerHTML='<p class="wt-auto-overview">Selecciona un bloque para cambiar sus propiedades y conectar sus salidas. Los recorridos Sí/No se ejecutan según estas conexiones; no según el orden de la lista.</p>'+
    (nodes.length?nodes.map(n=>`<div class="auto-node ${selected===n.id?'selected':''} ${!reached.has(n.id)?'wt-auto-orphan':''}" data-node="${safe(n.id)}" tabindex="0" role="button"><span class="node-type">${safe(nodeTypes[n.type]||n.type)}</span><h4>${safe(n.label||nodeTypes[n.type])}</h4><p>${safe(nodeSummary(n))}</p>${!reached.has(n.id)?'<p style="color:#a25b2b;font-size:12px">Bloque sin conexión desde el inicio</p>':''}${arrows(n)}${n.type!=='trigger'?`<button class="secondary mini" data-remove="${safe(n.id)}" type="button">Quitar</button>`:''}</div>`).join(''):'<div class="empty-state">Agrega un disparador y acciones.</div>');
  c.querySelectorAll('[data-node]').forEach(e=>{
    const activate=()=>{selected=e.dataset.node;renderCanvas();renderProps()};
    e.onclick=ev=>{if(ev.target.closest('[data-remove]'))return;activate()};
    e.onkeydown=ev=>{if((ev.key==='Enter'||ev.key===' ')&&ev.target===e){ev.preventDefault();activate()}};
  });
  c.querySelectorAll('[data-remove]').forEach(e=>e.onclick=ev=>{ev.stopPropagation();removeNode(e.dataset.remove)});
}
function nodeSummary(n){const c=n.config||{};if(n.type==='condition')return `${c.field||'campo'} ${c.operator||'='} ${c.value||''}`;if(n.type==='delay')return `Esperar ${c.seconds||0}s`;if(n.type==='send_whatsapp')return c.message||'Enviar mensaje';if(n.type==='tag_contact')return 'Etiqueta: '+(c.tag||'');if(n.type==='move_pipeline')return 'Etapa: '+(c.stage||'');if(n.type==='ai_task')return (wtAgentsV273547.find(a=>a.id===String(c.agent_id))?.name||'Selecciona agente')+' · '+(c.output_mode==='classification'?'Clasificación':'Análisis');if(n.type==='create_task')return 'Tarea: '+(c.title||'Sin título');if(n.type==='assign_owner')return 'Responsable: '+(wtSalesUsersV273545.find(u=>u.id===String(c.owner_user_id))?.name||'Por seleccionar');return n.type==='trigger'?($a('#wfTrigger')?.value||'manual'):'Configurable'}
function removeNode(id){
  const n=graph.nodes.find(x=>x.id===id);
  if(!n||n.type==='trigger')return;
  if(!confirm('¿Quitar «'+(n.label||nodeTypes[n.type])+'» del flujo? Se conservarán las demás ramas.'))return;
  const incoming=graph.edges.filter(e=>e.to===id),outgoing=graph.edges.filter(e=>e.from===id);
  graph.nodes=graph.nodes.filter(x=>x.id!==id);
  graph.edges=graph.edges.filter(e=>e.from!==id&&e.to!==id);
  if(outgoing.length===1){
    // Repair simple chains only; never flatten or overwrite a branched graph.
    for(const edge of incoming){
      const candidate={from:edge.from,to:outgoing[0].to,branch:edge.branch||'next'};
      if(candidate.from!==candidate.to&&!graph.edges.some(e=>e.from===candidate.from&&(e.branch||'next')===candidate.branch)){
        if(!automationHasCycleV273539([...graph.edges,candidate]))graph.edges.push(candidate);
      }
    }
  }
  if(selected===id)selected=null;
  renderCanvas();renderProps();
  if(outgoing.length!==1&&incoming.length)toast('Revisa las conexiones que apuntaban al bloque eliminado.');
}
function renderProps(){const box=$a('#nodeProperties'),n=graph.nodes.find(x=>x.id===selected);if(!n){box.innerHTML='<p>Selecciona un nodo del flujo.</p>';return}let html=`<label>Nombre<input id="nodeLabel" value="${safe(n.label||'')}"></label>`;const input=(k,l,v=n.config?.[k]||'')=>`<label>${l}<input data-cfg="${k}" value="${safe(v)}"></label>`;if(n.type==='condition')html+=`<label>Campo del evento<input data-cfg="field" list="wtAutoFieldsV273540" value="${safe(n.config?.field||'')}" placeholder="Ej. score"><datalist id="wtAutoFieldsV273540"><option value="score"><option value="name"><option value="phone"><option value="channel"><option value="text"><option value="stage"><option value="from_stage"><option value="platform"><option value="email"><option value="form_id"><option value="lead_id"><option value="interest"><option value="city"><option value="ai.intent"><option value="ai.priority"><option value="ai.summary"><option value="ai.reply"></datalist></label><p class="muted" style="font-size:12px">Elige un campo del evento o escribe uno personalizado. El valor debe existir en el disparador.</p>`+`<label>Operador<select data-cfg="operator"><option value="equals">Igual</option><option value="contains">Contiene</option><option value="not_empty">No vacío</option></select></label>`+input('value','Valor');if(n.type==='delay')html+=`<label>Tiempo de espera (segundos)<input data-cfg="seconds" type="number" min="0" max="2592000" step="1" value="${safe(n.config?.seconds??0)}"></label><p class="muted" style="font-size:12px">Hasta 30 días. Desde 11 segundos se programa una espera persistente; el servidor continúa el flujo al vencimiento.</p>`;if(n.type==='send_whatsapp')html+=renderWaCloudSelectV273540(n)+input('to','Teléfono fijo (vacío = número del evento)')+`<label>Mensaje<textarea data-cfg="message">${safe(n.config?.message||'')}</textarea></label>`+'<p class="muted" style="font-size:12px">Solo respuestas a mensajes WhatsApp Cloud verificados dentro de la ventana de atención (margen seguro de 23 horas). Los envíos automáticos están deshabilitados por defecto mientras se completa la verificación de permisos y bajas; las campañas requieren plantillas aprobadas.</p>';if(n.type==='tag_contact')html+=input('tag','Etiqueta');if(n.type==='move_pipeline')html+=opportunityStageSelectV273535('stage','Etapa del pipeline',n.config?.stage||'Contactado');if(n.type==='assign_owner')html+=renderSalesHubOwnerV273545(n,true);if(n.type==='ai_task')html+=renderAutomationAgentV273547(n)+`<label>Instrucción IA<textarea data-cfg="prompt" maxlength="2000">${safe(n.config?.prompt||'')}</textarea></label>`;if(n.type==='create_task')html+=automationTaskSettingsV273545(n);if(n.type==='webhook')html+=input('url','URL HTTPS');html+=automationRouteControlsV273539(n);
box.innerHTML=html;
box.querySelectorAll('#nodeLabel,[data-cfg],[data-stage-field]').forEach(x=>x.onchange=()=>{
  if(x.id==='nodeLabel')n.label=x.value;
  else {n.config=n.config||{};n.config[x.dataset.cfg||x.dataset.stageField]=x.value}
  renderCanvas();
});
if(n.type==='condition'&&box.querySelector('[data-cfg="operator"]'))box.querySelector('[data-cfg="operator"]').value=n.config?.operator||'equals';
box.querySelectorAll('[data-auto-route]').forEach(input=>input.addEventListener('change',()=>{
  const result=automationSetRouteV273539(n.id,input.dataset.autoRoute,input.value);
  if(!result.ok){toast(result.error);renderProps();return}
  renderCanvas();renderProps();
}));
const addNo=box.querySelector('#wtAutoAddNoBranch');
if(addNo)addNo.onclick=()=>{const type=box.querySelector('#wtAutoBranchNewType')?.value||'tag_contact';automationAddNodeV273539(type,n.id,'no')};
const refreshWa=box.querySelector('#wtReloadWaV273540');
if(refreshWa)refreshWa.onclick=()=>loadWaCloudV273540(true);
const refreshAgents=box.querySelector('#wtAutoReloadAgentsV273547');
if(refreshAgents)refreshAgents.onclick=()=>loadAutomationAgentsV273547(true);
if(n.type==='ai_task'&&!wtAgentsAttemptedV273547&&!wtAgentsLoadingV273547)void loadAutomationAgentsV273547();
const refreshUsers=box.querySelector('#wtReloadOwnersV273545');
if(refreshUsers)refreshUsers.onclick=()=>loadSalesHubUsersV273545(true);
if(['assign_owner','create_task'].includes(n.type)&&!wtSalesUsersAttemptedV273545&&!wtSalesUsersPromiseV273545)void loadSalesHubUsersV273545();
if(n.type==='send_whatsapp'&&!wtWaCloudAttemptedV273540&&!wtWaCloudPromiseV273540)void loadWaCloudV273540();}
// V27.35.69: Captacion IA cannot activate for all forms by accident.
function wtIsCaptacionGraphV273569(g){
  const ids=new Set((g?.nodes||[]).map(n=>n.id));
  return ['wt566_inicio','wt566_ai','wt566_prioridad','wt566_tarea'].every(id=>ids.has(id));
}
function wtCaptacionFormIssueV273569(g,cfg){
  if(!wtIsCaptacionGraphV273569(g))return '';
  const id=String(cfg?.form_id||'').trim();
  if(!/^[1-9]\d*$/.test(id))return 'Selecciona el formulario de origen antes de activar Captación IA.';
  if(!wtFormsLoadedV273544)return 'No se pudo validar el catálogo de formularios. Reintenta Activar o usa Actualizar formularios en el constructor.';
  if(!wtFormsV273544.some(x=>x.value===id))return 'El formulario seleccionado no está activo o ya no está disponible.';
  return '';
}
async function saveWorkflow(){if(!current) return toast('Crea o abre un flujo');current.name=$a('#wfName').value.trim();current.description=$a('#wfDesc').value.trim();current.status=$a('#wfStatus').value;current.trigger_type=$a('#wfTrigger').value;current.trigger_config=current.trigger_config||{};current.graph=graph;if(!current.name)return toast('Nombre obligatorio');if(current.status==='active'){if(wtIsCaptacionGraphV273569(graph))await loadFormsForAutomationsV273544(true);const issue=wtCaptacionFormIssueV273569(graph,current.trigger_config);if(issue)return toast(issue);if(graph.nodes.some(n=>n.type==='send_whatsapp'))await loadWaCloudV273540();if(graph.nodes.some(n=>['assign_owner','create_task'].includes(n.type)))await loadSalesHubUsersV273545(true);if(graph.nodes.some(n=>n.type==='ai_task'))await loadAutomationAgentsV273547(true);if(!wtShowActivationIssuesV273540(wtActivationIssuesV273540(graph)))return;}const saved=await api('/api/automations/workflows',{method:current.id?'PUT':'POST',body:JSON.stringify(current)});current=saved;toast('Workflow guardado');await loadWorkflowList()}
async function testWorkflow(){
  if(!current?.id){await saveWorkflow();if(!current?.id)return}
  // Preview endpoint is server-side dry run, with zero business side effects.
  const payload={name:'Lead de prueba',phone:'573001234567',score:'80',contact_id:'0',opportunity_id:'0',...(current.trigger_config||{})};
  try{
    const r=await api('/api/automations/run',{method:'POST',body:JSON.stringify({workflow_id:current.id,payload})});
    if(!r?.dry_run)throw new Error('El servidor no confirmó una simulación segura.');
    $a('#automationPreview')?.remove();
    const pane=document.createElement('section');pane.id='automationPreview';pane.setAttribute('role','status');
    pane.style.cssText='margin-top:14px;padding:16px;border:1px solid #c6b9fa;border-radius:16px;background:var(--card-bg,#fff);color:inherit;max-width:740px';
    const title=document.createElement('strong');title.textContent='Simulación segura · Sin envíos ni cambios en CRM';pane.appendChild(title);
    const summary=document.createElement('p');summary.textContent=`Disparador: ${r.trigger_matches?'coincide con el evento simulado':'NO coincide con el evento simulado'}. Recorrido previsto: ${(r.steps||[]).length} nodos. No se ejecutaron acciones reales.`;pane.appendChild(summary);
    const list=document.createElement('ol');
    for(const step of r.steps||[]){const li=document.createElement('li');li.textContent=`${step.label||step.node_id} (${step.type})${step.matched===false?' · NO':step.matched===true?' · SÍ':''}`;list.appendChild(li)}
    pane.appendChild(list);
    const host=$a('#automationCanvas');if(host?.parentElement)host.parentElement.appendChild(pane);
    toast('Simulación segura completada');
  }catch(e){toast(e.message||'No se pudo simular')}
}
async function loadExec(){
  // Health information is read-only; no replays or effects on external providers.
  void loadAutomationQueueHealthV273537();
  void loadAutomationAIUsageV273549();
  const xs=await api('/api/automations/executions');
  const el=$a('#automationHistory');
  el.innerHTML=xs.length?`<table class="execution-table"><tr><th>ID</th><th>Workflow</th><th>Evento</th><th>Estado</th><th>Inicio</th><th>Error</th></tr>${xs.map(x=>`<tr><td><button class="secondary mini" data-exec-id="${x.id}" title="Ver pasos">#${x.id}</button></td><td>${x.workflow_id}</td><td>${safe(x.event_type)}</td><td><span class="wf-status ${x.status==='success'?'active':''}">${safe(x.status)}</span></td><td>${safe(x.started_at)}</td><td>${safe(x.error_message)}</td></tr>`).join('')}</table>`:'<p>No hay ejecuciones todavía.</p>';
  el.querySelectorAll('[data-exec-id]').forEach(btn=>btn.addEventListener('click',async()=>{
    try{
      const data=await api('/api/automations/executions?execution_id='+encodeURIComponent(btn.dataset.execId));
      el.querySelector('#automationExecDetails')?.remove();
      const pane=document.createElement('section');pane.id='automationExecDetails';pane.style.cssText='margin-top:14px;padding:16px;border:1px solid #c6b9fa;border-radius:16px;background:var(--card-bg,#fff);color:inherit';
      const heading=document.createElement('strong');heading.textContent='Detalle de ejecución #'+btn.dataset.execId;pane.appendChild(heading);
      const list=document.createElement('ol');
      for(const step of data.steps||[]){const item=document.createElement('li');item.textContent=`${step.node_type} (${step.node_id}) · ${step.status}${step.error_message?' · '+step.error_message:''}`;list.appendChild(item)}
      if(!list.childElementCount){const empty=document.createElement('p');empty.textContent='Esta ejecución no tiene pasos registrados.';pane.appendChild(empty)}else pane.appendChild(list);
      el.appendChild(pane);
    }catch(e){toast(e.message||'No se pudieron leer los pasos')}
  }));
}


async function loadAutomationAIUsageV273549(){
  const history=$a('#automationHistory');
  if(!history)return;
  let panel=$a('#wtAutomationAIUsageV273549');
  if(!panel){
    panel=document.createElement('section');panel.id='wtAutomationAIUsageV273549';
    panel.setAttribute('aria-label','Uso de IA en automatizaciones');
    panel.style.cssText='border:1px solid #dcdcf4;border-radius:16px;padding:15px 16px;margin:0 0 18px;background:#fff;color:#151b39';
    history.insertAdjacentElement('beforebegin',panel);
  }
  panel.replaceChildren();
  const loading=document.createElement('p');loading.textContent='Consultando acciones IA…';panel.appendChild(loading);
  try{
    const u=await api('/api/automations/executions?ai_usage=1');
    panel.replaceChildren();
    const heading=document.createElement('strong');heading.textContent='Actividad IA de Automatizaciones · '+String(u.period||'mes actual');panel.appendChild(heading);
    const grid=document.createElement('div');grid.style.cssText='display:grid;grid-template-columns:repeat(auto-fit,minmax(125px,1fr));gap:9px;margin:12px 0';
    for(const [label,num] of [['Acciones IA registradas',u.total_actions],['Completadas',u.successful_actions],['Fallidas',u.failed_actions],['Máximo de nodos IA por flujo',u.max_ai_nodes_per_workflow]]){
      const card=document.createElement('div');card.style.cssText='border:1px solid #e6e8f4;border-radius:12px;padding:10px 12px';
      const name=document.createElement('small');name.textContent=label;name.style.cssText='display:block;color:#66718e;margin-bottom:4px';
      const value=document.createElement('strong');value.textContent=String(num??0);value.style.fontSize='17px';
      card.append(name,value);grid.appendChild(card);
    }
    panel.appendChild(grid);
    const note=document.createElement('p');note.style.cssText='font-size:12px;opacity:.78;margin:0';
    note.textContent='Estas cifras son pasos de IA registrados, no tokens ni dinero facturado. El consumo y sus costos siguen controlados por el medidor comercial de Worktic AI. No se realizan llamadas IA al consultar este panel.';
    panel.appendChild(note);
  }catch(e){
    panel.replaceChildren();const note=document.createElement('p');
    note.textContent='No se pudo consultar la actividad IA. '+String(e?.message||'');panel.appendChild(note);
  }
}

async function loadAutomationQueueHealthV273537(){
  const history=$a('#automationHistory');
  if(!history)return;
  let panel=$a('#wtAutomationQueueHealthV273537');
  if(!panel){
    panel=document.createElement('section');
    panel.id='wtAutomationQueueHealthV273537';
    panel.setAttribute('aria-label','Salud del motor de automatizaciones');
    panel.style.cssText='border:1px solid #dcdcf4;border-radius:16px;padding:15px 16px;margin:0 0 18px;background:#fff;color:#151b39';
    history.insertAdjacentElement('beforebegin',panel);
  }
  panel.replaceChildren();
  const loading=document.createElement('p');loading.textContent='Comprobando la salud de la cola…';panel.appendChild(loading);
  try{
    const h=await api('/api/automations/executions?health=1');
    panel.replaceChildren();
    const title=document.createElement('strong');title.textContent='Salud de las automatizaciones';panel.appendChild(title);
    const note=document.createElement('p');
    note.style.cssText='font-size:12px;opacity:.7;margin:5px 0 12px';
    note.textContent='Diagnóstico de solo lectura · únicamente datos de tu empresa · no reenvía ni reintenta acciones';
    panel.appendChild(note);
    const grid=document.createElement('div');grid.style.cssText='display:grid;grid-template-columns:repeat(auto-fit,minmax(135px,1fr));gap:9px';
    const items=[
      ['Motor',h.worker_enabled?'Habilitado':'Deshabilitado'],
      ['Eventos pendientes',h.pending],
      ['En procesamiento',h.processing],
      ['Fallidos',h.failed],
      ['Eventos a revisar',h.needs_review],
      ['Leads por transferir',h.lead_pending],
      ['Leads a revisar',h.lead_needs_review],
      ['Pendientes (+30 min)',h.pending_older_30m],
      ['Procesando (+30 min)',h.processing_older_30m],
      ['Ejecuciones abiertas (+30 min)',h.running_older_30m],
      ['Esperas programadas',h.scheduled_waits],
      ['Esperas vencidas',h.overdue_waits],
      ['Esperas a revisar',h.waits_need_review]
    ];
    for(const [name,value] of items){
      const cell=document.createElement('div');cell.style.cssText='border:1px solid #e6e8f4;border-radius:12px;padding:10px 12px';
      const cap=document.createElement('small');cap.textContent=name;cap.style.cssText='display:block;color:#66718e;margin-bottom:4px';
      const val=document.createElement('strong');val.textContent=String(value??0);val.style.fontSize='17px';
      cell.append(cap,val);grid.appendChild(cell);
    }
    panel.appendChild(grid);
    if(!h.worker_enabled||Number(h.processing_older_30m)>0||Number(h.running_older_30m)>0||Number(h.failed)>0||Number(h.needs_review)>0||Number(h.lead_needs_review)>0||Number(h.waits_need_review)>0){
      const alert=document.createElement('p');alert.style.cssText='margin:12px 0 0;font-size:12px;color:#9b5315';
      alert.textContent=!h.worker_enabled?'El worker no está habilitado: los eventos se acumularán hasta que se active.'
        : 'Hay eventos fallidos o procesos antiguos. Requieren revisión; no se reintentan automáticamente para evitar mensajes duplicados.';
      panel.appendChild(alert);
    }
  }catch(e){
    panel.replaceChildren();
    const err=document.createElement('p');
    err.textContent='No fue posible consultar la salud del motor. Las ejecuciones siguen disponibles. '+String(e?.message||'');
    panel.appendChild(err);
  }
}

function applyTemplate(t){newWorkflow();if(t==='captacion_ia'){
  const draft=wtCaptacionDraftV273566();
  current={name:draft.name,description:draft.description,status:'draft',trigger_type:draft.trigger_type,trigger_config:draft.trigger_config};
  graph=draft.graph; selected='wt566_ai';
  $a('#wfName').value=current.name;
  $a('#wfDesc').value=current.description;
  $a('#wfStatus').value='draft';
  $a('#wfTrigger').value='form.submitted';
  renderTriggerSettingsV273535();renderCanvas();renderProps();switchTab('builder');
  return;
}
if(t==='lead'){current.name='Lead Ads → CRM → WhatsApp';current.trigger_type='lead.created';graph.nodes=[{id:'t1',type:'trigger',label:'Lead nuevo',config:{}},{id:'t2',type:'condition',label:'Lead calificado',config:{field:'score',operator:'not_empty',value:''}},{id:'t3',type:'tag_contact',label:'Etiquetar lead',config:{tag:'meta-lead'}},{id:'t4',type:'send_whatsapp',label:'Seguimiento WhatsApp',config:{connection_id:'',to:'',message:'Hola {{name}}, gracias por tu interés. ¿Cómo podemos ayudarte?'}}]}
if(t==='inbox'){current.name='Mensaje entrante → IA → Pipeline';current.trigger_type='message.received';graph.nodes=[{id:'t1',type:'trigger',label:'Mensaje recibido',config:{}},{id:'t2',type:'ai_task',label:'Analizar intención',config:{prompt:'Clasifica intención e interés comercial.'}},{id:'t3',type:'move_pipeline',label:'Mover a Contactado',config:{stage:'Contactado'}}]}
if(t==='appointment'){current.name='Cita creada → seguimiento';current.trigger_type='appointment.created';graph.nodes=[{id:'t1',type:'trigger',label:'Cita creada',config:{}},{id:'t2',type:'delay',label:'Espera controlada',config:{seconds:2}},{id:'t3',type:'tag_contact',label:'Etiquetar cita',config:{tag:'cita-programada'}}]}
graph.edges=[];for(let i=1;i<graph.nodes.length;i++)graph.edges.push({from:graph.nodes[i-1].id,to:graph.nodes[i].id,branch:'next'});$a('#wfName').value=current.name;$a('#wfTrigger').value=current.trigger_type;renderTriggerSettingsV273535();renderCanvas();renderProps();switchTab('builder')}
})();
