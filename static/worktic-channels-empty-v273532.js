/* Worktic AI V27.35.32 — Mejora visual exclusivamente del estado vacío en Canales.
   No modifica API, conexiones, agentes, otros módulos ni eventos originales. */
(function () {
  'use strict';
  const phrase = /^No hay conexiones\.\s*Usa\s*[«"“]?Nueva conexión[»"”]?\s*para conectar tu primer canal\.?$/i;
  let pending = false;

  function makeEmptyCard() {
    const article = document.createElement('article');
    article.className = 'wt32-channels-empty';
    article.setAttribute('aria-label', 'Cómo conectar tu primer canal');

    const title = document.createElement('h3');
    title.textContent = 'Conecta tu primer canal';

    const description = document.createElement('p');
    description.textContent = 'Haz clic en «Nueva conexión», elige WhatsApp, Messenger, Telegram o Chat Web y sigue los pasos de vinculación. Después podrás asignar un agente de IA para atender tus conversaciones.';

    article.append(title, description);
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
