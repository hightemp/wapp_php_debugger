// WCAG contrast of theme colors: the tests of built-in themes and the theme editor use the same pairs.

function luminance(hex: string): number {
  const channels = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  const [r = 0, g = 0, b = 0] = channels.map((c) =>
    c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4,
  );
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

export function contrast(a: string, b: string): number {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (light + 0.05) / (dark + 0.05);
}

// [color, background, minimum contrast]: 4.5 for text, 3 for large and non-text elements (WCAG AA)
export const contrastPairs: readonly [string, string, number][] = [
  ['text', 'bg', 4.5],
  ['text', 'surface', 4.5],
  ['text-muted', 'surface', 4.5],
  ['text-muted', 'surface-hover', 4.5],
  ['level-error', 'surface', 4.5],
  ['level-warning', 'surface', 4.5],
  ['level-info', 'surface', 4.5],
  ['level-error', 'level-error-soft', 4.5],
  ['level-warning', 'level-warning-soft', 4.5],
  ['syntax-string', 'surface', 4.5],
  ['syntax-number', 'surface', 4.5],
  ['syntax-keyword', 'surface', 4.5],
  ['syntax-null', 'surface', 4.5],
  ['env-dev', 'surface', 4.5],
  ['env-stage', 'surface', 4.5],
  ['env-prod', 'surface', 4.5],
  ['accent', 'surface', 3],
  ['border', 'surface', 1.2],
];

export interface ContrastProblem {
  foreground: string;
  background: string;
  ratio: number;
  minimum: number;
}

/** The pairs of tokens (names without dashes) below their minimum contrast. */
export function contrastProblems(tokens: Record<string, string>): ContrastProblem[] {
  const problems: ContrastProblem[] = [];
  for (const [foreground, background, minimum] of contrastPairs) {
    const fg = tokens[foreground];
    const bg = tokens[background];
    if (fg && bg) {
      const ratio = contrast(fg, bg);
      if (ratio < minimum) {
        problems.push({ foreground, background, ratio, minimum });
      }
    }
  }
  return problems;
}
