import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App.svelte';
import { i18n } from './lib/i18n/locale.svelte';
import { fakeTransport, sampleMessage, sampleRequest } from './test/transport';

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

  it('opens a request and a message and keeps the selection in the URL', async () => {
    const { transport } = setup();
    render(App, { transport });
    const [row] = await screen.findAllByRole('listitem');
    await fireEvent.click(within(row as HTMLElement).getByRole('button'));
    expect(window.location.hash).toBe('#/requests/42');

    const [message] = await screen.findAllByRole('listitem', { name: /message/i });
    await fireEvent.click(within(message as HTMLElement).getByRole('button'));
    expect(window.location.hash).toBe('#/requests/42/messages/1001');
    expect(transport.getMessage).toHaveBeenCalledWith(1001);
  });

  it('restores the selection from the URL', async () => {
    window.location.hash = '#/requests/42/messages/1001';
    const { transport } = setup();
    render(App, { transport });
    expect(await screen.findByRole('heading', { name: 'GET /item/42' })).toBeInTheDocument();
    expect(transport.getMessage).toHaveBeenCalledWith(1001);
  });
});
