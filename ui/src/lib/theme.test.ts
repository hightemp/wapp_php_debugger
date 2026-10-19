import { describe, expect, it } from 'vitest';
import { parseThemePreference, resolveTheme } from './theme';

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
