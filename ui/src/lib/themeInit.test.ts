import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import type { Settings } from './api/types';
import { appearanceOf, defaultSettings, saveCachedSettings } from './appearance';

// public/theme-init.js paints the first frame from the cached settings: it must agree with appearanceOf
const script = readFileSync(resolve(process.cwd(), 'public/theme-init.js'), 'utf8');

function run() {
  const root = document.documentElement;
  root.removeAttribute('style');
  delete root.dataset.theme;
  delete root.dataset.density;
  new Function(script)();
  const vars: Record<string, string> = {};
  for (const name of Array.from(root.style)) {
    vars[name] = root.style.getPropertyValue(name);
  }
  return { theme: root.dataset.theme, density: root.dataset.density, vars };
}

const media = { dark: false, moreContrast: false };

describe('theme-init.js', () => {
  afterEach(() => localStorage.clear());

  it.each<[string, Partial<Settings>]>([
    ['the defaults', {}],
    ['a built-in theme', { theme: 'high-contrast', density: 'compact' }],
    [
      'a custom theme with accent and fonts',
      {
        theme: 'custom:paper',
        custom_themes: [{ id: 'paper', name: 'Paper', base: 'dark', tokens: { bg: '#101010' } }],
        accent_color: '#d14d00',
        ui_font_size: 16,
        mono_font: "'Iosevka Term'",
      },
    ],
    ['a custom theme that is gone', { theme: 'custom:nord' }],
  ])('agrees with appearanceOf on %s', (_, change) => {
    const settings = { ...defaultSettings, ...change };
    saveCachedSettings(localStorage, settings);
    expect(run()).toEqual(appearanceOf(settings, media));
  });

  it('takes the theme of an older version, and survives a broken cache', () => {
    localStorage.setItem('wapp-debugger.theme', 'dark');
    expect(run().theme).toBe('dark');
    localStorage.setItem('wapp-debugger.settings', '{broken');
    expect(run()).toEqual(appearanceOf(defaultSettings, media));
  });
});
