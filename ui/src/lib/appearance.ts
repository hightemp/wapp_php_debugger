// UI settings applied to the page: theme, accent, fonts and density. The viewer keeps the settings
// (GET/PUT /api/v1/settings); localStorage holds a copy so that public/theme-init.js paints the first
// frame right. theme-init.js repeats appearanceOf and applyAppearance: keep the two in step.
import type { CustomTheme, Settings } from './api/types';
import { colorTokens, themeBases, type ThemeBase } from './themeTokens';

export const defaultSettings: Settings = {
  theme: 'system',
  accent_color: null,
  density: 'comfortable',
  ui_font_size: 14,
  mono_font: null,
  env_frame: true,
  locale: null,
  custom_themes: [],
};

export interface Media {
  dark: boolean;
  moreContrast: boolean;
}

export interface Appearance {
  theme: ThemeBase;
  density: Settings['density'];
  /** CSS custom properties over the theme on the root element. */
  vars: Record<string, string>;
}

const monoFallback = 'ui-monospace, monospace';

export function customThemeOf(settings: Settings): CustomTheme | undefined {
  const id = settings.theme.startsWith('custom:') ? settings.theme.slice(7) : undefined;
  return settings.custom_themes.find((theme) => theme.id === id);
}

export function appearanceOf(settings: Settings, media: Media): Appearance {
  const custom = customThemeOf(settings);
  const vars: Record<string, string> = {};
  let theme: ThemeBase;
  if (custom) {
    theme = custom.base;
    for (const [name, value] of Object.entries(custom.tokens)) {
      vars[`--${name}`] = value;
    }
  } else if (
    settings.theme === 'light' ||
    settings.theme === 'dark' ||
    settings.theme === 'high-contrast'
  ) {
    theme = settings.theme;
  } else {
    // "system", or a custom theme that is gone
    theme = media.moreContrast ? 'high-contrast' : media.dark ? 'dark' : 'light';
  }
  if (settings.accent_color) {
    vars['--accent'] = settings.accent_color;
  }
  if (settings.ui_font_size !== defaultSettings.ui_font_size) {
    vars['--font-size'] = `${settings.ui_font_size}px`;
    vars['--font-size-small'] = `${settings.ui_font_size - 2}px`;
  }
  if (settings.mono_font) {
    vars['--font-mono'] = `${settings.mono_font}, ${monoFallback}`;
  }
  return { theme, density: settings.density, vars };
}

const applied = new WeakMap<HTMLElement, string[]>();

export function applyAppearance(root: HTMLElement, appearance: Appearance) {
  for (const name of applied.get(root) ?? []) {
    root.style.removeProperty(name);
  }
  for (const [name, value] of Object.entries(appearance.vars)) {
    root.style.setProperty(name, value);
  }
  applied.set(root, Object.keys(appearance.vars));
  root.dataset.theme = appearance.theme;
  root.dataset.density = appearance.density;
}

export function systemMedia(): Media {
  const matches = (query: string) => globalThis.matchMedia?.(query).matches ?? false;
  return {
    dark: matches('(prefers-color-scheme: dark)'),
    moreContrast: matches('(prefers-contrast: more)'),
  };
}

const hex = /^#[0-9a-fA-F]{6}$/;

/** Reads an exported theme file; the id may be taken and is replaced by the caller. */
export function parseCustomTheme(text: string): { theme: CustomTheme } | { error: string } {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return { error: 'not JSON' };
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return { error: 'not a theme object' };
  }
  const { id, name, base, tokens } = value as Record<string, unknown>;
  if (typeof name !== 'string' || name.trim() === '' || name.length > 60) {
    return { error: 'name: from 1 to 60 characters' };
  }
  if (!themeBases.includes(base as ThemeBase)) {
    return { error: `base: ${themeBases.join(', ')}` };
  }
  if (typeof tokens !== 'object' || tokens === null || Array.isArray(tokens)) {
    return { error: 'tokens: an object of colors' };
  }
  const known: Record<string, string> = {};
  for (const [token, color] of Object.entries(tokens)) {
    if (typeof color !== 'string' || !hex.test(color)) {
      return { error: `tokens.${token}: #rrggbb` };
    }
    // Tokens of a newer version are left out
    if (colorTokens.includes(token)) {
      known[token] = color;
    }
  }
  return {
    theme: {
      id: typeof id === 'string' && /^[a-z0-9-]{1,40}$/.test(id) ? id : themeId(name, []),
      name: name.trim(),
      base: base as ThemeBase,
      tokens: known,
    },
  };
}

export function exportCustomTheme(theme: CustomTheme): string {
  return `${JSON.stringify(theme, null, 2)}\n`;
}

/** A free id made of the name: lowercase latin letters, digits and dashes. */
export function themeId(name: string, taken: readonly string[]): string {
  const slug =
    name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
      .slice(0, 32) || 'theme';
  let id = slug;
  for (let n = 2; taken.includes(id); n++) {
    id = `${slug}-${n}`;
  }
  return id;
}

export const settingsStorageKey = 'wapp-debugger.settings';
const legacyThemeKey = 'wapp-debugger.theme';

type SettingsStorage = Pick<Storage, 'getItem' | 'setItem'>;

// The cache may be missing, broken, of an older version or blocked: the defaults are always there
export function loadCachedSettings(storage: SettingsStorage): Settings {
  try {
    const cached = storage.getItem(settingsStorageKey);
    if (cached !== null) {
      const value: unknown = JSON.parse(cached);
      return typeof value === 'object' && value !== null
        ? { ...defaultSettings, ...(value as Partial<Settings>) }
        : defaultSettings;
    }
    const theme = storage.getItem(legacyThemeKey);
    return theme ? { ...defaultSettings, theme } : defaultSettings;
  } catch {
    return defaultSettings;
  }
}

export function saveCachedSettings(storage: SettingsStorage, settings: Settings) {
  try {
    storage.setItem(settingsStorageKey, JSON.stringify(settings));
  } catch {
    // private mode or blocked storage: the server copy is enough
  }
}
