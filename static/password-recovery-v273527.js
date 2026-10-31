/* Worktic AI V27.35.27 | Recovery UI for the approved login. No credential storage. */
(function () {
  'use strict';
  var card = document.querySelector('.auth-card');
  var login = document.getElementById('loginForm');
  var register = document.getElementById('registerForm');
  var tabs = document.querySelector('.tabs');
  var forgot = document.getElementById('wt25HelpPassword');
  if (!card || !login || !register || !tabs || !forgot) return;
  var recoveryInfo = document.getElementById('wt25RecoveryInfo');
  var security = document.querySelector('.wt25-security');
  var token = '';
  var currentStage = 'request';
  var busy = false;
  var sourceLink = window.location.hash.match(/(?:^#|&)reset=([^&]+)/);
  if (sourceLink) {
    try { token = decodeURIComponent(sourceLink[1]); } catch (_) { token = ''; }
    history.replaceState(null, '', window.location.pathname + window.location.search);
  }
  var dictionary = {
    es: {
      titleRequest: 'Recuperar contraseña', descRequest: 'Escribe el correo asociado a tu cuenta y te enviaremos un enlace seguro.',
      send: 'Enviar enlace de recuperación', email: 'Correo electrónico', back: 'Volver a ingresar',
      titleReset: 'Crea tu nueva contraseña', descReset: 'Elige una contraseña segura de al menos 12 caracteres.',
      pass: 'Nueva contraseña', confirm: 'Confirmar contraseña', save: 'Actualizar contraseña',
      sent: 'Revisa tu correo', sentDesc: 'Si tu cuenta está registrada, recibirás un enlace para recuperar el acceso. Revisa también spam.',
      success: 'Contraseña actualizada', successDesc: 'La contraseña se cambió. Ya puedes volver a iniciar sesión.',
      mismatch: 'Las contraseñas no coinciden.', minimum: 'Utiliza al menos 12 caracteres.',
      error: 'No fue posible completar la solicitud. Inténtalo de nuevo.', unavailable: 'La recuperación todavía no está habilitada en el servidor.',
      invalid: 'Enlace inválido o vencido. Solicita uno nuevo.', wait: 'Procesando…'
    },
    en: {
      titleRequest: 'Recover your password', descRequest: 'Enter your account email and we will send a secure recovery link.',
      send: 'Send recovery link', email: 'Email address', back: 'Back to sign in',
      titleReset: 'Create a new password', descReset: 'Choose a secure password with at least 12 characters.',
      pass: 'New password', confirm: 'Confirm password', save: 'Update password',
      sent: 'Check your email', sentDesc: 'If your account exists, you will receive a recovery link. Check your spam folder too.',
      success: 'Password updated', successDesc: 'Your password was changed. You can now sign in.',
      mismatch: 'Passwords do not match.', minimum: 'Use at least 12 characters.',
      error: 'The request could not be completed. Please retry.', unavailable: 'Recovery is not yet enabled on the server.',
      invalid: 'Invalid or expired link. Request a new one.', wait: 'Processing…'
    }
  };
  function language() { return document.documentElement.lang && document.documentElement.lang.toLowerCase().startsWith('en') ? 'en' : 'es'; }
  function t(k) { return dictionary[language()][k]; }
  var panel = document.createElement('section');
  panel.className = 'wt27-recovery';
  panel.id = 'wt27Recovery';
  panel.hidden = true;
  panel.setAttribute('aria-live', 'polite');
  panel.innerHTML = '<header class="wt27-recovery-head"><h3 id="wt27Title"></h3><p id="wt27Description"></p></header>' +
    '<form id="wt27RequestForm" class="wt27-form" autocomplete="on"><label><span id="wt27EmailLabel"></span><input id="wt27Email" name="email" type="email" autocomplete="email" placeholder="tu@correo.com" required maxlength="254"></label><button class="wt24-primary wt25-primary" type="submit" id="wt27RequestSubmit"></button></form>' +
    '<form id="wt27ResetForm" class="wt27-form" autocomplete="off" hidden><label><span id="wt27PasswordLabel"></span><input id="wt27Password" name="new-password" type="password" autocomplete="new-password" minlength="12" maxlength="128" required></label><label><span id="wt27ConfirmLabel"></span><input id="wt27Confirm" name="confirm-password" type="password" autocomplete="new-password" minlength="12" maxlength="128" required></label><button class="wt24-primary wt25-primary" type="submit" id="wt27ResetSubmit"></button></form>' +
    '<div id="wt27Status" class="wt27-status" role="status" hidden></div><p id="wt27Error" class="wt27-error" role="alert" hidden></p><button id="wt27Back" class="wt27-back" type="button"></button>';
  tabs.insertAdjacentElement('afterend', panel);
  function $id(id) { return document.getElementById(id); }
  var controls = [$id('wt27RequestSubmit'), $id('wt27ResetSubmit')];
  function messageError(msg) { var el = $id('wt27Error'); el.textContent = msg; el.hidden = false; }
  function clearMessages() { $id('wt27Error').hidden = true; $id('wt27Status').hidden = true; }
  function setBusy(value) { busy = value; controls.forEach(function (el) { el.disabled = busy; }); }
  function show(stage) {
    currentStage = stage;
    login.hidden = true;
    register.hidden = true;
    tabs.hidden = true;
    panel.hidden = false;
    if (security) security.hidden = true;
    if (recoveryInfo) recoveryInfo.hidden = true;
    clearMessages();
    var request = stage === 'request';
    var reset = stage === 'reset';
    $id('wt27RequestForm').hidden = !request;
    $id('wt27ResetForm').hidden = !reset;
    if (stage === 'sent' || stage === 'success') {
      $id('wt27Status').hidden = false;
      $id('wt27Status').textContent = t(stage === 'sent' ? 'sentDesc' : 'successDesc');
    }
    translate();
  }
  function translate() {
    var key = currentStage === 'reset' ? 'Reset' : currentStage === 'sent' ? 'Request' : currentStage === 'success' ? 'Reset' : 'Request';
    $id('wt27Title').textContent = t(currentStage === 'sent' ? 'sent' : currentStage === 'success' ? 'success' : 'title' + key);
    $id('wt27Description').textContent = currentStage === 'sent' || currentStage === 'success' ? '' : t('desc' + key);
    $id('wt27EmailLabel').textContent = t('email');
    $id('wt27PasswordLabel').textContent = t('pass');
    $id('wt27ConfirmLabel').textContent = t('confirm');
    $id('wt27RequestSubmit').textContent = busy ? t('wait') : t('send');
    $id('wt27ResetSubmit').textContent = busy ? t('wait') : t('save');
    $id('wt27Back').textContent = t('back');
    $id('wt27Email').placeholder = language() === 'es' ? 'tu@correo.com' : 'you@example.com';
    if (currentStage === 'sent' || currentStage === 'success') $id('wt27Status').textContent = t(currentStage === 'sent' ? 'sentDesc' : 'successDesc');
  }
  function back() {
    panel.hidden = true; tabs.hidden = false;
    if (security) security.hidden = false;
    clearMessages();
    document.getElementById('loginTab').click();
    login.hidden = false;
  }
  forgot.addEventListener('click', function (e) {
    e.preventDefault(); e.stopImmediatePropagation();
    $id('wt27Email').value = (login.querySelector('[name="email"]') || {}).value || '';
    show('request');
  }, true);
  $id('wt27Back').addEventListener('click', back);
  document.addEventListener('worktic:langchange', translate);
  document.querySelectorAll('[data-lang-switch]').forEach(function (el) { el.addEventListener('click', function () { setTimeout(translate, 40); }); });
  function post(endpoint, payload) {
    return fetch(endpoint, { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' }, body: JSON.stringify(payload), cache: 'no-store' }).then(async function (response) {
      var data = {}; try { data = await response.json(); } catch (_) { }
      if (!response.ok) { var error = new Error(response.status === 404 ? t('unavailable') : data.error || t('error')); throw error; }
      return data;
    });
  }
  $id('wt27RequestForm').addEventListener('submit', async function (e) {
    e.preventDefault(); if (busy) return;
    clearMessages(); setBusy(true); translate();
    try {
      await post('/api/auth/password/reset/request', { email: $id('wt27Email').value.trim() });
      show('sent');
    } catch (err) { messageError(err.message || t('error')); }
    finally { setBusy(false); translate(); }
  });
  $id('wt27ResetForm').addEventListener('submit', async function (e) {
    e.preventDefault(); if (busy) return;
    clearMessages();
    var pass = $id('wt27Password').value;
    if (pass.length < 12) { messageError(t('minimum')); return; }
    if (pass !== $id('wt27Confirm').value) { messageError(t('mismatch')); return; }
    setBusy(true); translate();
    try {
      await post('/api/auth/password/reset/confirm', { token: token, password: pass });
      token = ''; $id('wt27Password').value = ''; $id('wt27Confirm').value = '';
      show('success');
    } catch (err) { messageError(err.message || t('invalid')); }
    finally { setBusy(false); translate(); }
  });
  // Token in URL fragment is cleared before it reaches the server or browser history.
  if (token) show('reset');
})();
