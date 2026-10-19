import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import { ApiError, type Transport } from '../lib/api/client';
import type { Request, RequestPage, StreamEvent } from '../lib/api/types';
import RequestList from './RequestList.svelte';

function request(id: number, overrides: Partial<Request> = {}): Request {
  return {
    id,
    project_id: 1,
    server_id: 1,
    request_key: `k${id}`,
    format_version: 1,
    sapi: 'web',
    method: 'GET',
    url: `https://shop.test/item/${id}`,
    is_ajax: false,
    status_code: 200,
    started_at: Date.UTC(2026, 9, 19, 12, 0, id),
    duration_ms: 40,
    finished: true,
    message_count: 5,
    error_count: 0,
    warning_count: 0,
    received_at: 0,
    ...overrides,
  };
}

function fakeTransport(pages: RequestPage[] | Error) {
  let emit: (event: StreamEvent) => void = () => {};
  const listRequests = vi.fn(async () => {
    if (pages instanceof Error) {
      throw pages;
    }
    return pages.shift() ?? { items: [], next_cursor: null };
  });
  const unsubscribe = vi.fn();
  const transport: Transport = {
    getVersion: vi.fn(),
    listRequests,
    subscribe: (onEvent) => {
      emit = onEvent;
      return unsubscribe;
    },
  };
  return { transport, listRequests, unsubscribe, emit: (event: StreamEvent) => emit(event) };
}

describe('RequestList', () => {
  it('shows requests newest first with status, duration and counters', async () => {
    const { transport } = fakeTransport([
      {
        items: [request(2, { error_count: 2, warning_count: 1 }), request(1, { sapi: 'cli' })],
        next_cursor: null,
      },
    ]);
    render(RequestList, { transport });

    const rows = await screen.findAllByRole('listitem');
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent('GET /item/2');
    expect(rows[0]).toHaveTextContent('200');
    expect(rows[0]).toHaveTextContent('40 мс');
    expect(rows[0]).toHaveTextContent('2 ошибки');
    expect(rows[1]).toHaveTextContent('CLI');
  });

  it('shows an empty state with a hint', async () => {
    const { transport } = fakeTransport([{ items: [], next_cursor: null }]);
    render(RequestList, { transport });
    expect(await screen.findByText('Запросов пока нет')).toBeInTheDocument();
  });

  it('shows an error and retries', async () => {
    const { transport, listRequests } = fakeTransport(
      new ApiError({ status: 0, title: 'Network error' }),
    );
    render(RequestList, { transport });
    expect(await screen.findByRole('alert')).toHaveTextContent('Network error');
    await fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    expect(listRequests).toHaveBeenCalledTimes(2);
  });

  it('adds live requests on top and reloads on resync', async () => {
    const { transport, emit, listRequests } = fakeTransport([
      { items: [request(1)], next_cursor: null },
      { items: [request(3), request(2), request(1)], next_cursor: null },
    ]);
    render(RequestList, { transport });
    await screen.findAllByRole('listitem');

    emit({ event: 'request_created', request: request(2) });
    await waitFor(() => expect(screen.getAllByRole('listitem')[0]).toHaveTextContent('/item/2'));

    emit({ event: 'resync' });
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(3));
    expect(listRequests).toHaveBeenCalledTimes(2);
  });

  it('loads the next page', async () => {
    const { transport, listRequests } = fakeTransport([
      { items: [request(3)], next_cursor: 'c1' },
      { items: [request(2)], next_cursor: null },
    ]);
    render(RequestList, { transport });
    await fireEvent.click(await screen.findByRole('button', { name: 'Загрузить ещё' }));

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    expect(listRequests).toHaveBeenLastCalledWith({ limit: 100, cursor: 'c1' });
    expect(screen.queryByRole('button', { name: 'Загрузить ещё' })).not.toBeInTheDocument();
  });

  it('unsubscribes from the stream when removed', async () => {
    const { transport, unsubscribe } = fakeTransport([{ items: [], next_cursor: null }]);
    const { unmount } = render(RequestList, { transport });
    await screen.findByText('Запросов пока нет');
    unmount();
    expect(unsubscribe).toHaveBeenCalled();
  });
});
