// Built-in theme choices. "system" follows prefers-color-scheme and prefers-contrast; custom themes
// are "custom:<id>" (lib/appearance.ts).
export const themePreferences = ['light', 'dark', 'system', 'high-contrast'] as const;

export type ThemePreference = (typeof themePreferences)[number];
