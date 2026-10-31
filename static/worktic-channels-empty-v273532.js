/* Worktic AI V27.35.33 — Mejora visual exclusivamente del estado vacío en Canales.
   No modifica API, conexiones, agentes, otros módulos ni eventos originales. */
(function () {
  'use strict';
  const phrase = /^No hay conexiones\.\s*Usa\s*[«"“]?Nueva conexión[»"”]?\s*para conectar tu primer canal\.?$/i;
  let pending = false;

  function makeEmptyCard() {
    const article = document.createElement('article');
    article.className = 'wt32-channels-empty';
    article.setAttribute('aria-label', 'Cómo conectar tu primer canal');

    const header = document.createElement('div');
    header.className = 'wt32-channels-empty-header';

    const icon = document.createElement('span');
    icon.className = 'wt32-channels-empty-icon';
    icon.setAttribute('aria-hidden', 'true');
    icon.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 11.5a8 8 0 0 1-8 8 9 9 0 0 1-3.3-.6L4 20l1.2-4.3A8 8 0 1 1 20 11.5Z"/><path d="M8.5 11.5h7"/></svg>';

    const heading = document.createElement('div');
    heading.className = 'wt32-channels-empty-heading';
    const title = document.createElement('h3');
    title.textContent = 'Conecta tu primer canal';
    const subtitle = document.createElement('span');
    subtitle.className = 'wt32-channels-empty-subtitle';
    subtitle.textContent = 'Comienza a atender a tus clientes desde un solo lugar.';
    heading.append(title, subtitle);
    header.append(icon, heading);

    const description = document.createElement('p');
    description.textContent = 'Haz clic en «Nueva conexión» y elige WhatsApp Business Cloud (Oficial), WhatsApp QR, Telegram Bot, Facebook Messenger, Chat Web o Llamadas IA. Sigue los pasos de vinculación y configura la atención con tus agentes de IA.';

    article.append(header, description);
    return article;
  }

  function enhance() {
    // Solo transformar el mensaje exacto de Canales; no intervenir en listas con conexiones.
    const root = document.querySelector('main') || document.body;
    if (!root) return;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    let node;
    while ((node = walker.nextNode())) {
      const value = (node.nodeValue || '').replace(/\s+/g, ' ').trim();
      if (!phrase.test(value)) continue;
      const parent = node.parentElement;
      if (!parent || parent.closest('.wt32-channels-empty,script,style')) continue;
      const card = makeEmptyCard();
      if (parent.textContent.replace(/\s+/g, ' ').trim() === value &&
          !['BODY', 'MAIN', 'SECTION', 'TABLE', 'TBODY', 'TR'].includes(parent.tagName)) {
        parent.replaceWith(card);
      } else {
        // No sustituimos contenedores compartidos con otros controles de la pantalla.
        const slot = document.createElement('span');
        slot.className = 'wt32-channels-empty-slot';
        slot.append(card);
        node.replaceWith(slot);
      }
      return;
    }
  }

  function schedule() {
    if (pending) return;
    pending = true;
    window.setTimeout(function () {
      pending = false;
      enhance();
    }, 120);
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', enhance, { once: true });
  else enhance();
  const observer = new MutationObserver(schedule);
  observer.observe(document.documentElement, { childList: true, subtree: true });
})();
