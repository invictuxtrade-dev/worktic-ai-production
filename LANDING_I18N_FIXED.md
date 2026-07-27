# Landing pública bilingüe restaurada

Esta versión restaura en `static/index.html`:

- Botón visible ES/EN en la navegación pública.
- Atributos `data-i18n` en toda la landing.
- Carga de `/i18n.js` antes de cerrar `body`.
- Persistencia del idioma con `localStorage` mediante `worktic_lang`.
- Traducción de navegación, hero, beneficios, funciones, canales, onboarding, planes, FAQ, CTA y footer.
- Comportamiento responsive del selector de idioma.

La aplicación interna conserva su selector y sus traducciones existentes.
