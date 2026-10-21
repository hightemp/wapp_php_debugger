import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

// The viewer is served with script-src 'self' (server/internal/api/security.go): inline scripts would not run.
describe('index.html', () => {
  const html = readFileSync(resolve(process.cwd(), 'index.html'), 'utf8');
  // Parsed as a browser does: tags in any case and spelling are found
  const page = new DOMParser().parseFromString(html, 'text/html');

  it('has no inline scripts', () => {
    const scripts = [...page.querySelectorAll('script')];
    expect(scripts.length).toBeGreaterThan(0);
    for (const script of scripts) {
      expect(script.getAttribute('src')).toBeTruthy();
      expect(script.textContent?.trim()).toBe('');
    }
  });

  it('sets the theme before the first paint with a blocking script', () => {
    const init = page.head.querySelector('script[src="/theme-init.js"]');
    expect(init).not.toBeNull();
    expect(init?.hasAttribute('async') || init?.hasAttribute('defer')).toBe(false);
  });
});
