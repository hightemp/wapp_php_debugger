import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../lib/api/client';
import { i18n } from '../lib/i18n/locale.svelte';
import {
  fakeTransport,
  sampleMessage,
  sampleRequest,
  sampleRequestDetail,
} from '../test/transport';
import RequestDetail from './RequestDetail.svelte';

function setup(overrides = {}) {
  const fake = fakeTransport({
    getRequest: vi.fn(async (id: number) =>
      sampleRequestDetail(id, {
        php_version: '8.3.12',
        children: [sampleRequest(43, { is_ajax: true, method: 'POST' })],
      }),
    ),
    listMessages: vi.fn(async () => ({
      items: [
        sampleMessage(2, { level: 'info', message: 'Loading item' }),
        sampleMessage(3, {
          level: 'error',
          message: 'Undefined index',
          source: { file: 'app/a.php', line: 3 },
        }),
      ],
      next_cursor: null,
    })),
    ...overrides,
  });
  const onselectmessage = vi.fn();
  const onselectrequest = vi.fn();
  render(RequestDetail, {
    transport: fake.transport,
    requestId: 42,
    onselectmessage,
    onselectrequest,
  });
  return { ...fake, onselectmessage, onselectrequest };
}

describe('RequestDetail', () => {
  beforeEach(() => i18n.set('en'));

  it('shows the request summary and its messages', async () => {
    setup();
    expect(await screen.findByRole('heading', { name: 'GET /item/42' })).toBeInTheDocument();
    expect(screen.getByText('PHP 8.3.12')).toBeInTheDocument();
    const messages = await screen.findAllByRole('listitem', { name: /message/i });
    expect(messages).toHaveLength(2);
    expect(messages[1]).toHaveTextContent('Undefined index');
    expect(messages[1]).toHaveTextContent('app/a.php:3');
  });

  it('opens a message', async () => {
    const { onselectmessage } = setup();
    const [first] = await screen.findAllByRole('listitem', { name: /message/i });
    await fireEvent.click(within(first as HTMLElement).getByRole('button'));
    expect(onselectmessage).toHaveBeenCalledWith(2);
  });

  it('filters messages by level', async () => {
    const { transport } = setup();
    await screen.findAllByRole('listitem', { name: /message/i });
    const errors = screen.getByRole('button', { name: 'Errors' });
    await fireEvent.click(errors);
    expect(errors).toHaveAttribute('aria-pressed', 'true');
    await waitFor(() =>
      expect(transport.listMessages).toHaveBeenLastCalledWith(42, { level: ['error'], limit: 500 }),
    );
  });

  it('searches messages together with the level filter', async () => {
    const { transport } = setup();
    await screen.findByRole('heading', { level: 2 });
    await fireEvent.click(screen.getByRole('button', { name: 'Errors' }));
    await fireEvent.input(screen.getByRole('searchbox', { name: 'Search messages' }), {
      target: { value: 'кеша' },
    });
    await waitFor(() =>
      expect(transport.listMessages).toHaveBeenLastCalledWith(42, {
        level: ['error'],
        q: 'кеша',
        limit: 500,
      }),
    );
  });

  it('lists child requests and opens them', async () => {
    const { onselectrequest } = setup();
    const child = await screen.findByRole('button', { name: /POST \/item\/43/ });
    await fireEvent.click(child);
    expect(onselectrequest).toHaveBeenCalledWith(43);
  });

  it('reloads when the request is updated live', async () => {
    const { transport, emit } = setup();
    await screen.findAllByRole('listitem', { name: /message/i });
    // The list and the views behind the tabs load once each
    await waitFor(() => expect(transport.listMessages).toHaveBeenCalledTimes(2));
    emit({ event: 'request_updated', request: sampleRequest(42, { message_count: 3 }) });
    await waitFor(() => expect(transport.listMessages).toHaveBeenCalledTimes(4));
    emit({ event: 'request_updated', request: sampleRequest(7) });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(transport.listMessages).toHaveBeenCalledTimes(4);
  });

  it('shows every message of a long request, page after page', async () => {
    const page = (from: number, count: number) =>
      Array.from({ length: count }, (_, i) =>
        sampleMessage(from + i, { message: `step ${from + i}` }),
      );
    // Answers by the cursor: the list and the views behind the tabs both read every page
    const listMessages = vi.fn(async (_id: number, params: { cursor?: string }) =>
      params.cursor === 'c1'
        ? { items: page(501, 120), next_cursor: null }
        : { items: page(1, 500), next_cursor: 'c1' },
    );
    setup({ listMessages });
    await waitFor(() =>
      expect(listMessages).toHaveBeenCalledWith(42, { limit: 500, cursor: 'c1' }),
    );
    const list = await screen.findByRole('list', { name: 'Messages' });
    await waitFor(() =>
      expect(within(list).getAllByRole('listitem')[0]).toHaveAttribute('aria-setsize', '620'),
    );
  });

  it('names levels in the UI language', async () => {
    i18n.set('ru');
    setup();
    const messages = await screen.findAllByRole('listitem', { name: /Сообщение/ });
    expect(messages[0]).toHaveTextContent('инфо');
    expect(messages[1]).toHaveTextContent('ошибка');
  });

  it('says so when the request is gone', async () => {
    setup({
      getRequest: vi.fn(async () =>
        Promise.reject(new ApiError({ status: 404, title: 'Not Found' })),
      ),
    });
    expect(await screen.findByText('Request not found')).toBeInTheDocument();
  });

  it('pins the request and keeps a note on it', async () => {
    const { transport } = setup({
      markRequest: vi.fn(async (id: number, patch: { pinned?: boolean; note?: string }) =>
        sampleRequest(id, { pinned: patch.pinned ?? false, note: patch.note }),
      ),
    });
    const pin = await screen.findByRole('button', { name: 'Pin' });
    expect(pin).toHaveAttribute('aria-pressed', 'false');
    await fireEvent.click(pin);
    await waitFor(() => expect(transport.markRequest).toHaveBeenCalledWith(42, { pinned: true }));
    expect(pin).toHaveAttribute('aria-pressed', 'true');

    await fireEvent.click(screen.getByRole('button', { name: 'Note' }));
    const note = screen.getByRole('textbox', { name: 'Note' });
    await fireEvent.input(note, { target: { value: 'the checkout bug' } });
    await fireEvent.blur(note);
    await waitFor(() =>
      expect(transport.markRequest).toHaveBeenLastCalledWith(42, { note: 'the checkout bug' }),
    );
  });

  describe('views of the request', () => {
    const start = Date.UTC(2026, 9, 19, 12, 0, 0);
    function insight() {
      const messages = [
        sampleMessage(2, { ts: start + 5, message: '$_GET', tags: ['globals'], has_context: true }),
        sampleMessage(3, {
          ts: start + 12,
          message: 'SELECT * FROM items WHERE order_id = ?',
          tags: ['sql'],
          duration_ms: 1.5,
          level: 'debug',
        }),
        sampleMessage(4, {
          ts: start + 15,
          message: 'SELECT * FROM items  WHERE order_id = ?',
          tags: ['sql'],
          duration_ms: 1.25,
          level: 'debug',
        }),
        sampleMessage(5, {
          ts: start + 150,
          message: 'GET https://pay.test/status',
          tags: ['http'],
          duration_ms: 120,
        }),
        sampleMessage(6, { ts: start + 160, type: 'timer', message: 'render', duration_ms: 40 }),
      ];
      return setup({
        getRequest: vi.fn(async (id: number) =>
          sampleRequestDetail(id, { started_at: start, duration_ms: 200 }),
        ),
        listMessages: vi.fn(async () => ({ items: messages, next_cursor: null })),
        getMessage: vi.fn(async (id: number) =>
          sampleMessage(id, { context: { id: '42', page: '2' } }),
        ),
      });
    }

    it('switches between messages, timeline, SQL and superglobals', async () => {
      insight();
      const tabs = await screen.findByRole('tablist', { name: 'Request views' });
      const names = within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent?.trim());
      expect(names?.map((name) => name?.replace(/\s+/g, ' '))).toEqual([
        'Messages',
        'Timeline',
        'SQL 2',
        'Superglobals',
      ]);
      expect(within(tabs).getByRole('tab', { name: 'Messages' })).toHaveAttribute(
        'aria-selected',
        'true',
      );
    });

    it('lists SQL with the total time and points out repeated queries', async () => {
      const { onselectmessage } = insight();
      await fireEvent.click(await screen.findByRole('tab', { name: /SQL/ }));
      const panel = screen.getByRole('tabpanel', { name: /SQL/ });
      expect(panel).toHaveTextContent('2 queries · 3 ms');
      expect(panel).toHaveTextContent('×2');
      expect(within(panel).getByText(/Repeated 2 times/)).toBeInTheDocument();
      await fireEvent.click(
        within(panel).getAllByRole('button', { name: /SELECT \* FROM items/ })[0] as HTMLElement,
      );
      expect(onselectmessage).toHaveBeenCalledWith(3);
    });

    it('puts timed events on the request timeline', async () => {
      const { onselectmessage } = insight();
      await fireEvent.click(await screen.findByRole('tab', { name: 'Timeline' }));
      const panel = screen.getByRole('tabpanel', { name: 'Timeline' });
      const http = within(panel).getByRole('button', { name: /GET https:\/\/pay.test\/status/ });
      expect(http).toHaveTextContent('120 ms');
      await fireEvent.click(http);
      expect(onselectmessage).toHaveBeenCalledWith(5);
    });

    it('shows the superglobals with their values', async () => {
      const { transport } = insight();
      await fireEvent.click(await screen.findByRole('tab', { name: 'Superglobals' }));
      const panel = screen.getByRole('tabpanel', { name: 'Superglobals' });
      expect(await within(panel).findByText('$_GET')).toBeInTheDocument();
      expect(await within(panel).findByText('page', { selector: '.key' })).toBeInTheDocument();
      expect(transport.getMessage).toHaveBeenCalledWith(2);
    });
  });
});
