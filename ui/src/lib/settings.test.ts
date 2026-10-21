import { describe, expect, it, vi } from 'vitest';
import { fakeTransport } from '../test/transport';
import type { Settings } from './api/types';
import { defaultSettings, loadCachedSettings, saveCachedSettings } from './appearance';
import { createAppearanceStore } from './settings.svelte';

function memory() {
  const values = new Map<string, string>();
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
  };
}

function setup(saved: Partial<Settings> = {}) {
  const storage = memory();
  const root = document.createElement('html');
  const fake = fakeTransport({
    getSettings: vi.fn(async () => ({ ...defaultSettings, ...saved })),
  });
  const store = createAppearanceStore({
    storage,
    root,
    media: () => ({ dark: false, moreContrast: false }),
  });
  return { storage, root, store, ...fake };
}

describe('appearance store', () => {
  it('starts from the cache, then takes the settings of the viewer', async () => {
    const { storage, root, store, transport } = setup({ theme: 'dark', density: 'compact' });
    saveCachedSettings(storage, { ...defaultSettings, theme: 'high-contrast' });
    const cached = createAppearanceStore({
      storage,
      root,
      media: () => ({ dark: false, moreContrast: false }),
    });
    expect(cached.settings.theme).toBe('high-contrast');

    await store.load(transport);
    expect(store.settings.theme).toBe('dark');
    expect(root.dataset.theme).toBe('dark');
    expect(root.dataset.density).toBe('compact');
    expect(loadCachedSettings(storage).theme).toBe('dark');
  });

  it('applies a change at once and saves it on the viewer', async () => {
    const { root, store, transport } = setup();
    const saving = store.update(transport, { theme: 'dark', accent_color: '#d14d00' });
    expect(root.dataset.theme).toBe('dark');
    expect(root.style.getPropertyValue('--accent')).toBe('#d14d00');
    await saving;
    expect(transport.updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ theme: 'dark', accent_color: '#d14d00' }),
    );
  });

  it('takes the change back when the viewer refuses it', async () => {
    const { root, store, transport } = setup();
    vi.mocked(transport.updateSettings).mockRejectedValueOnce(new Error('refused'));
    await expect(store.update(transport, { theme: 'dark' })).rejects.toThrow('refused');
    expect(store.settings.theme).toBe('system');
    expect(root.dataset.theme).toBe('light');
  });

  it('follows saves of other windows, but not while its own save is on the way', async () => {
    const { root, store, transport, emit } = setup();
    const stop = store.connect(transport);
    await vi.waitFor(() => expect(transport.getSettings).toHaveBeenCalled());

    emit({ event: 'settings_updated', settings: { ...defaultSettings, theme: 'high-contrast' } });
    expect(root.dataset.theme).toBe('high-contrast');

    let finish: (settings: Settings) => void = () => {};
    vi.mocked(transport.updateSettings).mockImplementationOnce(
      () => new Promise<Settings>((resolve) => (finish = resolve)),
    );
    const saving = store.update(transport, { theme: 'dark' });
    // The echo of an older save must not flip the theme back
    emit({ event: 'settings_updated', settings: { ...defaultSettings, theme: 'light' } });
    expect(root.dataset.theme).toBe('dark');
    finish({ ...defaultSettings, theme: 'dark' });
    await saving;
    expect(root.dataset.theme).toBe('dark');

    stop();
    emit({ event: 'settings_updated', settings: { ...defaultSettings, theme: 'light' } });
    expect(root.dataset.theme).toBe('dark');
  });

  it('keeps the last of quick changes', async () => {
    const { store, transport } = setup();
    const first = store.update(transport, { density: 'compact' });
    const second = store.update(transport, { ui_font_size: 16 });
    await Promise.all([first, second]);
    expect(store.settings).toMatchObject({ density: 'compact', ui_font_size: 16 });
  });
});
