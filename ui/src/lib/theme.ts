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
