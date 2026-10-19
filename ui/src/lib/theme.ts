// Built-in themes. "system" follows prefers-color-scheme.
export const themePreferences = ['light', 'dark', 'system', 'high-contrast'] as const;

export type ThemePreference = (typeof themePreferences)[number];
export type Theme = Exclude<ThemePreference, 'system'>;

export function resolveTheme(preference: ThemePreference, prefersDark: boolean): Theme {
  if (preference === 'system') {
    return prefersDark ? 'dark' : 'light';
  }
  return preference;
}

// A stored value (localStorage, server settings) may be stale or empty
export function parseThemePreference(value: string | null): ThemePreference {
  return themePreferences.find((preference) => preference === value) ?? 'system';
}

export const themeStorageKey = 'wapp-debugger.theme';

/** Sets data-theme on the root element: the tokens in theme.css depend on it. */
export function applyTheme(root: HTMLElement, preference: ThemePreference, prefersDark: boolean) {
  root.dataset.theme = resolveTheme(preference, prefersDark);
}

type ThemeStorage = Pick<Storage, 'getItem' | 'setItem'>;

// localStorage is only a cache for instant startup; it may be unavailable
export function loadThemePreference(storage: ThemeStorage): ThemePreference {
  try {
    return parseThemePreference(storage.getItem(themeStorageKey));
  } catch {
    return 'system';
  }
}

export function saveThemePreference(storage: ThemeStorage, preference: ThemePreference) {
  try {
    storage.setItem(themeStorageKey, preference);
  } catch {
    // private mode or blocked storage: the theme lasts until reload
  }
}

export function systemPrefersDark(): boolean {
  return globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false;
}
