import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

// The viewer is served with script-src 'self' (server/internal/api/security.go): inline scripts would not run.
describe('index.html', () => {
  const html = readFileSync(resolve(process.cwd(), 'index.html'), 'utf8');

  it('has no inline scripts', () => {
    // Any case and spacing of the tags: HTML does not care
    const scripts = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script\s*>/gi)];
    expect(scripts.length).toBeGreaterThan(0);
    for (const [, attributes, body] of scripts) {
      expect(attributes).toMatch(/\bsrc=/);
      expect(body?.trim()).toBe('');
    }
  });

  it('sets the theme before the first paint with a blocking script', () => {
    expect(html).toMatch(/<head>[\s\S]*<script src="\/theme-init\.js"><\/script>[\s\S]*<\/head>/);
  });
});
