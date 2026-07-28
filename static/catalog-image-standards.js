(() => {
  if (typeof window.productForm !== 'function') return;

  const standard = {
    recommendedWidth: 1200,
    recommendedHeight: 1200,
    minWidth: 800,
    minHeight: 800,
    targetRatio: 1,
    tolerance: 0.05,
    maxBytes: 5 * 1024 * 1024
  };

  const originalProductForm = window.productForm;
  let measureTimer = 0;

  const ratioOK = ({ width, height }) => {
    if (!width || !height) return false;
    const ratio = width / height;
    return Math.abs(ratio - standard.targetRatio) / standard.targetRatio <= standard.tolerance;
  };

  const evaluate = dimensions => {
    if (!dimensions?.width || !dimensions?.height) {
      return { state: 'pending', valid: true, title: 'Dimensiones pendientes', text: 'Portada y galería: 1200 × 1200 px recomendado, mínimo 800 × 800 px, relación 1:1.' };
    }
    const enough = dimensions.width >= standard.minWidth && dimensions.height >= standard.minHeight;
    const square = ratioOK(dimensions);
    if (!enough || !square) {
      return {
        state: 'error', valid: false,
        title: `${dimensions.width} × ${dimensions.height} px · No cumple`,
        text: !enough ? 'La imagen es demasiado pequeña. El mínimo es 800 × 800 px.' : 'La imagen debe ser cuadrada (1:1) para catálogo y envío por WhatsApp.'
      };
    }
    const perfect = dimensions.width === standard.recommendedWidth && dimensions.height === standard.recommendedHeight;
    return {
      state: perfect ? 'perfect' : 'ok', valid: true,
      title: `${dimensions.width} × ${dimensions.height} px · ${perfect ? 'Perfecta' : 'Compatible'}`,
      text: perfect ? 'Tamaño exacto recomendado para catálogo, tarjetas y WhatsApp.' : 'Cumple el estándar 1:1. Para máxima nitidez recomendamos 1200 × 1200 px.'
    };
  };

  const renderStatus = (dimensions, enforce = false, custom) => {
    const el = document.querySelector('#catalogImageMetrics');
    if (!el) return true;
    const result = custom || evaluate(dimensions);
    el.className = `catalog-image-metrics ${result.state}`;
    el.innerHTML = `<span class="catalog-metric-icon">${result.state === 'error' ? '!' : result.state === 'perfect' ? '✓' : 'i'}</span><span><b>${escapeHTML(result.title)}</b><small>${escapeHTML(result.text)}</small></span>`;
    el.dataset.valid = result.valid === false ? '0' : '1';
    el.dataset.enforce = enforce ? '1' : '0';
    return result.valid !== false;
  };

  const escapeHTML = value => String(value ?? '').replace(/[&<>'"]/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[char]));

  const inspectFile = file => new Promise(resolve => {
    const image = new Image();
    const url = URL.createObjectURL(file);
    image.onload = () => {
      const dimensions = { width: image.naturalWidth, height: image.naturalHeight };
      URL.revokeObjectURL(url);
      resolve(dimensions);
    };
    image.onerror = () => { URL.revokeObjectURL(url); resolve({ width: 0, height: 0 }); };
    image.src = url;
  });

  const inspectURL = (raw, enforce) => {
    const url = String(raw || '').trim();
    if (!url) {
      renderStatus(null, enforce);
      return;
    }
    const image = new Image();
    image.onload = () => renderStatus({ width: image.naturalWidth, height: image.naturalHeight }, enforce);
    image.onerror = () => renderStatus(null, false, {
      state: 'pending', valid: true,
      title: 'No pudimos medir esta URL',
      text: 'Verifica que sea pública. Debe ser cuadrada y recomendamos 1200 × 1200 px.'
    });
    image.src = url + (url.includes('?') ? '&' : '?') + 'wtc_measure=1';
  };

  function enhanceCatalogImageForm(product = {}) {
    const mainFile = document.querySelector('#pMainFile');
    const galleryFiles = document.querySelector('#pGalleryFiles');
    const imageURL = document.querySelector('#pImageURL');
    const save = document.querySelector('#saveProduct');
    if (!mainFile || !galleryFiles || !imageURL || !save || mainFile.dataset.standardReady === '1') return;
    mainFile.dataset.standardReady = '1';
    mainFile.accept = 'image/jpeg,image/png,image/webp';
    galleryFiles.accept = 'image/jpeg,image/png,image/webp';

    const section = mainFile.closest('.form-section');
    const headingText = section?.querySelector('.form-section-title p');
    if (headingText) headingText.textContent = 'Imágenes uniformes, listas para el catálogo y para compartir por WhatsApp.';
    const uploadHelp = mainFile.closest('.upload-box')?.querySelector('small');
    if (uploadHelp) uploadHelp.textContent = '1200 × 1200 px ideal · mínimo 800 × 800 · relación 1:1 · JPG, PNG o WEBP · máximo 5 MB.';

    const standardPanel = document.createElement('div');
    standardPanel.className = 'catalog-standard-panel';
    standardPanel.innerHTML = `
      <div class="catalog-standard-visual"><div class="catalog-safe-square"><span>1:1</span></div></div>
      <div><span class="catalog-standard-kicker">ESTÁNDAR VISUAL DEL CATÁLOGO</span><h4>Usa imágenes cuadradas de 1200 × 1200 px</h4><p>Este formato evita recortes extraños en las tarjetas y mantiene una presentación consistente al compartir productos por WhatsApp.</p><div class="catalog-standard-tags"><span>Ideal 1200 × 1200</span><span>Mínimo 800 × 800</span><span>Máximo 5 MB</span><span>JPG · PNG · WEBP</span></div></div>`;
    section?.querySelector('.catalog-image-editor')?.before(standardPanel);

    const metrics = document.createElement('div');
    metrics.id = 'catalogImageMetrics';
    metrics.className = 'catalog-image-metrics pending';
    metrics.innerHTML = '<span class="catalog-metric-icon">i</span><span><b>Dimensiones pendientes</b><small>La imagen principal y las imágenes de galería deben usar relación 1:1.</small></span>';
    document.querySelector('.catalog-image-editor')?.after(metrics);

    const originalMainChange = mainFile.onchange;
    mainFile.onchange = async event => {
      const file = event.target.files?.[0];
      if (!file) return;
      if (file.size > standard.maxBytes) {
        event.target.value = '';
        renderStatus(null, true, { state: 'error', valid: false, title: 'Archivo demasiado grande', text: 'El máximo permitido es 5 MB.' });
        return typeof toast === 'function' && toast('La imagen supera 5 MB.');
      }
      const dimensions = await inspectFile(file);
      const valid = renderStatus(dimensions, true);
      if (!valid) {
        event.target.value = '';
        return typeof toast === 'function' && toast('La imagen no cumple el estándar cuadrado del catálogo.');
      }
      if (typeof originalMainChange === 'function') await originalMainChange.call(mainFile, event);
    };

    const originalGalleryChange = galleryFiles.onchange;
    galleryFiles.onchange = async event => {
      const files = [...(event.target.files || [])];
      if (!files.length) return;
      for (const file of files) {
        if (file.size > standard.maxBytes) {
          event.target.value = '';
          return typeof toast === 'function' && toast(`${file.name}: supera el máximo de 5 MB.`);
        }
        const dimensions = await inspectFile(file);
        const result = evaluate(dimensions);
        if (!result.valid) {
          event.target.value = '';
          renderStatus(dimensions, true);
          return typeof toast === 'function' && toast(`${file.name}: debe ser cuadrada y medir mínimo 800 × 800 px.`);
        }
      }
      if (typeof originalGalleryChange === 'function') await originalGalleryChange.call(galleryFiles, event);
    };

    imageURL.addEventListener('input', () => {
      clearTimeout(measureTimer);
      measureTimer = setTimeout(() => inspectURL(imageURL.value, true), 400);
    });

    const originalSave = save.onclick;
    save.onclick = async event => {
      const status = document.querySelector('#catalogImageMetrics');
      if (status?.dataset.enforce === '1' && status.dataset.valid === '0') {
        event.preventDefault();
        return typeof toast === 'function' && toast('Corrige la imagen principal antes de guardar el producto.');
      }
      if (typeof originalSave === 'function') return originalSave.call(save, event);
    };

    inspectURL(product.image_url || '', Boolean(product.image_url));
  }

  window.productForm = function productFormWithImageStandards(product) {
    originalProductForm(product);
    requestAnimationFrame(() => enhanceCatalogImageForm(product || {}));
  };
})();
