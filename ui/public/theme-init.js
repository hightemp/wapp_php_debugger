// Appearance before the first paint, no flash: the settings cached by the UI (lib/appearance.ts,
// appearanceOf and applyAppearance; theme-init.test.ts keeps the two in step). A separate file,
// not an inline script: the viewer is served with script-src 'self'.
(function () {
  var settings = {};
  try {
    var cached = localStorage.getItem('wapp-debugger.settings');
    settings = cached
      ? JSON.parse(cached) || {}
      : { theme: localStorage.getItem('wapp-debugger.theme') };
  } catch {
    // storage is blocked or broken: the defaults
  }
  var media = function (query) {
    return !!(window.matchMedia && matchMedia(query).matches);
  };
  var root = document.documentElement;
  var theme = settings.theme;
  var custom = null;
  (settings.custom_themes || []).forEach(function (item) {
    if ('custom:' + item.id === theme) {
      custom = item;
    }
  });
  if (custom) {
    theme = custom.base;
    Object.keys(custom.tokens || {}).forEach(function (name) {
      root.style.setProperty('--' + name, custom.tokens[name]);
    });
  } else if (['light', 'dark', 'high-contrast'].indexOf(theme) < 0) {
    theme = media('(prefers-contrast: more)')
      ? 'high-contrast'
      : media('(prefers-color-scheme: dark)')
        ? 'dark'
        : 'light';
  }
  if (settings.accent_color) {
    root.style.setProperty('--accent', settings.accent_color);
  }
  if (settings.ui_font_size && settings.ui_font_size !== 14) {
    root.style.setProperty('--font-size', settings.ui_font_size + 'px');
    root.style.setProperty('--font-size-small', settings.ui_font_size - 2 + 'px');
  }
  if (settings.mono_font) {
    root.style.setProperty('--font-mono', settings.mono_font + ', ui-monospace, monospace');
  }
  root.dataset.theme = theme;
  root.dataset.density = settings.density || 'comfortable';
})();
