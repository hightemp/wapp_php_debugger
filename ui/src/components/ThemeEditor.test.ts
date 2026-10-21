import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { CustomTheme, Settings } from '../lib/api/types';
import { defaultSettings } from '../lib/appearance';
import { i18n } from '../lib/i18n/locale.svelte';
import { createAppearanceStore } from '../lib/settings.svelte';
import { toasts } from '../lib/toasts.svelte';
import { builtInTokens } from '../lib/themeTokens';
import { fakeTransport } from '../test/transport';
import ThemeEditor from './ThemeEditor.svelte';

const paper: CustomTheme = {
  id: 'paper',
  name: 'Paper',
  base: 'light',
  tokens: { bg: '#fdf6e3' },
};

function setup(settings: Partial<Settings> = {}) {
  localStorage.setItem(
    'wapp-debugger.settings',
    JSON.stringify({ ...defaultSettings, ...settings }),
  );
  const store = createAppearanceStore({
    storage: localStorage,
    root: document.documentElement,
    media: () => ({ dark: true, moreContrast: false }),
  });
  const fake = fakeTransport();
  const onclose = vi.fn();
  render(ThemeEditor, { open: true, transport: fake.transport, store, onclose });
  return { store, onclose, ...fake };
}

const dialog = () => screen.getByRole('dialog', { name: 'Theme editor' });
const preview = () => screen.getByRole('region', { name: 'Preview' });

describe('ThemeEditor', () => {
  beforeEach(() => {
    localStorage.clear();
    toasts.clear();
    i18n.set('en');
  });

  it('starts a new theme from the theme in use and previews every change', async () => {
    const { transport, store } = setup();
    expect(screen.getByRole('radio', { name: 'Dark' })).toHaveAttribute('aria-checked', 'true');
    expect(preview()).toHaveAttribute('data-theme', 'dark');
    const field = screen.getByRole('textbox', { name: 'Color of accent' });
    expect(field).toHaveValue(builtInTokens.dark.accent);

    await fireEvent.input(field, { target: { value: '#ff8800' } });
    expect(preview().style.getPropertyValue('--accent')).toBe('#ff8800');
    expect(within(dialog()).getByText('changed')).toBeInTheDocument();

    await fireEvent.input(screen.getByRole('textbox', { name: 'Name' }), {
      target: { value: 'Night owl' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Save and use' }));
    await waitFor(() =>
      expect(transport.updateSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          theme: 'custom:night-owl',
          custom_themes: [
            { id: 'night-owl', name: 'Night owl', base: 'dark', tokens: { accent: '#ff8800' } },
          ],
        }),
      ),
    );
    expect(store.settings.theme).toBe('custom:night-owl');
  });

  it('warns about colors that are hard to read', async () => {
    setup({ theme: 'light' });
    expect(screen.getByRole('status')).toHaveTextContent('Every pair of colors is readable');
    await fireEvent.input(screen.getByRole('textbox', { name: 'Color of text' }), {
      target: { value: '#e0e0e0' },
    });
    expect(screen.getByRole('status')).toHaveTextContent('text on bg');
  });

  it('puts a token back to the color of the base', async () => {
    setup({ theme: 'custom:paper', custom_themes: [paper] });
    expect(screen.getByRole('textbox', { name: 'Color of bg' })).toHaveValue('#fdf6e3');
    await fireEvent.click(screen.getByRole('button', { name: 'Back to the base color of bg' }));
    expect(screen.getByRole('textbox', { name: 'Color of bg' })).toHaveValue(
      builtInTokens.light.bg,
    );
    expect(within(dialog()).queryByText('changed')).not.toBeInTheDocument();
  });

  it('edits the custom theme in use and deletes it after a question', async () => {
    const { transport, store } = setup({ theme: 'custom:paper', custom_themes: [paper] });
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('Paper');
    await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    const confirm = screen.getByRole('alertdialog', { name: 'Delete the theme Paper?' });
    await fireEvent.click(within(confirm).getByRole('button', { name: 'Delete' }));
    await waitFor(() =>
      expect(transport.updateSettings).toHaveBeenCalledWith(
        expect.objectContaining({ theme: 'system', custom_themes: [] }),
      ),
    );
    expect(store.settings.custom_themes).toEqual([]);
  });

  it('imports a theme file under a free id, and refuses what is not one', async () => {
    setup({ custom_themes: [paper] });
    const input = screen.getByLabelText('Import…');
    const file = new File(
      [JSON.stringify({ ...paper, tokens: { accent: '#123456' } })],
      'paper.json',
    );
    await fireEvent.change(input, { target: { files: [file] } });
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('Paper'));
    expect(screen.getByRole('textbox', { name: 'Color of accent' })).toHaveValue('#123456');
    expect(toasts.items.at(-1)?.text).toBe('Theme Paper is imported: check it and save');

    await fireEvent.change(input, { target: { files: [new File(['{'], 'broken.json')] } });
    await waitFor(() => expect(toasts.items.at(-1)?.text).toBe('Not a theme file: not JSON'));
  });

  it('exports the theme as a file', async () => {
    setup({ theme: 'custom:paper', custom_themes: [paper] });
    const createObjectURL = vi.fn(() => 'blob:theme');
    const revokeObjectURL = vi.fn();
    Object.assign(URL, { createObjectURL, revokeObjectURL });
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    await fireEvent.click(screen.getByRole('button', { name: 'Export' }));
    const blob = (createObjectURL.mock.calls[0] as unknown as [Blob])[0];
    expect(JSON.parse(await blob.text())).toEqual(paper);
    expect((click.mock.contexts[0] as HTMLAnchorElement).download).toBe('paper.theme.json');
    click.mockRestore();
  });
});
