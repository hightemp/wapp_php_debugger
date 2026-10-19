import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import ThemeSwitch from './ThemeSwitch.svelte';

describe('ThemeSwitch', () => {
  beforeEach(() => {
    localStorage.clear();
    i18n.set('en');
    delete document.documentElement.dataset.theme;
  });

  it('is a radio group with all built-in themes', () => {
    render(ThemeSwitch);
    const group = screen.getByRole('radiogroup', { name: 'Theme' });
    expect(group).toBeInTheDocument();
    expect(screen.getAllByRole('radio').map((radio) => radio.textContent?.trim())).toEqual([
      'Light',
      'Dark',
      'System',
      'High contrast',
    ]);
    expect(screen.getByRole('radio', { name: 'System' })).toHaveAttribute('aria-checked', 'true');
  });

  it('applies and remembers the chosen theme', async () => {
    render(ThemeSwitch);
    await fireEvent.click(screen.getByRole('radio', { name: 'Dark' }));
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('wapp-debugger.theme')).toBe('dark');
    expect(screen.getByRole('radio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true');
  });

  it('starts from the remembered theme', () => {
    localStorage.setItem('wapp-debugger.theme', 'light');
    render(ThemeSwitch);
    expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'true');
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('moves selection with arrow keys', async () => {
    render(ThemeSwitch);
    await fireEvent.keyDown(screen.getByRole('radio', { name: 'System' }), {
      key: 'ArrowRight',
    });
    expect(screen.getByRole('radio', { name: 'High contrast' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    expect(document.documentElement.dataset.theme).toBe('high-contrast');
  });
});
