function cpTheme(btn) {
  function current() { return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'; }
  function sync() { btn.setAttribute('aria-pressed', current() === 'dark'); }
  btn.addEventListener('click', function () {
    var next = current() === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = next;
    try { localStorage.setItem('cp-theme', next); } catch (e) { }
    sync();
  });
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function (e) {
    try { if (localStorage.getItem('cp-theme')) return; } catch (err) { return; }
    document.documentElement.dataset.theme = e.matches ? 'dark' : 'light';
    sync();
  });
  sync();
}
