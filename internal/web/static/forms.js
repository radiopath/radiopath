function cpAntennaGain() {
  document.querySelectorAll('select[data-gain-field]').forEach(function (sel) {
    var input = document.querySelector('[name="' + sel.dataset.gainField + '"]');
    if (!input) { return; }

    function apply() {
      var gain = sel.options[sel.selectedIndex].dataset.gain;
      if (gain === undefined) {
        input.readOnly = false;
        if (input.dataset.manual !== undefined) {
          input.value = input.dataset.manual;
          delete input.dataset.manual;
        }
        return;
      }
      if (!input.readOnly) { input.dataset.manual = input.value; }
      input.value = gain;
      input.readOnly = true;
    }

    sel.addEventListener('change', apply);
    apply();
  });
}

function cpPassword() {
  var alpha = 'abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!#%*+-?';
  for (;;) {
    var bytes = new Uint8Array(20);
    crypto.getRandomValues(bytes);
    var out = '';
    for (var i = 0; i < bytes.length; i++) { out += alpha[bytes[i] & 63]; }
    if (/[a-z]/.test(out) && /[A-Z]/.test(out) && /[2-9]/.test(out) && /[!#%*+\-?]/.test(out)) {
      return out;
    }
  }
}

function cpPasswordFields() {
  document.querySelectorAll('button[data-generate]').forEach(function (btn) {
    var input = document.getElementById(btn.dataset.generate);
    if (!input) { return; }
    if (!input.value) { input.value = cpPassword(); }
    btn.addEventListener('click', function () {
      input.value = cpPassword();
      input.focus();
      input.select();
    });
  });
}

function cpCopyFields() {
  document.querySelectorAll('button[data-copy]').forEach(function (btn) {
    var input = document.getElementById(btn.dataset.copy);
    if (!input) { return; }
    btn.addEventListener('click', function () {
      input.focus();
      input.select();
      if (!navigator.clipboard) { return; }
      navigator.clipboard.writeText(input.value).then(function () {
        btn.textContent = 'Copied';
        setTimeout(function () { btn.textContent = 'Copy'; }, 1500);
      });
    });
  });
}

function cpDialogs() {
  var key = 'cp-dialog';

  document.querySelectorAll('button[data-dialog]').forEach(function (btn) {
    var dlg = document.getElementById(btn.dataset.dialog);
    if (!dlg) { return; }
    btn.addEventListener('click', function () { dlg.showModal(); });
    dlg.addEventListener('click', function (e) { if (e.target === dlg) { dlg.close(); } });
    if (dlg.dataset.reopen !== undefined) {
      dlg.addEventListener('submit', function () {
        try { sessionStorage.setItem(key, dlg.id); } catch (e) { }
      });
    }
  });
  document.querySelectorAll('button[data-close]').forEach(function (btn) {
    var dlg = document.getElementById(btn.dataset.close);
    if (dlg) { btn.addEventListener('click', function () { dlg.close(); }); }
  });

  var again = null;
  try {
    again = sessionStorage.getItem(key);
    sessionStorage.removeItem(key);
  } catch (e) { }
  if (again) {
    var dlg = document.getElementById(again);
    if (dlg && dlg.showModal) { dlg.showModal(); }
  }
  document.querySelectorAll('dialog[data-open]').forEach(function (dlg) {
    if (dlg.showModal && !dlg.open) { dlg.showModal(); }
  });
}
