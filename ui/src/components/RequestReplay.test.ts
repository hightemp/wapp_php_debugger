import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { toasts } from '../lib/toasts.svelte';
import { fakeTransport, sampleMessage, sampleRequest } from '../test/transport';
import RequestReplay from './RequestReplay.svelte';

const globals = [
  sampleMessage(5, { message: '$_SERVER', tags: ['globals'], has_context: true }),
  sampleMessage(6, { message: '$_POST', tags: ['globals'], has_context: true }),
];

function setup() {
  return fakeTransport({
    getMessage: vi.fn(async (id: number) =>
      id === 5
        ? sampleMessage(5, { context: { HTTP_ACCEPT: 'text/html', HTTP_AUTHORIZATION: '***' } })
        : sampleMessage(6, { context: { q: 'chairs' } }),
    ),
  });
}

describe('RequestReplay', () => {
  beforeEach(() => {
    i18n.set('en');
    toasts.clear();
  });

  it('opens a GET request again, with Xdebug when asked', async () => {
    const { transport } = setup();
    render(RequestReplay, { transport, request: sampleRequest(1), messages: [] });
    const open = screen.getByRole('link', { name: 'Open again' });
    expect(open).toHaveAttribute('href', 'https://shop.test/item/1');
    await fireEvent.click(screen.getByRole('switch', { name: 'With Xdebug' }));
    expect(open).toHaveAttribute('href', 'https://shop.test/item/1?XDEBUG_SESSION=1');
  });

  it('copies the request as curl from the logged superglobals', async () => {
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    const { transport } = setup();
    render(RequestReplay, {
      transport,
      request: sampleRequest(1, { method: 'POST' }),
      messages: globals,
    });
    expect(screen.queryByRole('link', { name: 'Open again' })).not.toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Copy as curl' }));
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith(
        [
          "curl 'https://shop.test/item/1'",
          "  -H 'Accept: text/html'",
          "  -H 'Authorization: ***'",
          "  --data-urlencode 'q=chairs'",
        ].join(' \\\n'),
      ),
    );
    expect(toasts.items.at(-1)?.text).toBe('Copied as curl; masked values (***) need filling in');
  });

  it('has nothing to repeat for a console command', () => {
    const { transport } = setup();
    render(RequestReplay, {
      transport,
      request: sampleRequest(1, { sapi: 'cli', method: null, url: null }),
      messages: [],
    });
    expect(screen.queryByRole('button', { name: 'Copy as curl' })).not.toBeInTheDocument();
  });
});
