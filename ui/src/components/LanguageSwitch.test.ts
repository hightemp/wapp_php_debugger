import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import LanguageSwitch from './LanguageSwitch.svelte';
import ThemeSwitch from './ThemeSwitch.svelte';

describe('LanguageSwitch', () => {
  beforeEach(() => {
    localStorage.clear();
    i18n.set('en');
  });

  it('offers English and Russian, English by default', () => {
    render(LanguageSwitch);
    expect(screen.getByRole('radiogroup', { name: 'Language' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'English' })).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByRole('radio', { name: 'Русский' })).toHaveAttribute('aria-checked', 'false');
  });

  it('switches the whole UI, the document language and remembers the choice', async () => {
    render(LanguageSwitch);
    render(ThemeSwitch);
    await fireEvent.click(screen.getByRole('radio', { name: 'Русский' }));

    expect(document.documentElement.lang).toBe('ru');
    expect(localStorage.getItem('wapp-debugger.locale')).toBe('ru');
    expect(screen.getByRole('radiogroup', { name: 'Язык' })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: 'Тёмная' })).toBeInTheDocument();
  });
});
