import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App.svelte';
import { defaultSettings } from './lib/appearance';
import { i18n } from './lib/i18n/locale.svelte';
import { createAppearanceStore } from './lib/settings.svelte';
import {
  fakeTransport,
  sampleMessage,
  sampleProject,
  sampleRequest,
  sampleServer,
} from './test/transport';

function freshStore() {
  localStorage.clear();
  return createAppearanceStore({
    storage: localStorage,
    root: document.documentElement,
    media: () => ({ dark: false, moreContrast: false }),
  });
}

function setup() {
  return fakeTransport({
    listRequests: vi.fn(async () => ({ items: [sampleRequest(42)], next_cursor: null })),
    listMessages: vi.fn(async () => ({
      items: [sampleMessage(1001, { message: 'Loading item' })],
      next_cursor: null,
    })),
  });
}

describe('App', () => {
  beforeEach(() => {
    i18n.set('en');
    window.location.hash = '';
  });

  it('shows the product name, the switches and an empty feed', async () => {
    const { transport } = fakeTransport();
    render(App, { transport });
    expect(screen.getByRole('heading', { name: 'wapp-debugger', level: 1 })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Language' })).toBeInTheDocument();
    expect(await screen.findByText('No requests yet')).toBeInTheDocument();
    expect(await screen.findByText('v0.0.0-test')).toBeInTheDocument();
  });

  it('offers the settings of the desktop app only in the desktop app', async () => {
    const { transport } = fakeTransport();
    render(App, { transport });
    await screen.findByText('v0.0.0-test');
    expect(screen.queryByRole('button', { name: 'Settings' })).not.toBeInTheDocument();
  });

  it('opens the desktop settings', async () => {
    const { transport } = fakeTransport({
      getVersion: vi.fn(async () => ({
        app: 'desktop' as const,
        version: 'v1',
        commit: '',
        date: '',
      })),
    });
    render(App, { transport });
    await fireEvent.click(await screen.findByRole('button', { name: 'Settings' }));
    expect(await screen.findByRole('dialog', { name: 'Settings' })).toBeInTheDocument();
  });

  it('tells the desktop app the language of its notifications', async () => {
    const { transport } = fakeTransport({
      getVersion: vi.fn(async () => ({
        app: 'desktop' as const,
        version: 'v1',
        commit: '',
        date: '',
      })),
    });
    render(App, { transport });
    await waitFor(() =>
      expect(transport.updateDesktopSettings).toHaveBeenCalledWith(
        expect.objectContaining({ locale: 'en' }),
      ),
    );
    await fireEvent.click(screen.getByRole('radio', { name: 'Русский' }));
    await waitFor(() =>
      expect(transport.updateDesktopSettings).toHaveBeenLastCalledWith(
        expect.objectContaining({ locale: 'ru' }),
      ),
    );
  });

  it('tells about a newer version when the check is on', async () => {
    const { transport } = fakeTransport({
      getUpdates: vi.fn(async () => ({
        enabled: true,
        current: 'v0.3.0',
        latest: 'v0.4.0',
        url: 'https://github.com/hightemp/wapp_php_debugger/releases/tag/v0.4.0',
        newer: true,
      })),
    });
    render(App, { transport });
    const link = await screen.findByRole('link', { name: 'v0.4.0 is out' });
    expect(link).toHaveAttribute(
      'href',
      'https://github.com/hightemp/wapp_php_debugger/releases/tag/v0.4.0',
    );
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('says nothing when the check is off or the version is current', async () => {
    const { transport } = fakeTransport();
    render(App, { transport });
    await screen.findByText('v0.0.0-test');
    expect(screen.queryByRole('link', { name: /is out/ })).not.toBeInTheDocument();
  });

  it('opens a request and a message and keeps the selection in the URL', async () => {
    const { transport } = setup();
    render(App, { transport });
    await fireEvent.click(await screen.findByRole('button', { name: /GET \/item\/42/ }));
    expect(window.location.hash).toBe('#/requests/42');

    const [message] = await screen.findAllByRole('listitem', { name: /message/i });
    await fireEvent.click(within(message as HTMLElement).getByRole('button'));
    expect(window.location.hash).toBe('#/requests/42/messages/1001');
    expect(transport.getMessage).toHaveBeenCalledWith(1001);
  });

  it('filters the feed by the project chosen in the panel', async () => {
    const { transport } = fakeTransport({
      listProjects: vi.fn(async () => ({ items: [sampleProject(3, { name: 'shop' })] })),
    });
    render(App, { transport });
    await fireEvent.click(await screen.findByRole('button', { name: 'shop' }));
    expect(window.location.hash).toBe('#/projects/3');
    await waitFor(() =>
      expect(transport.listRequests).toHaveBeenLastCalledWith({ limit: 100, project_id: 3 }),
    );
  });

  it('restores the selection from the URL', async () => {
    window.location.hash = '#/requests/42/messages/1001';
    const { transport } = setup();
    render(App, { transport });
    expect(await screen.findByRole('heading', { name: 'GET /item/42' })).toBeInTheDocument();
    expect(transport.getMessage).toHaveBeenCalledWith(1001);
  });

  it('applies the settings of the viewer and follows other windows', async () => {
    const fake = fakeTransport({
      getSettings: vi.fn(async () => ({
        ...defaultSettings,
        theme: 'dark',
        density: 'compact' as const,
      })),
    });
    render(App, { transport: fake.transport, store: freshStore() });
    await waitFor(() => expect(document.documentElement.dataset.theme).toBe('dark'));
    expect(document.documentElement.dataset.density).toBe('compact');
    fake.emit({
      event: 'settings_updated',
      settings: { ...defaultSettings, theme: 'high-contrast' },
    });
    expect(document.documentElement.dataset.theme).toBe('high-contrast');
  });

  it('frames the window in the color of prod while the setting is on', async () => {
    window.location.hash = '#/projects/30';
    const fake = fakeTransport({
      listServers: vi.fn(async () => ({
        items: [sampleServer(1), sampleServer(3, { name: 'prod-web', environment: 'prod' })],
      })),
      listProjects: vi.fn(async () => ({
        items: [sampleProject(30, { name: 'shop', server_id: 3 })],
      })),
    });
    const store = freshStore();
    const { container } = render(App, { transport: fake.transport, store });
    const app = container.querySelector('.app') as HTMLElement;
    await waitFor(() => expect(app.dataset.envFrame).toBe('prod'));
    fake.emit({ event: 'settings_updated', settings: { ...defaultSettings, env_frame: false } });
    await waitFor(() => expect(app.dataset.envFrame).toBeUndefined());
  });

  it('opens the appearance settings and the theme editor from there', async () => {
    const { transport } = fakeTransport();
    render(App, { transport, store: freshStore() });
    await fireEvent.click(screen.getByRole('button', { name: 'Appearance' }));
    const dialog = await screen.findByRole('dialog', { name: 'Appearance' });
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Theme editor…' }));
    expect(await screen.findByRole('dialog', { name: 'Theme editor' })).toBeInTheDocument();
    expect(screen.queryByRole('dialog', { name: 'Appearance' })).not.toBeInTheDocument();
  });

  it('saves the language with the settings', async () => {
    const { transport } = fakeTransport();
    render(App, { transport, store: freshStore() });
    await fireEvent.click(screen.getByRole('radio', { name: 'Русский' }));
    await waitFor(() =>
      expect(transport.updateSettings).toHaveBeenCalledWith(
        expect.objectContaining({ locale: 'ru' }),
      ),
    );
  });

  it('shows the component showcase instead of the viewer', async () => {
    window.location.hash = '#/dev/components';
    const { transport } = fakeTransport();
    render(App, { transport, store: freshStore() });
    expect(
      screen.getByRole('heading', { name: 'Component showcase', level: 1 }),
    ).toBeInTheDocument();
    expect(screen.queryByText('No requests yet')).not.toBeInTheDocument();
  });
});
