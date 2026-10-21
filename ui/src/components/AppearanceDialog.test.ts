import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Settings } from '../lib/api/types';
import { defaultSettings } from '../lib/appearance';
import { i18n } from '../lib/i18n/locale.svelte';
import { createAppearanceStore } from '../lib/settings.svelte';
import { fakeTransport } from '../test/transport';
import AppearanceDialog from './AppearanceDialog.svelte';

function setup(settings: Partial<Settings> = {}) {
  localStorage.setItem(
    'wapp-debugger.settings',
    JSON.stringify({ ...defaultSettings, theme: 'light', ...settings }),
  );
  const root = document.documentElement;
  const store = createAppearanceStore({
    storage: localStorage,
    root,
    media: () => ({ dark: false, moreContrast: false }),
  });
  const fake = fakeTransport();
  const oneditthemes = vi.fn();
  render(AppearanceDialog, {
    open: true,
    transport: fake.transport,
    store,
    onclose: vi.fn(),
    oneditthemes,
  });
  const saved = () => vi.mocked(fake.transport.updateSettings).mock.lastCall?.[0];
  return { root, store, saved, oneditthemes, ...fake };
}

describe('AppearanceDialog', () => {
  beforeEach(() => {
    localStorage.clear();
    i18n.set('en');
  });

  it('chooses a built-in or a custom theme', async () => {
    const { saved } = setup({
      custom_themes: [{ id: 'paper', name: 'Paper', base: 'light', tokens: {} }],
    });
    await fireEvent.click(screen.getByRole('combobox', { name: 'Theme' }));
    await fireEvent.click(screen.getByRole('option', { name: /Paper/ }));
    await waitFor(() => expect(saved()?.theme).toBe('custom:paper'));
  });

  it('lays an accent over the theme and warns when it is hard to see', async () => {
    const { root, saved } = setup();
    await fireEvent.input(screen.getByRole('textbox', { name: 'Color of accent' }), {
      target: { value: '#d14d00' },
    });
    expect(root.style.getPropertyValue('--accent')).toBe('#d14d00');
    await waitFor(() => expect(saved()?.accent_color).toBe('#d14d00'));
    expect(screen.queryByText(/hard to see/)).not.toBeInTheDocument();

    await fireEvent.input(screen.getByRole('textbox', { name: 'Color of accent' }), {
      target: { value: '#fafafa' },
    });
    expect(screen.getByText(/hard to see on this theme/)).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Of the theme' }));
    expect(root.style.getPropertyValue('--accent')).toBe('');
    await waitFor(() => expect(saved()?.accent_color).toBeNull());
  });

  it('changes density, font size and the frame of environments', async () => {
    const { root, saved } = setup();
    await fireEvent.click(screen.getByRole('radio', { name: 'Compact' }));
    expect(root.dataset.density).toBe('compact');

    await fireEvent.click(screen.getByRole('combobox', { name: 'Font size' }));
    await fireEvent.click(screen.getByRole('option', { name: '16 px' }));
    expect(root.style.getPropertyValue('--font-size')).toBe('16px');

    await fireEvent.click(screen.getByRole('switch', { name: /Frame the window/ }));
    await waitFor(() =>
      expect(saved()).toMatchObject({ density: 'compact', ui_font_size: 16, env_frame: false }),
    );
  });

  it('takes a font of code that is only font names', async () => {
    const { root, transport } = setup();
    const field = screen.getByRole('textbox', { name: 'Font of code' });
    await fireEvent.input(field, { target: { value: 'x; } body {' } });
    await fireEvent.change(field);
    expect(field).toHaveAttribute('aria-invalid', 'true');
    expect(transport.updateSettings).not.toHaveBeenCalled();

    await fireEvent.input(field, { target: { value: "'JetBrains Mono'" } });
    await fireEvent.change(field);
    expect(root.style.getPropertyValue('--font-mono')).toBe(
      "'JetBrains Mono', ui-monospace, monospace",
    );
    await fireEvent.input(field, { target: { value: '' } });
    await fireEvent.change(field);
    expect(root.style.getPropertyValue('--font-mono')).toBe('');
  });

  it('opens the theme editor', async () => {
    const { oneditthemes } = setup();
    await fireEvent.click(screen.getByRole('button', { name: 'Theme editor…' }));
    expect(oneditthemes).toHaveBeenCalled();
  });
});
