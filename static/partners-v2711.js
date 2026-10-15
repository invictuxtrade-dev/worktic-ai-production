(function(){
'use strict';
var lang='es';
var switcher=document.getElementById('partnerLang');
function applyLang(){
  document.documentElement.lang=lang;
  document.querySelectorAll('[data-es][data-en]').forEach(function(el){el.textContent=el.getAttribute(lang==='es'?'data-es':'data-en')});
  if(switcher)switcher.textContent=lang==='es'?'EN':'ES';
}
if(switcher)switcher.addEventListener('click',function(){lang=lang==='es'?'en':'es';applyLang()});
applyLang();

var ranks=[
 {name:'Spark',direct:.25,n2:0,n3:0,minDirect:0,minAdopt:0},
 {name:'Pulse',direct:.28,n2:0,n3:0,minDirect:3,minAdopt:2},
 {name:'Orbit',direct:.30,n2:.03,n3:0,minDirect:8,minAdopt:6},
 {name:'Apex',direct:.32,n2:.04,n3:.02,minDirect:18,minAdopt:14},
 {name:'Infinity',direct:.34,n2:.05,n3:.03,minDirect:35,minAdopt:28}
];
var prices={personal:{m:25,y:240},business:{m:75,y:720},enterprise:{m:150,y:1440}};
function num(id){return Math.max(0,Number(document.getElementById(id).value||0))}
function money(v){return new Intl.NumberFormat(lang==='es'?'es-CO':'en-US',{style:'currency',currency:'USD',maximumFractionDigits:2}).format(v)}
function rankFor(direct,adopt,retention){var r=ranks[0];for(var i=0;i<ranks.length;i++){var x=ranks[i];if(direct>=x.minDirect&&adopt>=x.minAdopt)r=x}if(!retention&&ranks.indexOf(r)>=2)r=ranks[1];return r}
function calc(){
  var plan=document.getElementById('simPlan').value,cycle=document.getElementById('simCycle').value,directCount=num('simDirect'),adoptCount=Math.min(directCount,num('simAdopt')),renewCount=Math.min(directCount,num('simRenew')),n2Volume=num('simN2'),n3Volume=num('simN3'),mentorVolume=num('simMentor'),basePct=Math.min(100,num('simBase'))/100,retention=document.getElementById('simRetention').checked,cosell=document.getElementById('simCosell').value;
  var rank=rankFor(directCount,adoptCount,retention),unit=cycle==='annual'?prices[plan].y/12:prices[plan].m,base=unit*directCount*basePct;
  var directPool=base*rank.direct,direct=directPool;
  if(cosell==='referrer')direct=directPool*.70; else if(cosell==='closer')direct=directPool*.30;
  var adoption=unit*adoptCount*basePct*.04,renew=unit*renewCount*basePct*.02,n2=n2Volume*basePct*rank.n2,n3=n3Volume*basePct*rank.n3,mentor=mentorVolume*basePct*.02,total=direct+adoption+renew+n2+n3+mentor;
  document.getElementById('outRank').textContent=rank.name;
  document.getElementById('outDirectRate').textContent=Math.round(rank.direct*100)+'%';
  document.getElementById('outMonthly').textContent=money(total);
  document.getElementById('outAnnual').textContent=money(total*12);
  document.getElementById('outDirect').textContent=money(direct);
  document.getElementById('outAdoption').textContent=money(adoption);
  document.getElementById('outRenew').textContent=money(renew);
  document.getElementById('outN2').textContent=money(n2);
  document.getElementById('outN3').textContent=money(n3);
  document.getElementById('outMentor').textContent=money(mentor);
  document.getElementById('outUnit').textContent=money(unit);
}
document.querySelectorAll('#partnerSimulator input,#partnerSimulator select').forEach(function(el){el.addEventListener('input',calc);el.addEventListener('change',calc)});calc();
})();
