(() => {
  if (typeof openLandingEditor !== 'function' || typeof modal !== 'function') return;

  const premiumDefaults = {
    brand_name: '', media_type: 'image', video_url: '', video_placement: 'section', hero_alt: '', hero_fit: 'cover', hero_layout: 'split',
    contact_title: 'Hablemos de tu proyecto', contact_subtitle: 'Elige el canal que prefieras y recibe atención directa.', contact_message: '',
    channel_keys: [], stats: [], trust_text: 'Atención rápida, segura y personalizada', footer_text: '',
    show_contact_panel: true, show_floating_channels: true, show_sticky_cta: true
  };
  let channelCache = null;

  const parseJSON = (raw, fallback) => { try { return JSON.parse(raw || '') ?? fallback; } catch (_) { return fallback; } };
  const premiumOf = item => ({ ...premiumDefaults, ...parseJSON(item.premium_json, {}) });
  const shortChannel = type => ({ whatsapp: 'WA', telegram: 'TG', messenger: 'MS', instagram: 'IG', facebook: 'FB', linkedin: 'IN', tiktok: 'TK', youtube: 'YT' }[type] || '↗');
  const videoEmbed = raw => {
    try {
      const u = new URL((raw || '').trim());
      const host = u.hostname.replace(/^www\./, '').toLowerCase();
      let id = '';
      if (host === 'youtu.be') id = u.pathname.split('/').filter(Boolean)[0] || '';
      if (host.includes('youtube.com')) id = u.searchParams.get('v') || u.pathname.match(/\/(?:embed|shorts)\/([^/?]+)/)?.[1] || '';
      if (id) return `https://www.youtube-nocookie.com/embed/${encodeURIComponent(id)}?rel=0&modestbranding=1`;
      if (host.includes('vimeo.com')) {
        id = u.pathname.split('/').filter(Boolean).reverse().find(v => v !== 'video') || '';
        if (id) return `https://player.vimeo.com/video/${encodeURIComponent(id)}?title=0&byline=0&portrait=0`;
      }
    } catch (_) {}
    return '';
  };
  const statLines = stats => (Array.isArray(stats) ? stats : []).map(v => `${v.value || ''} | ${v.label || ''}`).join('\n');
  const parseLines = id => ($('#' + id)?.value || '').split('\n').map(v => v.trim()).filter(Boolean);
  const parsePairs = (id, a, b) => parseLines(id).map(v => {
    const [first, ...rest] = v.split('|');
    return { [a]: (first || '').trim(), [b]: rest.join('|').trim() };
  }).filter(v => v[a] || v[b]);
  const selectedChannels = () => $$('.wtl2-channel input:checked').map(input => input.value);

  async function loadChannels() {
    if (channelCache) return channelCache;
    try {
      const data = await api('/api/marketing/landings/channels');
      channelCache = data.channels || [];
    } catch (_) {
      channelCache = [];
    }
    return channelCache;
  }

  function setCTAMode(url) {
    const mode = $('#lpCTAMode')?.value || 'contact';
    const finalURL = mode === 'contact' ? '#contact' : mode === 'form' ? '#form' : ($('#lpCTACustom')?.value || url || '').trim();
    if ($('#lpCTAURL')) $('#lpCTAURL').value = finalURL;
    if ($('#lpCTACustomWrap')) $('#lpCTACustomWrap').hidden = mode !== 'custom';
  }

  function updateMediaPreview() {
    const type = $('input[name="lpMediaType"]:checked')?.value || 'image';
    const image = ($('#lpHero')?.value || '').trim();
    const embed = videoEmbed($('#lpVideoURL')?.value || '');
    const media = $('#wtl2PreviewMedia');
    const editorVideo = $('#lpVideoPreview');
    if (editorVideo) {
      editorVideo.classList.toggle('show', Boolean(embed));
      editorVideo.innerHTML = embed ? `<iframe src="${esc(embed)}" allowfullscreen></iframe>` : '';
    }
    if (!media) return;
    if (type === 'video' && embed) media.innerHTML = `<iframe src="${esc(embed)}" style="width:100%;height:100%;border:0" allowfullscreen></iframe>`;
    else if (image) media.innerHTML = `<img src="${esc(image)}" alt="Vista previa">`;
    else media.innerHTML = '<span>Imagen 1600 × 1200 px<br>o video YouTube/Vimeo</span>';
  }

  function updateLivePreview() {
    const accent = $('#lpAccent')?.value || '#7c3aed';
    const shell = $('.wtl2-shell');
    if (shell) shell.style.setProperty('--lp', accent);
    const text = (id, fallback = '') => ($('#' + id)?.value || fallback).trim();
    const brand = text('lpBrandName', text('lpName', 'Tu marca'));
    if ($('#wtl2PBrand')) $('#wtl2PBrand').textContent = brand || 'Tu marca';
    if ($('#wtl2PBadge')) $('#wtl2PBadge').textContent = text('lpBadge', 'Solución profesional');
    if ($('#wtl2PHeadline')) $('#wtl2PHeadline').textContent = text('lpHeadline', 'Una propuesta que conecta y convierte');
    if ($('#wtl2PSubheadline')) $('#wtl2PSubheadline').textContent = text('lpSubheadline', 'Presenta tu oferta de forma clara, profesional y lista para captar clientes.');
    if ($('#wtl2PCTA')) $('#wtl2PCTA').textContent = text('lpCTA', 'Solicitar información');
    const icons = $('#wtl2PChannels');
    if (icons) {
      const choices = $$('.wtl2-channel input:checked').slice(0, 4);
      icons.innerHTML = choices.map(i => `<span>${shortChannel(i.dataset.type)}</span>`).join('');
    }
    updateMediaPreview();
  }

  async function validateImage(file) {
    return new Promise(resolve => {
      const img = new Image();
      const objectURL = URL.createObjectURL(file);
      img.onload = () => { const result = { width: img.naturalWidth, height: img.naturalHeight }; URL.revokeObjectURL(objectURL); resolve(result); };
      img.onerror = () => { URL.revokeObjectURL(objectURL); resolve({ width: 0, height: 0 }); };
      img.src = objectURL;
    });
  }

  async function uploadHeroImage(file) {
    if (!file) return;
    if (file.size > 8 * 1024 * 1024) return formError('La imagen supera 8 MB. Usa JPG, PNG o WEBP optimizado.');
    const dimensions = await validateImage(file);
    if (dimensions.width && (dimensions.width < 1200 || dimensions.height < 700)) {
      toast(`La imagen mide ${dimensions.width} × ${dimensions.height}. Para un resultado premium recomendamos mínimo 1200 × 800 px.`);
    }
    const button = $('#lpUploadButton');
    const previous = button?.textContent;
    if (button) { button.disabled = true; button.textContent = 'Subiendo imagen…'; }
    try {
      const fd = new FormData(); fd.append('image', file);
      const response = await fetch('/api/marketing/landings/upload', { method: 'POST', credentials: 'same-origin', body: fd });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data.error || 'No fue posible subir la imagen');
      $('#lpHero').value = data.url || '';
      $('input[name="lpMediaType"][value="image"]').checked = true;
      const thumb = $('#lpImageThumb');
      if (thumb) thumb.innerHTML = `<img src="${esc(data.url)}" alt="Imagen cargada">`;
      updateLivePreview();
      $('#modalContent').dispatchEvent(new Event('input', { bubbles: true }));
      toast('Imagen cargada correctamente');
    } catch (error) {
      formError(error.message);
    } finally {
      if (button) { button.disabled = false; button.textContent = previous; }
    }
  }

  function buildPremiumJSON() {
    return JSON.stringify({
      brand_name: ($('#lpBrandName')?.value || '').trim(),
      media_type: $('input[name="lpMediaType"]:checked')?.value || 'image',
      video_url: ($('#lpVideoURL')?.value || '').trim(),
      video_placement: $('#lpVideoPlacement')?.value || 'section',
      hero_alt: ($('#lpHeroAlt')?.value || '').trim(), hero_fit: $('#lpHeroFit')?.value || 'cover', hero_layout: $('#lpHeroLayout')?.value || 'split',
      contact_title: ($('#lpContactTitle')?.value || '').trim(), contact_subtitle: ($('#lpContactSubtitle')?.value || '').trim(), contact_message: ($('#lpContactMessage')?.value || '').trim(),
      channel_keys: selectedChannels(), stats: parsePairs('lpStats', 'value', 'label'), trust_text: ($('#lpTrustText')?.value || '').trim(), footer_text: ($('#lpFooterText')?.value || '').trim(),
      show_contact_panel: Boolean($('#lpShowContact')?.checked), show_floating_channels: Boolean($('#lpShowFloating')?.checked), show_sticky_cta: Boolean($('#lpShowSticky')?.checked)
    });
  }
  window.collectLandingPremiumV2 = buildPremiumJSON;

  openLandingEditor = function(item = {}) {
    const benefits = parseJSON(item.benefits_json, []), features = parseJSON(item.features_json, []), testimonials = parseJSON(item.testimonials_json, []), faqs = parseJSON(item.faq_json, []);
    const premium = premiumOf(item);
    const formOptions = `<option value="0">Sin formulario</option>` + allLandingForms.map(f => `<option value="${f.id}">${esc(f.name)}</option>`).join('');
    const campaignOptions = `<option value="0">Sin campaña</option>` + allLandingCampaigns.map(c => `<option value="${c.id}">${esc(c.name)}</option>`).join('');
    const ctaURL = item.primary_url || '#contact';
    const ctaMode = ctaURL === '#contact' ? 'contact' : ctaURL === '#form' ? 'form' : 'custom';
    const mediaType = premium.media_type === 'video' ? 'video' : 'image';

    modal(`<div class="wtl2-shell">
      <header class="wtl2-header"><div><span class="wtl2-kicker">WORKTIC LANDING STUDIO</span><h2>${item.id ? 'Editar' : 'Crear'} landing premium</h2><p>Diseña una experiencia comercial profesional, responsive y conectada con tus canales.</p></div><div class="wtl2-head-right"><span class="wtl2-status landing-editor-status">${item.published ? 'Publicada' : 'Borrador'}</span></div></header>
      <div class="wtl2-body"><nav class="wtl2-nav">
        <button type="button" class="active" data-wtl2-step="content"><span class="num">1</span><div><b>Contenido</b><small>Marca y propuesta</small></div></button>
        <button type="button" data-wtl2-step="media"><span class="num">2</span><div><b>Multimedia</b><small>Imagen y video</small></div></button>
        <button type="button" data-wtl2-step="sections"><span class="num">3</span><div><b>Secciones</b><small>Beneficios y confianza</small></div></button>
        <button type="button" data-wtl2-step="publish"><span class="num">4</span><div><b>Publicar</b><small>Canales, URL y estado</small></div></button>
        <div class="wtl2-help"><b>Vista segura</b><br>Los cambios se protegen como borrador local mientras editas.</div>
      </nav><div class="wtl2-workspace"><main class="wtl2-editor">
        <section id="wtl2-content" class="wtl2-step">
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Identidad de la landing</h3><p>Estos datos organizan la página dentro de Worktic y presentan tu marca.</p></div><span class="wtl2-badge">Paso 1</span></div>
            <div class="wtl2-grid2"><label class="wtl2-field"><span>Nombre interno *</span><input id="lpName" value="${esc(item.name || '')}" placeholder="Ej. Landing venta plan empresarial"></label><label class="wtl2-field"><span>Marca visible</span><input id="lpBrandName" value="${esc(premium.brand_name || '')}" placeholder="Ej. Worktic AI"></label></div>
            <div class="wtl2-grid2"><label class="wtl2-field"><span>Producto o servicio</span><input id="lpProduct" placeholder="Plan, asesoría, curso, tratamiento..."></label><label class="wtl2-field"><span>Público ideal</span><input id="lpAudience" placeholder="Empresas, profesionales, familias..."></label></div>
            <div class="wtl2-grid2"><label class="wtl2-field"><span>Objetivo</span><select id="lpObjective"><option>Capturar leads</option><option>Conseguir conversaciones</option><option>Agendar citas</option><option>Vender un producto</option><option>Presentar un servicio</option></select></label><label class="wtl2-field"><span>Prueba de confianza</span><input id="lpTrustText" value="${esc(premium.trust_text || '')}" placeholder="Atención rápida, segura y personalizada"></label></div>
            <button type="button" id="generateLandingAI" class="wtl2-action secondary">✨ Generar contenido comercial con IA</button>
          </div>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Encabezado principal</h3><p>La primera pantalla debe comunicar con claridad qué ofreces y por qué importa.</p></div><span class="wtl2-badge">Hero</span></div>
            <label class="wtl2-field"><span>Etiqueta superior</span><input id="lpBadge" value="${esc(item.badge || 'Solución profesional')}"></label>
            <label class="wtl2-field"><span>Título principal *</span><textarea id="lpHeadline" rows="3" placeholder="Una promesa clara y específica">${esc(item.headline || '')}</textarea></label>
            <label class="wtl2-field"><span>Subtítulo</span><textarea id="lpSubheadline" rows="3" placeholder="Explica el resultado, el proceso y para quién es la oferta">${esc(item.subheadline || '')}</textarea></label>
            <div class="wtl2-grid2"><label class="wtl2-field"><span>Texto del botón principal</span><input id="lpCTA" value="${esc(item.primary_cta || 'Solicitar información')}"></label><label class="wtl2-field"><span>Destino del botón</span><select id="lpCTAMode"><option value="contact">Canales de contacto</option><option value="form">Formulario</option><option value="custom">Enlace personalizado</option></select></label></div>
            <label id="lpCTACustomWrap" class="wtl2-field" ${ctaMode === 'custom' ? '' : 'hidden'}><span>Enlace personalizado</span><input id="lpCTACustom" value="${esc(ctaMode === 'custom' ? ctaURL : '')}" placeholder="https://..."></label><input id="lpCTAURL" type="hidden" value="${esc(ctaURL)}">
            <div class="wtl2-grid2"><label class="wtl2-field"><span>Botón secundario</span><input id="lpSecondaryCTA" value="${esc(item.secondary_cta || '')}" placeholder="Ej. Ver catálogo"></label><label class="wtl2-field"><span>Enlace secundario</span><input id="lpSecondaryURL" value="${esc(item.secondary_url || '')}" placeholder="https://... o #form"></label></div>
          </div>
        </section>
        <section id="wtl2-media" class="wtl2-step" hidden>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Multimedia principal</h3><p>Usa una imagen horizontal de alta calidad o un video de YouTube/Vimeo.</p></div><span class="wtl2-badge">1600 × 1200 recomendado</span></div>
            <div class="wtl2-media-switch"><label><input type="radio" name="lpMediaType" value="image" ${mediaType === 'image' ? 'checked' : ''}><span>🖼️ Imagen principal</span></label><label><input type="radio" name="lpMediaType" value="video" ${mediaType === 'video' ? 'checked' : ''}><span>▶️ Video principal</span></label></div><br>
            <div class="wtl2-upload"><div id="lpImageThumb" class="wtl2-media-thumb">${item.hero_image ? `<img src="${esc(item.hero_image)}" alt="Vista previa">` : '<span>Vista previa<br>relación 4:3</span>'}</div><div class="wtl2-upload-actions">
              <label class="wtl2-drop"><input id="lpHeroFile" type="file" accept="image/jpeg,image/png,image/webp,image/gif" hidden><strong id="lpUploadButton">Subir imagen desde el equipo</strong><span>JPG, PNG o WEBP · máximo 8 MB</span></label>
              <label class="wtl2-field"><span>O pega la URL de la imagen</span><input id="lpHero" value="${esc(item.hero_image || '')}" placeholder="https://..."></label>
              <div class="wtl2-grid2"><label class="wtl2-field"><span>Texto alternativo</span><input id="lpHeroAlt" value="${esc(premium.hero_alt || '')}" placeholder="Describe brevemente la imagen"></label><label class="wtl2-field"><span>Ajuste de imagen</span><select id="lpHeroFit"><option value="cover">Cubrir el espacio</option><option value="contain">Mostrar completa</option></select></label></div>
            </div></div>
          </div>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Video de presentación</h3><p>Compatible con enlaces públicos de YouTube, YouTube Shorts y Vimeo.</p></div><span class="wtl2-badge">16:9</span></div>
            <label class="wtl2-field"><span>URL del video</span><input id="lpVideoURL" value="${esc(premium.video_url || '')}" placeholder="https://youtube.com/watch?v=... o https://vimeo.com/..."></label>
            <label class="wtl2-field"><span>Ubicación del video</span><select id="lpVideoPlacement"><option value="section">Sección independiente</option><option value="hero">En el encabezado principal</option></select></label>
            <div id="lpVideoPreview" class="wtl2-video-preview"></div>
          </div>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Composición visual</h3><p>Controla cómo se distribuye el contenido en la primera pantalla.</p></div></div><div class="wtl2-grid2"><label class="wtl2-field"><span>Diseño del hero</span><select id="lpHeroLayout"><option value="split">Texto + multimedia</option><option value="center">Contenido centrado</option></select></label><label class="wtl2-field"><span>Estadísticas destacadas</span><textarea id="lpStats" rows="4" placeholder="24/7 | Atención disponible\n+100 | Clientes atendidos">${esc(statLines(premium.stats))}</textarea><small>Una por línea: Valor | Descripción. Máximo 4.</small></label></div></div>
        </section>
        <section id="wtl2-sections" class="wtl2-step" hidden>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Beneficios rápidos</h3><p>Mensajes breves que el visitante puede entender en segundos.</p></div></div><label class="wtl2-field"><span>Un beneficio por línea</span><textarea id="lpBenefits" rows="6">${esc(benefits.join('\n'))}</textarea></label></div>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Características de la oferta</h3><p>Explica qué incluye, cómo funciona y qué valor entrega.</p></div></div><label class="wtl2-field"><span>Una por línea: Título | Descripción</span><textarea id="lpFeatures" rows="9">${esc(features.map(v => `${v.title || ''} | ${v.text || ''}`).join('\n'))}</textarea></label></div>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Testimonios</h3><p>Agrega únicamente experiencias reales y autorizadas.</p></div></div><label class="wtl2-field"><span>Uno por línea: Testimonio | Autor</span><textarea id="lpTestimonials" rows="7">${esc(testimonials.map(v => `${v.quote || ''} | ${v.author || ''}`).join('\n'))}</textarea></label></div>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Preguntas frecuentes</h3><p>Resuelve objeciones antes de que el visitante abandone la página.</p></div></div><label class="wtl2-field"><span>Una por línea: Pregunta | Respuesta</span><textarea id="lpFAQ" rows="10">${esc(faqs.map(v => `${v.question || ''} | ${v.answer || ''}`).join('\n'))}</textarea></label></div>
        </section>
        <section id="wtl2-publish" class="wtl2-step" hidden>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Canales de contacto conectados</h3><p>Selecciona los iconos que podrán usar los visitantes para escribirte directamente.</p></div><span class="wtl2-badge">Automático</span></div><div id="lpChannelGrid" class="wtl2-channel-grid"><div class="wtl2-empty">Cargando canales configurados…</div></div><br><label class="wtl2-field"><span>Mensaje inicial para WhatsApp</span><textarea id="lpContactMessage" rows="3" placeholder="Hola, vi la landing y quiero más información...">${esc(premium.contact_message || '')}</textarea></label><div class="wtl2-grid2"><label class="wtl2-field"><span>Título del bloque de contacto</span><input id="lpContactTitle" value="${esc(premium.contact_title || '')}"></label><label class="wtl2-field"><span>Descripción</span><input id="lpContactSubtitle" value="${esc(premium.contact_subtitle || '')}"></label></div></div>
          <div class="wtl2-section"><div class="wtl2-section-head"><div><h3>Diseño y publicación</h3><p>Configura la apariencia, la URL pública y las conexiones comerciales.</p></div></div>
            <div class="wtl2-grid2"><label class="wtl2-field"><span>Slug público</span><input id="lpSlug" value="${esc(item.slug || '')}" placeholder="mi-oferta"></label><label class="wtl2-field"><span>Estilo visual</span><select id="lpTemplate"><option value="aurora">Aurora premium</option><option value="minimal">Minimal profesional</option><option value="bold">Comercial bold</option><option value="midnight">Midnight oscuro</option></select></label></div>
            <div class="wtl2-grid2"><label class="wtl2-field"><span>Color principal</span><div class="wtl2-color-row"><input id="lpAccent" type="color" value="${esc(item.accent || '#7c3aed')}"><small>Se aplica a botones, detalles y llamadas a la acción.</small></div></label><label class="wtl2-field"><span>Formulario relacionado</span><select id="lpForm">${formOptions}</select></label></div>
            <div class="wtl2-grid2"><label class="wtl2-field"><span>Campaña relacionada</span><select id="lpCampaign">${campaignOptions}</select></label><label class="wtl2-field"><span>Texto del pie de página</span><input id="lpFooterText" value="${esc(premium.footer_text || '')}" placeholder="Creado con Worktic AI"></label></div>
          </div>
          <div class="wtl2-section"><div class="wtl2-switch"><span><b>Mostrar bloque de contacto</b><small>Presenta los canales como tarjetas dentro de la página.</small></span><input id="lpShowContact" type="checkbox" ${premium.show_contact_panel !== false ? 'checked' : ''}></div><div class="wtl2-switch"><span><b>Iconos flotantes</b><small>Mantiene accesos rápidos visibles en escritorio.</small></span><input id="lpShowFloating" type="checkbox" ${premium.show_floating_channels !== false ? 'checked' : ''}></div><div class="wtl2-switch"><span><b>Botón fijo en móvil</b><small>Facilita la conversión desde teléfonos.</small></span><input id="lpShowSticky" type="checkbox" ${premium.show_sticky_cta !== false ? 'checked' : ''}></div><div class="wtl2-switch"><span><b>Publicar landing</b><small>Al activarla, cualquier persona con el enlace podrá verla.</small></span><input id="lpPublished" type="checkbox" ${item.published ? 'checked' : ''}></div></div>
          <div class="wtl2-notice">La landing es completamente adaptable a móviles. La imagen usa una proporción estable y el video se integra sin romper el diseño.</div>
        </section>
      </main><aside class="wtl2-preview"><div class="wtl2-preview-sticky"><div class="wtl2-preview-head"><b>Vista previa</b><span class="wtl2-device">Escritorio · responsive</span></div><div class="wtl2-phone"><div class="wtl2-browserbar"><i></i><i></i><i></i></div><div class="wtl2-page-preview"><div class="wtl2-pnav"><span id="wtl2PBrand">${esc(premium.brand_name || item.name || 'Tu marca')}</span><span class="wtl2-pcta" id="wtl2PCTA">${esc(item.primary_cta || 'Solicitar información')}</span></div><div class="wtl2-phero"><span class="wtl2-peyebrow" id="wtl2PBadge">${esc(item.badge || 'Solución profesional')}</span><h4 id="wtl2PHeadline">${esc(item.headline || 'Una propuesta que conecta y convierte')}</h4><p id="wtl2PSubheadline">${esc(item.subheadline || 'Presenta tu oferta de forma clara, profesional y lista para captar clientes.')}</p><span class="wtl2-pbutton">${esc(item.primary_cta || 'Solicitar información')}</span></div><div id="wtl2PreviewMedia" class="wtl2-pmedia"></div><div id="wtl2PChannels" class="wtl2-pchannels"></div><div class="wtl2-pcards"><i></i><i></i><i></i></div></div></div></div></aside></div></div>
      <footer class="wtl2-footer"><span class="wtl2-save-note">Tus cambios se protegen automáticamente en este navegador.</span><button value="cancel" class="wtl2-action secondary">Cancelar</button><button type="button" id="saveLanding" class="wtl2-action">Guardar landing</button></footer>
    </div>`);

    $('#lpTemplate').value = item.template || 'aurora'; $('#lpForm').value = String(item.form_id || 0); $('#lpCampaign').value = String(item.campaign_id || 0);
    $('#lpCTAMode').value = ctaMode; $('#lpVideoPlacement').value = premium.video_placement || 'section'; $('#lpHeroFit').value = premium.hero_fit || 'cover'; $('#lpHeroLayout').value = premium.hero_layout || 'split';
    setCTAMode(ctaURL);

    $$('[data-wtl2-step]').forEach(button => button.onclick = () => {
      $$('[data-wtl2-step]').forEach(v => v.classList.remove('active')); button.classList.add('active');
      $$('.wtl2-step').forEach(v => v.hidden = true); $('#wtl2-' + button.dataset.wtl2Step).hidden = false;
    });
    $('#lpCTAMode').onchange = () => { setCTAMode(ctaURL); updateLivePreview(); };
    $('#lpCTACustom').oninput = () => setCTAMode(ctaURL);
    $('#lpHeroFile').onchange = event => uploadHeroImage(event.target.files?.[0]);
    $('#lpHero').oninput = () => { const value = $('#lpHero').value.trim(); $('#lpImageThumb').innerHTML = value ? `<img src="${esc(value)}" alt="Vista previa">` : '<span>Vista previa<br>relación 4:3</span>'; updateLivePreview(); };
    $('#lpVideoURL').oninput = updateLivePreview;
    $$('input[name="lpMediaType"]').forEach(input => input.onchange = updateLivePreview);
    $('#modalContent').addEventListener('input', updateLivePreview); $('#modalContent').addEventListener('change', updateLivePreview);

    loadChannels().then(channels => {
      const selected = new Set(premium.channel_keys || []), grid = $('#lpChannelGrid');
      if (!grid) return;
      grid.innerHTML = channels.length ? channels.map(ch => `<label class="wtl2-channel"><input type="checkbox" value="${esc(ch.key)}" data-type="${esc(ch.type)}" ${selected.has(ch.key) || (!premium.channel_keys?.length && ['whatsapp','telegram','messenger','instagram'].includes(ch.type)) ? 'checked' : ''}><span class="wtl2-channel-card"><span class="wtl2-channel-icon">${shortChannel(ch.type)}</span><span><b>${esc(ch.label || ch.type)}</b><small>${esc(ch.name || ch.url)}</small></span></span></label>`).join('') : '<div class="wtl2-empty">No hay canales conectados. Conecta WhatsApp, Telegram, Messenger o una red social para mostrarlos aquí.</div>';
      $$('.wtl2-channel input').forEach(input => input.onchange = updateLivePreview);
      updateLivePreview();
    });

    $('#generateLandingAI').onclick = async () => {
      const button = $('#generateLandingAI'); button.disabled = true; const old = button.textContent; button.textContent = 'Generando propuesta…';
      try {
        const result = await api('/api/marketing/landings/generate', { method: 'POST', body: JSON.stringify({ name: $('#lpName').value, product: $('#lpProduct').value, audience: $('#lpAudience').value, objective: $('#lpObjective').value, tone: 'Profesional, moderno y comercial' }) });
        if (result.raw) $('#lpSubheadline').value = result.raw;
        else {
          $('#lpHeadline').value = result.headline || ''; $('#lpSubheadline').value = result.subheadline || ''; $('#lpBadge').value = result.badge || ''; $('#lpCTA').value = result.primary_cta || 'Solicitar información';
          $('#lpBenefits').value = (result.benefits || []).join('\n'); $('#lpFeatures').value = (result.features || []).map(v => `${v.title || ''} | ${v.text || ''}`).join('\n');
          $('#lpTestimonials').value = (result.testimonials || []).map(v => `${v.quote || ''} | ${v.author || ''}`).join('\n'); $('#lpFAQ').value = (result.faq || []).map(v => `${v.question || ''} | ${v.answer || ''}`).join('\n');
          $('#lpStats').value = (result.stats || []).map(v => `${v.value || ''} | ${v.label || ''}`).join('\n'); if (result.trust_text) $('#lpTrustText').value = result.trust_text;
        }
        updateLivePreview(); $('#modalContent').dispatchEvent(new Event('input', { bubbles: true })); toast('Contenido generado. Revísalo y personalízalo antes de publicar.');
      } catch (error) { formError(error.message); } finally { button.disabled = false; button.textContent = old; }
    };

    // Respaldo funcional. landing-save-guard.js reemplaza este evento con guardado verificado y borrador local.
    $('#saveLanding').onclick = async () => {
      setCTAMode(ctaURL);
      const payload = { id: item.id || 0, name: $('#lpName').value.trim(), slug: $('#lpSlug').value.trim(), template: $('#lpTemplate').value, headline: $('#lpHeadline').value.trim(), subheadline: $('#lpSubheadline').value.trim(), badge: $('#lpBadge').value.trim(), primary_cta: $('#lpCTA').value.trim(), primary_url: $('#lpCTAURL').value.trim(), secondary_cta: $('#lpSecondaryCTA').value.trim(), secondary_url: $('#lpSecondaryURL').value.trim(), hero_image: $('#lpHero').value.trim(), benefits_json: JSON.stringify(parseLines('lpBenefits')), features_json: JSON.stringify(parsePairs('lpFeatures','title','text')), testimonials_json: JSON.stringify(parsePairs('lpTestimonials','quote','author')), faq_json: JSON.stringify(parsePairs('lpFAQ','question','answer')), premium_json: buildPremiumJSON(), form_id: Number($('#lpForm').value), campaign_id: Number($('#lpCampaign').value), accent: $('#lpAccent').value, published: $('#lpPublished').checked };
      if (!payload.name) return formError('Escribe el nombre interno.'); if (!payload.headline) return formError('Escribe el título principal.');
      try { await api('/api/marketing/landings', { method: item.id ? 'PUT' : 'POST', body: JSON.stringify(payload) }); $('#modal').close(); await loadLandings(); toast('Landing guardada'); } catch (error) { formError(error.message); }
    };
    updateLivePreview();
  };
})();
