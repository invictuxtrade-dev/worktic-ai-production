/* V27.19: larger video preview and native AI video generation. Scoped to Studio. */
(()=>{'use strict';
const $=id=>document.getElementById(id), api=()=>window.WorkticStudioV2718;
let lastJob='',polling=false,active=false,pollCount=0,pollTimer=null;
const state=msg=>{if($('wtsVideoGenStatus'))$('wtsVideoGenStatus').textContent=msg};
const ratio=()=>$('wtsVideoFormat').value;
function currentProject(){const a=api();if(a?.project?.kind!=='video')a?.modeChange('video');return a?.project?.kind==='video'?a.project:null}
function syncMode(){const root=$('studio')?.querySelector('.wts-app'),a=api();if(!root||!a)return;
 const video=a.project?.kind==='video';root.classList.toggle('wts-video-mode',video);
 if(video){$('wtsVideoFormat').value=a.project.ratio||'9:16'}
 if($('wtsTimelineToggle'))$('wtsTimelineToggle').disabled=!video;
}
function syncImages(){const a=api(),select=$('wtsVideoImage');if(!select||!a)return;const previous=select.value;select.replaceChildren(new Option('Selecciona una imagen…',''));
 for(const asset of a.assets.filter(v=>v.media_type==='image'))select.add(new Option((asset.name||'Imagen').slice(0,60),asset.url));
 if([...select.options].some(v=>v.value===previous))select.value=previous;
}
function gradientScene(text,i,aspect){const c=document.createElement('canvas');let dims=({'9:16':[540,960],'16:9':[960,540],'1:1':[720,720],'4:5':[576,720]})[aspect]||[540,960];[c.width,c.height]=dims;
 let g=c.getContext('2d'),palette=[['#102947','#3c1871'],['#113a42','#175f6a'],['#3c1749','#9a3b62'],['#132939','#283b92'],['#2b2254','#087c77']][i%5];let grad=g.createLinearGradient(0,0,c.width,c.height);grad.addColorStop(0,palette[0]);grad.addColorStop(1,palette[1]);g.fillStyle=grad;g.fillRect(0,0,c.width,c.height);
 g.fillStyle='rgba(255,255,255,.045)';for(let j=0;j<8;j++){g.beginPath();g.arc(c.width*((j*71%100)/100),c.height*((j*41%100)/100),c.width*(.05+j*.015),0,Math.PI*2);g.fill()}
 g.font=`bold ${Math.round(c.width/33)}px system-ui`;g.fillStyle='rgba(255,255,255,.67)';g.fillText('WORKTIC STUDIO',c.width*.07,c.height*.08);return c.toDataURL('image/webp',.76)
}
function headlineLines(raw,aspect){const max=aspect==='16:9'?31:aspect==='1:1'?24:19;let lines=[],current='';for(const word of String(raw).split(/\s+/)){if(lines.length>=3)break;if((current+' '+word).trim().length>max&&current){lines.push(current);current=word}else{current=(current+' '+word).trim()}}if(current&&lines.length<3)lines.push(current);return lines.join('\n')}
function makeLocalVideo(){const a=api(),p=currentProject();if(!p){state('Abre Video Studio y guarda primero el proyecto actual si tienes cambios sin guardar.');return}
 let prompt=$('wtsVideoPrompt').value.trim();if(prompt.length<8){state('Describe el anuncio, producto o historia en al menos 8 caracteres.');return}
 let images=a.assets.filter(v=>v.media_type==='image');const selected=$('wtsVideoImage').value;if(selected){const image=images.find(v=>v.url===selected);if(image)images=[image,...images.filter(v=>v.url!==selected)]}
 const scenes=prompt.split(/(?:[.!?]\s+|\n+)/).map(s=>s.trim()).filter(Boolean).slice(0,6);
 if(scenes.length===1){scenes.push('Conoce la experiencia','Descúbrelo hoy')} const len=Math.min(6,Math.max(3,scenes.length));
 if(p.clips.length&&!window.confirm('¿Reemplazar la pista actual de video? Se conservarán tus otras pistas.'))return;
 let aspect=ratio();p.kind='video';p.ratio=aspect;p.clips=[];p.texts=[];let time=0;
 for(let i=0;i<len;i++){const photo=images.length?images[i%images.length]:null,dur=3.3,textSpan=(scenes[i]||'Conoce más').slice(0,92);
 p.clips.push({id:a.uid(),kind:'image',url:photo?photo.url:gradientScene(textSpan,i,aspect),name:photo?.name||`Escena ${i+1} · Animada`,duration:dur,trim:0,zoom:.12});
 p.texts.push({id:a.uid(),type:'text',text:headlineLines(textSpan,aspect),x:.09,y:.39,size:aspect==='9:16'?48:56,color:'#ffffff',start:time+.12,duration:dur-.24,opacity:1});time+=dur;
 }
 p.duration=time;p.transition='fade';p.name='Video · '+prompt.slice(0,48);a.playhead=.55;a.selected=null;a.mark();a.renderEverything();syncMode();state(`✓ Proyecto de ${Math.round(time)} s creado con ${len} escenas animadas. Edita las pistas, añade voz o música y pulsa Exportar para guardar el WebM en tu Galería.`);
 a.say('Video animado construido en la línea de tiempo. Exporta para obtener el archivo real.');
}
async function startNative(){const a=api(),p=currentProject();if(!p){state('Primero abre Video Studio.');return}const prompt=$('wtsVideoPrompt').value.trim();if(prompt.length<10){state('El prompt IA necesita al menos 10 caracteres.');return}
 const image=$('wtsVideoSource').value==='image'?$('wtsVideoImage').value:'';
 if($('wtsVideoSource').value==='image'&&!image){state('Selecciona una imagen de tu Galería.');return}
 let confirmText=`Generar un video IA de ${$('wtsVideoDuration').value} s con Luma Agents puede consumir créditos de pago. ¿Autorizar esta generación?`;
 if(window.WorkticCostConfirm){if(!(await window.WorkticCostConfirm('video',{duration:Number($('wtsVideoDuration').value)})))return;}else if(!window.confirm(confirmText))return;
 const btn=$('wtsNativeVideoGenerate');btn.disabled=true;state('Enviando solicitud de video al proveedor…');try{
 const result=await a.request('/api/studio/video/generate',{method:'POST',body:JSON.stringify({prompt,ratio:ratio(),duration:Number($('wtsVideoDuration').value),image_url:image})});lastJob=result.id;pollCount=0;
 try{sessionStorage.setItem('workticStudioVideoJob',lastJob)}catch{}
 $('wtsResumeVideoJob').hidden=false;state(`Solicitud aceptada · ${lastJob.slice(0,8)}.\nEstado: ${result.state}. Puedes seguir editando mientras se genera.`);
 pollNative();
 }catch(e){state('No se pudo solicitar el video IA: '+e.message)}finally{btn.disabled=false}
}
async function pollNative(){if(!lastJob||polling)return;polling=true;try{
 const response=await api().request('/api/studio/video/generate?id='+encodeURIComponent(lastJob));
 if(response.state==='completed'&&response.url){try{sessionStorage.removeItem('workticStudioVideoJob')}catch{}
 state('✓ MP4 generado y guardado en tu Galería. Añadiéndolo a la línea de tiempo…');await api().refreshAssets();const found=api().assets.find(v=>v.url===response.url);api().addAsset(found||{url:response.url,media_type:'video',name:'Video IA · '+lastJob.slice(0,8)});api().say('Video IA MP4 añadido a la línea de tiempo');lastJob='';$('wtsResumeVideoJob').hidden=true;
 }else if(response.state==='failed'){state('La generación del proveedor no pudo completarse. Consulta sus créditos y vuelve a intentarlo.');lastJob='';$('wtsResumeVideoJob').hidden=true}
 else {state(`Video IA en proceso (${response.state==='processing'?'generando fotogramas':'en cola'}). ID ${lastJob.slice(0,8)}.\nPuedes continuar editando.`);}
 }catch(e){state('No se pudo consultar el estado: '+e.message+' — usa «Consultar generación pendiente».')}
 finally{polling=false;if(lastJob&&pollCount++<80){clearTimeout(pollTimer);pollTimer=setTimeout(()=>{if(document.visibilityState==='visible')pollNative();else{pollTimer=setTimeout(pollNative,8000)}},8000)}}
}
function init(){if(active||!$('wtsVideoPrompt')||!api())return;active=true;
 const app=$('studio').querySelector('.wts-app');const modes=$('studio').querySelector('.wts-modes');
 const observer=new MutationObserver(syncMode);observer.observe(modes,{subtree:true,attributes:true,attributeFilter:['class']});syncMode();
 $('wtsVideoSource').onchange=()=>{$('wtsSourceImageRow').hidden=$('wtsVideoSource').value!=='image';syncImages()};
 $('wtsVideoFormat').onchange=()=>{let a=api(),p=a?.project;if(p?.kind==='video'){$('wtsRatio').value=ratio();$('wtsRatio').dispatchEvent(new Event('change'));a.draw()}};
 $('wtsVideoSource').onchange();$('wtsVideoFormat').value=api().project?.ratio||'9:16';
 $('wtsPreviewScale').oninput=e=>app.style.setProperty('--wts-preview-scale',String(Number(e.target.value)/100));
 $('wtsTimelineToggle').onclick=()=>{app.classList.toggle('wts-timeline-closed');$('wtsTimelineToggle').textContent=app.classList.contains('wts-timeline-closed')?'▣ Mostrar timeline':'▤ Ocultar timeline';api()?.draw()};
 $('wtsLocalVideoGenerate').onclick=makeLocalVideo;$('wtsNativeVideoGenerate').onclick=startNative;$('wtsResumeVideoJob').onclick=pollNative;
 $('wtsPanelTabs').addEventListener('click',e=>{if(e.target.closest('[data-panel="wtsVideoGeneratorTab"]'))syncImages()});
 let cached='';try{cached=sessionStorage.getItem('workticStudioVideoJob')||''}catch{}
 if(/^[a-f\d-]{36}$/i.test(cached)){lastJob=cached;$('wtsResumeVideoJob').hidden=false;state('Hay una generación IA pendiente. Pulsa «Consultar generación pendiente» para recuperar su estado.')}
 fetch('/api/studio/video/generate?status=config',{credentials:'same-origin'}).then(r=>r.ok?r.json():null).then(v=>{if(v&&!v.available&&!lastJob)state('Generación nativa disponible al configurar LUMA_AGENTS_API_KEY en Render. Puedes crear videos animados locales ahora mismo.');}).catch(()=>{});
}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init,{once:true});else init();
})();
