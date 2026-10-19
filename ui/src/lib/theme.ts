// Встроенные темы (п. 6.4 плана). "system" следует prefers-color-scheme.
export const themePreferences = ['light', 'dark', 'system', 'high-contrast'] as const;

export type ThemePreference = (typeof themePreferences)[number];
export type Theme = Exclude<ThemePreference, 'system'>;

export function resolveTheme(preference: ThemePreference, prefersDark: boolean): Theme {
  if (preference === 'system') {
    return prefersDark ? 'dark' : 'light';
  }
  return preference;
}

// Сохранённое значение (localStorage, настройки сервера) может быть устаревшим или пустым
export function parseThemePreference(value: string | null): ThemePreference {
  return themePreferences.find((preference) => preference === value) ?? 'system';
}

export const themeStorageKey = 'wapp-debugger.theme';

export const themeLabels: Record<ThemePreference, string> = {
  light: 'Светлая',
  dark: 'Тёмная',
  system: 'Системная',
  'high-contrast': 'Контрастная',
};

/** Ставит data-theme на корневой элемент: от него зависят токены в theme.css. */
export function applyTheme(root: HTMLElement, preference: ThemePreference, prefersDark: boolean) {
  root.dataset.theme = resolveTheme(preference, prefersDark);
}

type ThemeStorage = Pick<Storage, 'getItem' | 'setItem'>;

// localStorage — только кэш для мгновенной загрузки (п. 10.25); он бывает недоступен
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
    // приватный режим или запрет хранилища: тема работает до перезагрузки
  }
}

export function systemPrefersDark(): boolean {
  return globalThis.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false;
}
