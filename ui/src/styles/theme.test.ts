import { describe, expect, it } from 'vitest';
import { contrast, contrastPairs } from '../lib/contrast';
import { builtInTokens, themeBases } from '../lib/themeTokens';

// The tokens are read from the real theme.css (lib/themeTokens.ts): what is checked is what ships.
describe.each(themeBases)('theme %s', (theme) => {
  const values = builtInTokens[theme];

  it.each(contrastPairs)('%s on %s has contrast ≥ %d', (foreground, background, minimum) => {
    const fg = values[foreground];
    const bg = values[background];
    expect(fg, `--${foreground}`).toBeDefined();
    expect(bg, `--${background}`).toBeDefined();
    expect(contrast(fg as string, bg as string)).toBeGreaterThanOrEqual(minimum);
  });
});

describe('theme.css', () => {
  it('defines the same tokens in every theme', () => {
    const names = themeBases.map((theme) => Object.keys(builtInTokens[theme]).sort().join(','));
    expect(new Set(names).size).toBe(1);
  });

  it('has the high contrast theme reach AAA for body text', () => {
    const values = builtInTokens['high-contrast'];
    expect(contrast(values.text as string, values.bg as string)).toBeGreaterThanOrEqual(7);
  });
});
