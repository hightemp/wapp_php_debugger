import { describe, expect, it } from 'vitest';
import { builtInTokens, colorTokens, themeBases } from './themeTokens';

describe('theme tokens', () => {
  it('are read from theme.css', () => {
    expect(builtInTokens.light.bg).toBe('#f6f7f9');
    expect(builtInTokens['high-contrast'].accent).toBe('#ffff00');
  });

  it('are the colors every base theme defines, without shadows and sizes', () => {
    expect(colorTokens).toContain('accent');
    expect(colorTokens).toContain('level-error');
    expect(colorTokens).not.toContain('shadow-small');
    for (const base of themeBases) {
      expect(Object.keys(builtInTokens[base]).sort()).toEqual([...colorTokens].sort());
    }
  });
});
