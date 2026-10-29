/* Worktic AI V27.35.20 - Paginated data views, not interactive timelines/calendar. */
(()=>{
'use strict';
const DEFAULT_SIZE=10;
const DATA_HOSTS=[
'contactsTable','productsGrid','appointmentsTable','agendaHoursTable','agendaProfessionalsTable','agendaServicesTable',
'agentsGrid','agentKnowledgeList','agentMemoryList','agentRoutesTable','agentMetricsTable','agentPermissionsTable',
'waContactsTable','landingsGrid','managedGroupsGrid','groupProspectsTable','workflowList','rulesList',
'socialPostsTable','socialConnectionsGrid','ads25CampaignTable','ads25AttributionTable',
'campaignsGrid','creativeGrid','leadFormsGrid','usersTable',
'channelConnections','ww36List','sh18Main','v23MediaGrid','v23ApprovalsList','v23CommentsList'
];
const CARD_HOSTS=new Set(['productsGrid','agentsGrid','agentKnowledgeList','agentMemoryList','landingsGrid','managedGroupsGrid','workflowList','rulesList','socialConnectionsGrid','campaignsGrid','creativeGrid','leadFormsGrid','channelConnections','ww36List','v23MediaGrid','v23ApprovalsList','v23CommentsList']);
const SKIP_EXISTING=new Set(['wtAdminUsersTable','wtAdminPlansTable','wtAdminPaymentsTable','wtAdminLicensesTable','wtAdminPromosTable']);
const states=new Map();
let scheduled=false;
const $=id=>document.getElementById(id);
function format(n){return new Intl.NumberFormat(document.documentElement.lang==='en'?'en':'es').format(n);}
function ensureBar(root,id){
  let bar=$('wtpg-'+id);
  if(bar&&bar.previousElementSibling!==root){bar.remove();bar=null;}
  if(!bar){bar=document.createElement('nav');bar.id='wtpg-'+id;bar.className='wtpg-bar';bar.setAttribute('aria-label','Paginación de '+id);
    bar.innerHTML='<span class="wtpg-meta" aria-live="polite"></span><div class="wtpg-controls"><label>Mostrar <select aria-label="Registros por página"><option value="10">10</option><option value="20">20</option><option value="50">50</option><option value="100">100</option></select></label><button type="button" data-wtpg="prev" aria-label="Página anterior">‹</button><span class="wtpg-page" aria-live="polite"></span><button type="button" data-wtpg="next" aria-label="Página siguiente">›</button></div>';
    root.insertAdjacentElement('afterend',bar);
    const state=states.get(id);
    bar.querySelector('select').value=String(state.size);
    bar.querySelector('select').addEventListener('change',e=>{state.size=Number(e.target.value)||DEFAULT_SIZE;state.page=1;render(id);});
    bar.addEventListener('click',e=>{const btn=e.target.closest('[data-wtpg]');if(!btn)return;const pages=Math.max(1,Math.ceil(state.total/state.size));state.page=Math.max(1,Math.min(pages,state.page+(btn.dataset.wtpg==='next'?1:-1)));render(id);});
  }
  return bar;
}
function rowsFor(host,id){
  const table=host.matches('table')?host:host.querySelector('table');
  if(table){
    const body=table.tBodies?.[0];if(!body)return [];
    // Do not paginate a placeholder row with colspan: empty state remains intact.
    return Array.from(body.rows).filter(row=>!row.querySelector('th')&&!(body.rows.length===1&&row.querySelector('[colspan]')));
  }
  if(id==='sh18Main')return Array.from(host.querySelectorAll('.sh18-follow-row'));
  if(!CARD_HOSTS.has(id))return [];
  const children=Array.from(host.children);
  return children.filter(el=>!['SCRIPT','STYLE'].includes(el.tagName)&&!el.matches('.catalog-empty-state,.v22-empty,.wt-admin-empty,.empty,.no-data,[data-wtpg-skip]')&&
    (el.matches('article,.card,.conv,.v22-item,.v22-list-item,.workflow-card,.social-card,.channel-card,.ww36-channel,.ww36-card,.agent-card,.landing-card,.product-card')||
    ['productsGrid','agentsGrid','landingsGrid','managedGroupsGrid','workflowList','campaignsGrid','creativeGrid','leadFormsGrid','channelConnections','ww36List','v23MediaGrid','v23ApprovalsList','v23CommentsList','agentKnowledgeList','agentMemoryList','socialConnectionsGrid'].includes(id)));
}
function render(id){
  const host=$(id),state=states.get(id);if(!host||!state)return;
  const rows=rowsFor(host,id),length=rows.length;
  let page=Math.min(Math.max(1,state.page),Math.max(1,Math.ceil(length/state.size)));
  state.page=page;state.total=length;
  const start=(page-1)*state.size;
  for(let i=0;i<length;i++)rows[i].classList.toggle('wtpg-hidden',i<start||i>=start+state.size);
  const bar=ensureBar(host,id),end=Math.min(length,start+state.size),pages=Math.max(1,Math.ceil(length/state.size));
  bar.hidden=length===0;
  bar.querySelector('.wtpg-meta').textContent=length?'Mostrando '+format(start+1)+'–'+format(end)+' de '+format(length)+' registros':'Sin registros';
  bar.querySelector('.wtpg-page').textContent='Página '+page+' de '+pages;
  bar.querySelector('[data-wtpg="prev"]').disabled=page<=1;
  bar.querySelector('[data-wtpg="next"]').disabled=page>=pages;
}
function refresh(){
  scheduled=false;
  for(const id of DATA_HOSTS){
    const host=$(id);if(!host)continue;
    const rows=rowsFor(host,id),length=rows.length;
    let state=states.get(id);
    if(!state){state={page:1,size:DEFAULT_SIZE,total:0,head:null};states.set(id,state);}
    // Dynamic renderer replaced content: go back to page 1 to show latest filtered data.
    const head=rows[0]||null;
    if(state.total!==length||state.head!==head){state.page=1;state.head=head;render(id);}
    else if(length>0&&!$('wtpg-'+id))render(id);
  }
}
function schedule(){if(scheduled)return;scheduled=true;setTimeout(refresh,80);}
function init(){
  refresh();
  const watcher=new MutationObserver(mutations=>{
    for(const m of mutations){if(m.type!=='childList')continue;
      const target=m.target;
      if(target?.closest?.('.wtpg-bar'))continue;
      if(m.addedNodes?.length||m.removedNodes?.length){schedule();break;}
    }
  });
  watcher.observe(document.body,{subtree:true,childList:true});
  document.addEventListener('wtpg-refresh',schedule);
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init,{once:true});else init();
})();
