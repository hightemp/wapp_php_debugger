import { describe, expect, it } from 'vitest';
import { contrast, contrastProblems } from './contrast';
import { builtInTokens } from './themeTokens';

describe('contrast', () => {
  it('follows WCAG: black on white is 21, a color on itself is 1', () => {
    expect(contrast('#000000', '#ffffff')).toBeCloseTo(21);
    expect(contrast('#ffffff', '#000000')).toBeCloseTo(21);
    expect(contrast('#2f6fed', '#2f6fed')).toBe(1);
  });

  it('finds no problems in a built-in theme', () => {
    expect(contrastProblems(builtInTokens.light)).toEqual([]);
  });

  it('names the pairs that are too faint', () => {
    const problems = contrastProblems({ ...builtInTokens.light, text: '#e0e0e0' });
    expect(problems.map((p) => `${p.foreground}/${p.background}`)).toEqual([
      'text/bg',
      'text/surface',
    ]);
    expect(problems[0]?.minimum).toBe(4.5);
    expect(problems[0]?.ratio).toBeLessThan(4.5);
  });
});
