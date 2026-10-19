// @vitest-environment node
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { themePreferences } from '../lib/theme';

// Токены тем читаются из настоящего theme.css: что проверено — то и в сборке.
const css = readFileSync(new URL('./theme.css', import.meta.url), 'utf8');

function tokens(theme: string): Record<string, string> {
  const block = new RegExp(`\\[data-theme='${theme}'\\]\\s*\\{([^}]*)\\}`).exec(css)?.[1];
  if (!block) {
    throw new Error(`theme ${theme} not found in theme.css`);
  }
  return Object.fromEntries(
    [...block.matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6})\s*;/g)].map((m) => [m[1], m[2]]),
  );
}

function luminance(hex: string): number {
  const channels = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  const [r = 0, g = 0, b = 0] = channels.map((c) =>
    c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4,
  );
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (light + 0.05) / (dark + 0.05);
}

// [цвет, фон, минимальный контраст]: 4.5 — текст, 3 — крупные и неречевые элементы (WCAG AA)
const pairs: [string, string, number][] = [
  ['text', 'bg', 4.5],
  ['text', 'surface', 4.5],
  ['text-muted', 'surface', 4.5],
  ['text-muted', 'surface-hover', 4.5],
  ['level-error', 'surface', 4.5],
  ['level-warning', 'surface', 4.5],
  ['level-info', 'surface', 4.5],
  ['level-error', 'level-error-soft', 4.5],
  ['level-warning', 'level-warning-soft', 4.5],
  ['accent', 'surface', 3],
  ['border', 'surface', 1.2],
];

const builtIn = themePreferences.filter((theme) => theme !== 'system');

describe.each(builtIn)('theme %s', (theme) => {
  const values = tokens(theme);

  it.each(pairs)('%s on %s has contrast ≥ %d', (foreground, background, minimum) => {
    const fg = values[foreground];
    const bg = values[background];
    expect(fg, `--${foreground}`).toBeDefined();
    expect(bg, `--${background}`).toBeDefined();
    expect(contrast(fg as string, bg as string)).toBeGreaterThanOrEqual(minimum);
  });
});

describe('theme.css', () => {
  it('defines the same tokens in every theme', () => {
    const names = builtIn.map((theme) => Object.keys(tokens(theme)).sort().join(','));
    expect(new Set(names).size).toBe(1);
  });

  it('has the high contrast theme reach AAA for body text', () => {
    const values = tokens('high-contrast');
    expect(contrast(values.text as string, values.bg as string)).toBeGreaterThanOrEqual(7);
  });
});
