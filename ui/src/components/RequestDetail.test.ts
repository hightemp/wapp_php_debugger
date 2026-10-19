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
    emit({ event: 'request_updated', request: sampleRequest(42, { message_count: 3 }) });
    await waitFor(() => expect(transport.listMessages).toHaveBeenCalledTimes(2));
    emit({ event: 'request_updated', request: sampleRequest(7) });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(transport.listMessages).toHaveBeenCalledTimes(2);
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
});
