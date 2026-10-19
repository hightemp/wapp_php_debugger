// Theme before the first paint, no flash. Same logic as src/lib/theme.ts.
// A separate file, not an inline script: the viewer is served with script-src 'self'.
(function () {
  var preference = 'system';
  try {
    preference = localStorage.getItem('wapp-debugger.theme') || 'system';
  } catch {
    // storage is blocked: keep the system theme
  }
  if (['light', 'dark', 'system', 'high-contrast'].indexOf(preference) < 0) {
    preference = 'system';
  }
  if (preference === 'system') {
    preference =
      window.matchMedia && matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  document.documentElement.dataset.theme = preference;
})();
