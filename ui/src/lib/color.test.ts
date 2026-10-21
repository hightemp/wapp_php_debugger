import { describe, expect, it } from 'vitest';
import { hexToHsl, hslToHex, normalizeHex } from './color';

describe('colors', () => {
  it.each([
    ['#ff0000', { h: 0, s: 100, l: 50 }],
    ['#000000', { h: 0, s: 0, l: 0 }],
    ['#ffffff', { h: 0, s: 0, l: 100 }],
    ['#2f6fed', { h: 220, s: 84, l: 56 }],
  ])('%s is %o in HSL', (hex, hsl) => {
    expect(hexToHsl(hex)).toEqual(hsl);
  });

  it('come back from HSL close to where they were', () => {
    for (const hex of ['#2f6fed', '#c62828', '#1d6b40', '#f6f7f9', '#0f1115']) {
      const back = hslToHex(hexToHsl(hex));
      const distance = [1, 3, 5].map((i) =>
        Math.abs(parseInt(back.slice(i, i + 2), 16) - parseInt(hex.slice(i, i + 2), 16)),
      );
      expect(Math.max(...distance)).toBeLessThanOrEqual(3);
    }
    expect(hslToHex({ h: 120, s: 100, l: 25 })).toBe('#008000');
  });

  it('read what people type', () => {
    expect(normalizeHex('#ABCDEF')).toBe('#abcdef');
    expect(normalizeHex(' abcdef ')).toBe('#abcdef');
    expect(normalizeHex('#abc')).toBe('#aabbcc');
    expect(normalizeHex('#abcd')).toBeUndefined();
    expect(normalizeHex('red')).toBeUndefined();
  });
});
