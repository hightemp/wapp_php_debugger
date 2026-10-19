import { describe, expect, it } from 'vitest';
import {
  applyTheme,
  loadThemePreference,
  parseThemePreference,
  resolveTheme,
  saveThemePreference,
} from './theme';

describe('resolveTheme', () => {
  it('returns explicit themes as is', () => {
    expect(resolveTheme('light', true)).toBe('light');
    expect(resolveTheme('dark', false)).toBe('dark');
    expect(resolveTheme('high-contrast', false)).toBe('high-contrast');
  });

  it('follows the system for "system"', () => {
    expect(resolveTheme('system', true)).toBe('dark');
    expect(resolveTheme('system', false)).toBe('light');
  });
});

describe('parseThemePreference', () => {
  it('accepts known preferences', () => {
    for (const value of ['light', 'dark', 'system', 'high-contrast']) {
      expect(parseThemePreference(value)).toBe(value);
    }
  });

  it('falls back to "system" for missing or unknown values', () => {
    expect(parseThemePreference(null)).toBe('system');
    expect(parseThemePreference('')).toBe('system');
    expect(parseThemePreference('solarized')).toBe('system');
  });
});

describe('theme storage and application', () => {
  it('applies the resolved theme to the root element', () => {
    const root = document.createElement('html');
    applyTheme(root, 'system', true);
    expect(root.dataset.theme).toBe('dark');
    applyTheme(root, 'high-contrast', true);
    expect(root.dataset.theme).toBe('high-contrast');
  });

  it('remembers the preference and survives broken storage', () => {
    const storage = new Map<string, string>();
    const fake = {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => void storage.set(key, value),
    };
    expect(loadThemePreference(fake)).toBe('system');
    saveThemePreference(fake, 'dark');
    expect(loadThemePreference(fake)).toBe('dark');

    const broken = {
      getItem: () => {
        throw new Error('denied');
      },
      setItem: () => {
        throw new Error('denied');
      },
    };
    expect(loadThemePreference(broken)).toBe('system');
    expect(() => saveThemePreference(broken, 'dark')).not.toThrow();
  });
});
