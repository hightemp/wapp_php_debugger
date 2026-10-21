import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defaultSettings } from '../lib/appearance';
import { emitCommand } from '../lib/commands';
import { i18n } from '../lib/i18n/locale.svelte';
import { createAppearanceStore } from '../lib/settings.svelte';
import { toasts } from '../lib/toasts.svelte';
import { fakeTransport } from '../test/transport';
import ThemeSwitch from './ThemeSwitch.svelte';

function setup(theme = 'system') {
  localStorage.setItem('wapp-debugger.settings', JSON.stringify({ ...defaultSettings, theme }));
  const store = createAppearanceStore({
    storage: localStorage,
    root: document.documentElement,
    media: () => ({ dark: false, moreContrast: false }),
  });
  const { transport } = fakeTransport();
  render(ThemeSwitch, { transport, store });
  return { store, transport };
}

describe('ThemeSwitch', () => {
  beforeEach(() => {
    localStorage.clear();
    toasts.clear();
    i18n.set('en');
    delete document.documentElement.dataset.theme;
  });

  it('is a radio group with all built-in themes', () => {
    setup();
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument();
    expect(screen.getAllByRole('radio').map((radio) => radio.textContent?.trim())).toEqual([
      'Light',
      'Dark',
      'System',
      'High contrast',
    ]);
    expect(screen.getByRole('radio', { name: 'System' })).toHaveAttribute('aria-checked', 'true');
  });

  it('applies the chosen theme and saves it on the viewer', async () => {
    const { transport } = setup();
    await fireEvent.click(screen.getByRole('radio', { name: 'Dark' }));
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(transport.updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ theme: 'dark' }),
    );
    expect(screen.getByRole('radio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true');
  });

  it('starts from the cached theme', () => {
    setup('light');
    expect(screen.getByRole('radio', { name: 'Light' })).toHaveAttribute('aria-checked', 'true');
  });

  it('takes the theme from the command palette', async () => {
    const { store } = setup();
    emitCommand({ type: 'theme', theme: 'high-contrast' });
    await vi.waitFor(() => expect(store.settings.theme).toBe('high-contrast'));
  });

  it('says when the viewer refuses the theme', async () => {
    const { transport } = setup();
    vi.mocked(transport.updateSettings).mockRejectedValueOnce(new Error('the viewer is gone'));
    await fireEvent.click(screen.getByRole('radio', { name: 'Dark' }));
    await vi.waitFor(() => expect(toasts.items[0]?.kind).toBe('error'));
    expect(document.documentElement.dataset.theme).toBe('light');
  });
});
