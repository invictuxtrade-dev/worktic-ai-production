/* V27.34.7: navigate wallet independently; no financial rules or fake transactions. */
(()=>{'use strict';
 function activate(view){const btn=document.querySelector('nav button[data-view="'+view+'"]');if(btn)btn.click()}
 function init(){
  document.getElementById('wcc47GoEditor')?.addEventListener('click',()=>activate('studio'));
  document.getElementById('wcc47OpenFromStudio')?.addEventListener('click',()=>{if(window.WorkticWalletV27346?.open){window.WorkticWalletV27346.open('overview')}else activate('studio-credits')});
  document.addEventListener('click',event=>{const b=event.target.closest('nav button[data-view="studio-credits"]');if(b){window.setTimeout(()=>window.WorkticWalletV27346?.refresh(),90)}});
  // Do not allow the old panels to remain below Studio if a different script initializes them later.
  const movePanels=()=>{const billing=document.getElementById('billing');if(!billing)return;const nodes=['wcmFeaturesV2722','wrqV2724'].map(id=>document.getElementById(id)).filter(Boolean);if(!nodes.length)return;
    let details=document.getElementById('wcc47PlanDiagnostics');if(!details){details=document.createElement('details');details.id='wcc47PlanDiagnostics';details.className='wcc47-diagnostics';const summary=document.createElement('summary');summary.textContent='Ver detalle técnico de funcionalidades y cuotas';details.append(summary);billing.append(details)}for(const node of nodes)if(node.parentElement!==details)details.append(node)};
  movePanels();
 }
 if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
})();
