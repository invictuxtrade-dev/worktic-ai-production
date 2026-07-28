(() => {
  if (typeof openLandingEditor !== 'function') return;

  const originalOpenLandingEditor = openLandingEditor;
  const draftKey = id => `worktic_landing_safe_draft_${id || 'new'}`;

  const readDraft = id => {
    try {
      const raw = localStorage.getItem(draftKey(id));
      return raw ? JSON.parse(raw) : null;
    } catch (_) {
      return null;
    }
  };

  const saveDraft = (id, value) => {
    try {
      localStorage.setItem(draftKey(id), JSON.stringify({ ...value, _saved_at: new Date().toISOString() }));
      return true;
    } catch (_) {
      return false;
    }
  };

  const removeDraft = id => {
    try { localStorage.removeItem(draftKey(id)); } catch (_) {}
  };

  const lines = id => ($(`#${id}`)?.value || '').split('\n').map(value => value.trim()).filter(Boolean);
  const pairs = (id, firstKey, secondKey) => lines(id).map(value => {
    const [first, ...rest] = value.split('|');
    return { [firstKey]: (first || '').trim(), [secondKey]: rest.join('|').trim() };
  }).filter(value => value[firstKey] || value[secondKey]);

  const collectLanding = item => ({
    id: item.id || 0,
    name: ($('#lpName')?.value || '').trim(),
    slug: ($('#lpSlug')?.value || '').trim(),
    template: $('#lpTemplate')?.value || 'aurora',
    headline: ($('#lpHeadline')?.value || '').trim(),
    subheadline: ($('#lpSubheadline')?.value || '').trim(),
    badge: ($('#lpBadge')?.value || '').trim(),
    primary_cta: ($('#lpCTA')?.value || '').trim(),
    primary_url: ($('#lpCTAURL')?.value || '').trim(),
    secondary_cta: item.secondary_cta || '',
    secondary_url: item.secondary_url || '',
    hero_image: ($('#lpHero')?.value || '').trim(),
    benefits_json: JSON.stringify(lines('lpBenefits')),
    features_json: JSON.stringify(pairs('lpFeatures', 'title', 'text')),
    testimonials_json: JSON.stringify(pairs('lpTestimonials', 'quote', 'author')),
    faq_json: JSON.stringify(pairs('lpFAQ', 'question', 'answer')),
    form_id: Number($('#lpForm')?.value || 0),
    campaign_id: Number($('#lpCampaign')?.value || 0),
    accent: $('#lpAccent')?.value || '#7c3aed',
    published: Boolean($('#lpPublished')?.checked),
    _product: $('#lpProduct')?.value || '',
    _audience: $('#lpAudience')?.value || '',
    _objective: $('#lpObjective')?.value || ''
  });

  async function landingRequest(payload, editing) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 25000);
    try {
      const response = await fetch('/api/marketing/landings', {
        method: editing ? 'PUT' : 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        signal: controller.signal
      });
      const text = await response.text();
      let data = {};
      try { data = text ? JSON.parse(text) : {}; } catch (_) { data = {}; }
      if (response.status === 401) {
        location.replace('/login.html?reason=session');
        throw new Error(data.error || 'Tu sesión venció. El borrador quedó guardado en este navegador.');
      }
      if (!response.ok) throw new Error(data.error || text || `Error ${response.status} al guardar`);
      if (!data.id && !data.ok) throw new Error('El servidor respondió, pero no confirmó el guardado.');
      return data;
    } catch (error) {
      if (error.name === 'AbortError') throw new Error('El servidor tardó demasiado. Tus datos siguen guardados como borrador local.');
      throw error;
    } finally {
      clearTimeout(timer);
    }
  }

  openLandingEditor = function(item = {}) {
    const originalID = item.id || 0;
    const savedDraft = readDraft(originalID);
    const restored = savedDraft ? { ...item, ...savedDraft, id: originalID } : item;
    originalOpenLandingEditor(restored);

    if (!$('#saveLanding')) return;
    if (savedDraft) {
      if ($('#lpProduct')) $('#lpProduct').value = savedDraft._product || '';
      if ($('#lpAudience')) $('#lpAudience').value = savedDraft._audience || '';
      if ($('#lpObjective')) $('#lpObjective').value = savedDraft._objective || 'Capturar leads';
      setTimeout(() => toast('Recuperamos el borrador que no se había guardado'), 120);
    }

    const status = document.querySelector('.landing-editor-status');
    if (status) {
      status.insertAdjacentHTML('afterend', '<small id="landingLocalDraftStatus" class="landing-local-draft-status">Borrador local protegido</small>');
    }

    let timer;
    const persist = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        const ok = saveDraft(originalID, collectLanding(restored));
        const label = $('#landingLocalDraftStatus');
        if (label) label.textContent = ok ? `Borrador local guardado · ${new Date().toLocaleTimeString('es-CO', { hour: '2-digit', minute: '2-digit' })}` : 'No fue posible guardar el borrador local';
      }, 350);
    };
    $('#modalContent').addEventListener('input', persist);
    $('#modalContent').addEventListener('change', persist);
    persist();

    const generateButton = $('#generateLandingAI');
    if (generateButton?.onclick) {
      const originalGenerate = generateButton.onclick;
      generateButton.onclick = async event => {
        await originalGenerate.call(generateButton, event);
        persist();
      };
    }

    $('#saveLanding').onclick = async () => {
      const button = $('#saveLanding');
      const payload = collectLanding(restored);
      saveDraft(originalID, payload);
      if (!payload.name) return formError('Escribe el nombre interno de la landing. Tus demás datos ya están protegidos.');
      if (!payload.headline) return formError('Escribe el título principal. Tus demás datos ya están protegidos.');

      button.disabled = true;
      const oldText = button.textContent;
      button.textContent = 'Guardando y verificando…';
      const localStatus = $('#landingLocalDraftStatus');
      if (localStatus) localStatus.textContent = 'Guardando en el servidor…';

      try {
        const result = await landingRequest(payload, Boolean(originalID));
        const savedID = Number(result.id || originalID);
        if (!savedID) throw new Error('El servidor no confirmó el identificador de la landing.');
        removeDraft(originalID);
        if (!originalID) removeDraft('new');
        $('#modal').close();
        try {
          await loadLandings();
          toast('Landing guardada y verificada correctamente');
        } catch (_) {
          toast('Landing guardada. Actualiza el listado para verla.');
        }
      } catch (error) {
        formError(`No se guardó: ${error.message}`);
        if (localStatus) localStatus.textContent = 'Tus datos permanecen protegidos como borrador local';
        button.disabled = false;
        button.textContent = oldText;
      }
    };
  };
})();
