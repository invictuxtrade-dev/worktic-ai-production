const checkoutParams=new URLSearchParams(location.search);
const requestedPlan=(checkoutParams.get('plan')||'').toLowerCase();
const requestedBilling=checkoutParams.get('billing')==='annual'?'annual':'monthly';
const requestedTarget=requestedPlan&&requestedPlan!=='free'?`/app.html?view=billing&plan=${encodeURIComponent(requestedPlan)}&billing=${requestedBilling}`:'/app.html';
const $=s=>document.querySelector(s);const api=async(u,o={})=>{const r=await fetch(u,{credentials:'same-origin',headers:{'Content-Type':'application/json'},...o});const d=await r.json().catch(()=>({}));if(!r.ok)throw Error(d.error||'Error');return d};
$('#loginTab').onclick=()=>{$('#loginForm').hidden=false;$('#registerForm').hidden=true;$('#loginTab').classList.add('active');$('#registerTab').classList.remove('active')};$('#registerTab').onclick=()=>{$('#loginForm').hidden=true;$('#registerForm').hidden=false;$('#registerTab').classList.add('active');$('#loginTab').classList.remove('active')};
$('#loginForm').onsubmit=async e=>{e.preventDefault();try{await api('/api/auth/login',{method:'POST',body:JSON.stringify(Object.fromEntries(new FormData(e.target)))});location.href=requestedTarget}catch(x){$('#authError').textContent=x.message}};
$('#registerForm').onsubmit=async e=>{e.preventDefault();try{await api('/api/auth/register',{method:'POST',body:JSON.stringify(Object.fromEntries(new FormData(e.target)))});location.href=requestedTarget}catch(x){$('#authError').textContent=x.message}};
fetch('/api/auth/me',{credentials:'same-origin'}).then(r=>{if(r.ok)location.replace(requestedTarget)}).catch(()=>{});
const accountType=document.querySelector('#accountType');if(accountType)accountType.onchange=()=>{const lbl=document.querySelector('#companyLabel').querySelector('span');if(lbl){const key=accountType.value==='personal'?'auth.company_label_personal':'auth.company_label_company';lbl.setAttribute('data-i18n',key);lbl.textContent=(window.wtT&&window.wtT(key))||(accountType.value==='personal'?'Nombre de tu espacio (opcional)':'Nombre de la empresa')}};

const requestedTab=new URLSearchParams(location.search).get('tab');if(requestedTab==='register')document.querySelector('#registerTab').click();else document.querySelector('#loginTab').click();
