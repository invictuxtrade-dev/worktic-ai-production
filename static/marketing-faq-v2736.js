/* V27.34.2: Static SEO FAQ enhancement, no billing or server changes. */
(()=>{'use strict';
const root=document.getElementById('wts36FaqList');if(!root)return;
const items=Array.from(root.querySelectorAll('.wts36-faq-item'));
const search=document.getElementById('wts36FaqSearch');
const count=document.getElementById('wts36FaqCount');
const empty=document.getElementById('wts36FaqEmpty');
const chips=Array.from(document.querySelectorAll('[data-faq-filter]'));
let selected='all';
const english=()=>document.documentElement.lang==='en';
const normalize=s=>String(s||'').normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLocaleLowerCase().trim();
const refresh=()=>{
 const term=normalize(search?.value);
 let visible=0;
 for(const item of items){
   const eligible=selected==='all'||item.dataset.faqCategory===selected;
   const txt=item.querySelector(english()?'.wts36-en':'.wts36-es');
   const answer=item.querySelector(english()?'.wts36-answer .wts36-en':'.wts36-answer .wts36-es');
   const found=!term||normalize((txt?.textContent||'')+' '+(answer?.textContent||'')).includes(term);
   item.hidden=!(eligible&&found);
   if(!item.hidden)visible++; else item.open=false;
 }
 if(count)count.textContent=english()?`${visible} ${visible===1?'question':'questions'}`:`${visible} ${visible===1?'pregunta':'preguntas'}`;
 if(empty)empty.hidden=visible!==0;
 if(search){search.placeholder=english()?'Search questions...':'Buscar una pregunta...';search.setAttribute('aria-label',english()?'Search frequently asked questions':'Buscar en preguntas frecuentes');}
};
chips.forEach(btn=>btn.addEventListener('click',()=>{selected=btn.dataset.faqFilter;for(const chip of chips){const active=chip===btn;chip.classList.toggle('is-active',active);chip.setAttribute('aria-pressed',String(active));}refresh()}));
search?.addEventListener('input',refresh);
window.addEventListener('worktic:langchange',refresh);
const langObserver=new MutationObserver(refresh);langObserver.observe(document.documentElement,{attributes:true,attributeFilter:['lang']});
refresh();
})();
