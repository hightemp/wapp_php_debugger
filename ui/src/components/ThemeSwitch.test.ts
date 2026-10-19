import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it } from 'vitest';
import ThemeSwitch from './ThemeSwitch.svelte';

describe('ThemeSwitch', () => {
  beforeEach(() => {
    localStorage.clear();
    delete document.documentElement.dataset.theme;
  });

  it('is a radio group with all built-in themes', () => {
    render(ThemeSwitch);
    const group = screen.getByRole('radiogroup', { name: 'Тема' });
    expect(group).toBeInTheDocument();
    expect(screen.getAllByRole('radio').map((radio) => radio.textContent?.trim())).toEqual([
      'Светлая',
      'Тёмная',
      'Системная',
      'Контрастная',
    ]);
    expect(screen.getByRole('radio', { name: 'Системная' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
  });

  it('applies and remembers the chosen theme', async () => {
    render(ThemeSwitch);
    await fireEvent.click(screen.getByRole('radio', { name: 'Тёмная' }));
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('wapp-debugger.theme')).toBe('dark');
    expect(screen.getByRole('radio', { name: 'Тёмная' })).toHaveAttribute('aria-checked', 'true');
  });

  it('starts from the remembered theme', () => {
    localStorage.setItem('wapp-debugger.theme', 'light');
    render(ThemeSwitch);
    expect(screen.getByRole('radio', { name: 'Светлая' })).toHaveAttribute('aria-checked', 'true');
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('moves selection with arrow keys', async () => {
    render(ThemeSwitch);
    await fireEvent.keyDown(screen.getByRole('radio', { name: 'Системная' }), {
      key: 'ArrowRight',
    });
    expect(screen.getByRole('radio', { name: 'Контрастная' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    expect(document.documentElement.dataset.theme).toBe('high-contrast');
  });
});
