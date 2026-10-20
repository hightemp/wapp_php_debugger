import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App.svelte';
import { i18n } from './lib/i18n/locale.svelte';
import { fakeTransport, sampleMessage, sampleProject, sampleRequest } from './test/transport';

function setup() {
  return fakeTransport({
    listRequests: vi.fn(async () => ({
      items: [sampleRequest(3), sampleRequest(2), sampleRequest(1)],
      next_cursor: null,
    })),
    listProjects: vi.fn(async () => ({ items: [sampleProject(1, { name: 'shop' })] })),
    listMessages: vi.fn(async () => ({
      items: [sampleMessage(11, { message: 'first' }), sampleMessage(12, { message: 'second' })],
      next_cursor: null,
    })),
  });
}

const key = (k: string, init: KeyboardEventInit = {}) =>
  fireEvent.keyDown(document.body, { key: k, ...init });

describe('App keyboard', () => {
  beforeEach(() => {
    i18n.set('en');
    window.location.hash = '';
  });

  it('walks the feed with j and k and the messages with J and K', async () => {
    const { transport } = setup();
    render(App, { transport });
    await screen.findAllByRole('button', { name: /GET \/item\/3/ });
    await key('j');
    expect(window.location.hash).toBe('#/requests/3');
    await key('j');
    expect(window.location.hash).toBe('#/requests/2');
    await key('k');
    expect(window.location.hash).toBe('#/requests/3');
    await screen.findByRole('list', { name: 'Messages' });
    await key('J', { shiftKey: true });
    expect(window.location.hash).toBe('#/requests/3/messages/11');
    await key('J', { shiftKey: true });
    expect(window.location.hash).toBe('#/requests/3/messages/12');
    await key('Escape');
    expect(window.location.hash).toBe('#/requests/3');
  });

  it('goes to the filter line with / and leaves typing alone', async () => {
    const { transport } = setup();
    render(App, { transport });
    await screen.findAllByRole('button', { name: /GET \/item\/3/ });
    await key('/');
    const line = screen.getByRole('combobox', { name: 'Search' });
    expect(line).toHaveFocus();
    await fireEvent.keyDown(line, { key: 'j' });
    expect(window.location.hash).toBe('');
  });

  it('shows the shortcuts with ?', async () => {
    const { transport } = setup();
    render(App, { transport });
    await key('?', { shiftKey: true });
    const help = await screen.findByRole('dialog', { name: 'Keyboard shortcuts' });
    expect(help).toHaveTextContent('Ctrl+K');
    expect(help).toHaveTextContent('Next request');
  });

  it('runs commands from the palette', async () => {
    const { transport } = setup();
    render(App, { transport });
    await screen.findAllByRole('button', { name: /GET \/item\/3/ });
    await key('k', { ctrlKey: true });
    const palette = await screen.findByRole('dialog', { name: 'Commands' });
    const input = within(palette).getByRole('combobox', { name: 'Command' });
    await fireEvent.input(input, { target: { value: 'shop' } });
    expect(within(palette).getByRole('option', { name: /Project: shop/ })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(window.location.hash).toBe('#/projects/1'));
    expect(screen.queryByRole('dialog', { name: 'Commands' })).not.toBeInTheDocument();

    await key('k', { metaKey: true });
    const again = await screen.findByRole('dialog', { name: 'Commands' });
    await fireEvent.input(within(again).getByRole('combobox', { name: 'Command' }), {
      target: { value: 'dark' },
    });
    await fireEvent.keyDown(within(again).getByRole('combobox', { name: 'Command' }), {
      key: 'Enter',
    });
    expect(document.documentElement.dataset.theme).toBe('dark');

    await key('k', { ctrlKey: true });
    const search = await screen.findByRole('dialog', { name: 'Commands' });
    await fireEvent.input(within(search).getByRole('combobox', { name: 'Command' }), {
      target: { value: 'level:error' },
    });
    await fireEvent.click(
      within(search).getByRole('option', { name: /Filter the feed: level:error/ }),
    );
    expect(screen.getByRole('combobox', { name: 'Search' })).toHaveValue('level:error');
  });

  it('compares two open requests and steps back from the comparison', async () => {
    const { openTabs } = await import('./lib/tabs.svelte');
    openTabs.clear();
    const { transport } = setup();
    render(App, { transport });
    await screen.findAllByRole('button', { name: /GET \/item\/3/ });
    await key('j');
    await key('j');
    expect(window.location.hash).toBe('#/requests/2');
    const tabs = await screen.findByRole('navigation', { name: 'Open requests' });
    await fireEvent.click(
      await within(tabs).findByRole('button', { name: 'Compare with GET /item/3' }),
    );
    expect(window.location.hash).toBe('#/requests/2/compare/3');
    expect(await screen.findByRole('region', { name: 'Comparison' })).toBeInTheDocument();
    await key('Escape');
    expect(window.location.hash).toBe('#/requests/2');
    expect(screen.queryByRole('region', { name: 'Comparison' })).not.toBeInTheDocument();
  });
});
