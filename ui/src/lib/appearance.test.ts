import { describe, expect, it } from 'vitest';
import type { Settings } from './api/types';
import {
  applyAppearance,
  appearanceOf,
  defaultSettings,
  exportCustomTheme,
  loadCachedSettings,
  parseCustomTheme,
  saveCachedSettings,
  themeId,
} from './appearance';

const light = { dark: false, moreContrast: false };
const paper = {
  id: 'paper',
  name: 'Paper',
  base: 'light' as const,
  tokens: { bg: '#fdf6e3', accent: '#268bd2' },
};

function settings(change: Partial<Settings>): Settings {
  return { ...defaultSettings, ...change };
}

describe('appearanceOf', () => {
  it('resolves the system theme from the color scheme and contrast of the OS', () => {
    expect(appearanceOf(defaultSettings, light).theme).toBe('light');
    expect(appearanceOf(defaultSettings, { dark: true, moreContrast: false }).theme).toBe('dark');
    expect(appearanceOf(defaultSettings, { dark: false, moreContrast: true }).theme).toBe(
      'high-contrast',
    );
    expect(
      appearanceOf(settings({ theme: 'light' }), { dark: true, moreContrast: true }).theme,
    ).toBe('light');
  });

  it('puts a custom theme over its base', () => {
    const result = appearanceOf(settings({ theme: 'custom:paper', custom_themes: [paper] }), light);
    expect(result.theme).toBe('light');
    expect(result.vars).toEqual({ '--bg': '#fdf6e3', '--accent': '#268bd2' });
  });

  it('falls back to the system theme when the custom one is gone', () => {
    expect(
      appearanceOf(settings({ theme: 'custom:nord' }), { dark: true, moreContrast: false }),
    ).toMatchObject({ theme: 'dark', vars: {} });
  });

  it('lays accent, fonts and density over the theme', () => {
    const result = appearanceOf(
      settings({
        theme: 'custom:paper',
        custom_themes: [paper],
        accent_color: '#d14d00',
        ui_font_size: 16,
        mono_font: "'Iosevka Term'",
        density: 'compact',
      }),
      light,
    );
    expect(result.density).toBe('compact');
    expect(result.vars).toMatchObject({
      '--accent': '#d14d00',
      '--font-size': '16px',
      '--font-size-small': '14px',
      '--font-mono': "'Iosevka Term', ui-monospace, monospace",
    });
  });

  it('leaves the defaults to theme.css', () => {
    expect(appearanceOf(defaultSettings, light)).toEqual({
      theme: 'light',
      density: 'comfortable',
      vars: {},
    });
  });
});

describe('applyAppearance', () => {
  it('sets the theme, density and variables, and takes back the old ones', () => {
    const root = document.createElement('html');
    applyAppearance(root, { theme: 'dark', density: 'compact', vars: { '--accent': '#d14d00' } });
    expect(root.dataset.theme).toBe('dark');
    expect(root.dataset.density).toBe('compact');
    expect(root.style.getPropertyValue('--accent')).toBe('#d14d00');

    applyAppearance(root, { theme: 'light', density: 'comfortable', vars: { '--bg': '#ffffff' } });
    expect(root.style.getPropertyValue('--accent')).toBe('');
    expect(root.style.getPropertyValue('--bg')).toBe('#ffffff');
  });
});

describe('custom theme files', () => {
  it('round-trip through export and import', () => {
    const parsed = parseCustomTheme(exportCustomTheme(paper));
    expect(parsed).toEqual({ theme: paper });
  });

  it('drop tokens this version does not know', () => {
    const parsed = parseCustomTheme(
      JSON.stringify({ ...paper, tokens: { bg: '#000000', 'from-the-future': '#111111' } }),
    );
    expect(parsed).toEqual({ theme: { ...paper, tokens: { bg: '#000000' } } });
  });

  it.each([
    ['not json', '{'],
    ['not an object', '[]'],
    ['no name', JSON.stringify({ ...paper, name: '' })],
    ['a base that is not one', JSON.stringify({ ...paper, base: 'system' })],
    ['a color that is not one', JSON.stringify({ ...paper, tokens: { bg: 'url(x)' } })],
  ])('refuse %s', (_, text) => {
    expect(parseCustomTheme(text)).toHaveProperty('error');
  });

  it('get an id of their own', () => {
    expect(themeId('Solarized Light!', [])).toBe('solarized-light');
    expect(themeId('Paper', ['paper', 'paper-2'])).toBe('paper-3');
    expect(themeId('Тема', [])).toBe('theme');
  });
});

describe('settings cache', () => {
  function memory() {
    const values = new Map<string, string>();
    return {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => void values.set(key, value),
      values,
    };
  }

  it('keeps the settings for the first paint', () => {
    const storage = memory();
    expect(loadCachedSettings(storage)).toEqual(defaultSettings);
    const saved = settings({ theme: 'custom:paper', custom_themes: [paper], density: 'compact' });
    saveCachedSettings(storage, saved);
    expect(loadCachedSettings(storage)).toEqual(saved);
  });

  it('takes the theme of an older version and survives broken values', () => {
    const storage = memory();
    storage.setItem('wapp-debugger.theme', 'dark');
    expect(loadCachedSettings(storage).theme).toBe('dark');
    storage.setItem('wapp-debugger.settings', '{broken');
    expect(loadCachedSettings(storage)).toEqual(defaultSettings);
    const denied = {
      getItem: () => {
        throw new Error('denied');
      },
      setItem: () => {
        throw new Error('denied');
      },
    };
    expect(loadCachedSettings(denied)).toEqual(defaultSettings);
    expect(() => saveCachedSettings(denied, defaultSettings)).not.toThrow();
  });
});
