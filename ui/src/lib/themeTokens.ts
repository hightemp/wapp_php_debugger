// Colors of the built-in themes, read from theme.css itself: the editor starts from what ships.
import css from '../styles/theme.css?raw';

export const themeBases = ['light', 'dark', 'high-contrast'] as const;
export type ThemeBase = (typeof themeBases)[number];

function parse(theme: ThemeBase): Record<string, string> {
  const block = new RegExp(`\\[data-theme='${theme}'\\]\\s*\\{([^}]*)\\}`).exec(css)?.[1] ?? '';
  return Object.fromEntries(
    [...block.matchAll(/--([\w-]+):\s*(#[0-9a-fA-F]{6})\s*;/g)].map((m) => [m[1], m[2]]),
  );
}

export const builtInTokens = Object.fromEntries(
  themeBases.map((base) => [base, parse(base)]),
) as Record<ThemeBase, Record<string, string>>;

/** Names of the color tokens, without the leading dashes, in the order of theme.css. */
export const colorTokens: readonly string[] = Object.keys(builtInTokens.light);
