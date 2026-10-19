import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { ApiError } from '../lib/api/client';
import type { RequestPage } from '../lib/api/types';
import { fakeTransport, sampleRequest } from '../test/transport';
import RequestList from './RequestList.svelte';

const request = sampleRequest;

function feed(pages: RequestPage[] | Error) {
  const listRequests = vi.fn(async () => {
    if (pages instanceof Error) {
      throw pages;
    }
    return pages.shift() ?? { items: [], next_cursor: null };
  });
  const fake = fakeTransport({ listRequests });
  return { ...fake, listRequests };
}

describe('RequestList', () => {
  beforeEach(() => i18n.set('en'));

  it('shows requests newest first with status, duration and counters', async () => {
    const { transport } = feed([
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
    expect(rows[0]).toHaveTextContent('40 ms');
    expect(rows[0]).toHaveTextContent('2 errors');
    expect(rows[1]).toHaveTextContent('CLI');
  });

  it('shows an empty state with a hint', async () => {
    const { transport } = feed([{ items: [], next_cursor: null }]);
    render(RequestList, { transport });
    expect(await screen.findByText('No requests yet')).toBeInTheDocument();
  });

  it('shows an error and retries', async () => {
    const { transport, listRequests } = feed(new ApiError({ status: 0, title: 'Network error' }));
    render(RequestList, { transport });
    expect(await screen.findByRole('alert')).toHaveTextContent('Network error');
    await fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(listRequests).toHaveBeenCalledTimes(2);
  });

  it('adds live requests on top and reloads on resync', async () => {
    const { transport, emit, listRequests } = feed([
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
    const { transport, listRequests } = feed([
      { items: [request(3)], next_cursor: 'c1' },
      { items: [request(2)], next_cursor: null },
    ]);
    render(RequestList, { transport });
    await fireEvent.click(await screen.findByRole('button', { name: 'Load more' }));

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    expect(listRequests).toHaveBeenLastCalledWith({ limit: 100, cursor: 'c1' });
    expect(screen.queryByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
  });

  it('speaks Russian when the locale is ru', async () => {
    i18n.set('ru');
    const { transport } = feed([
      { items: [request(1, { error_count: 5, duration_ms: 1890.5 })], next_cursor: null },
    ]);
    render(RequestList, { transport });
    const [row] = await screen.findAllByRole('listitem');
    expect(row).toHaveTextContent('5 ошибок');
    expect(row).toHaveTextContent('1,89 с');
  });

  it('shows only the requests of the selected project', async () => {
    const { transport, listRequests, emit } = feed([
      { items: [request(1, { project_id: 3 })], next_cursor: null },
      { items: [], next_cursor: null },
    ]);
    const { rerender } = render(RequestList, { transport, projectId: 3 });
    await screen.findAllByRole('listitem');
    expect(listRequests).toHaveBeenCalledWith({ limit: 100, project_id: 3 });

    emit({ event: 'request_created', request: request(9, { project_id: 4 }) });
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(screen.getAllByRole('listitem')).toHaveLength(1);

    await rerender({ transport, projectId: undefined });
    await waitFor(() => expect(listRequests).toHaveBeenLastCalledWith({ limit: 100 }));
  });

  it('searches and filters the feed', async () => {
    const { transport, listRequests } = feed([{ items: [request(1)], next_cursor: null }]);
    render(RequestList, { transport });
    await screen.findAllByRole('listitem');

    await fireEvent.input(screen.getByRole('searchbox', { name: 'Search' }), {
      target: { value: 'cache miss' },
    });
    await waitFor(() =>
      expect(listRequests).toHaveBeenLastCalledWith({ limit: 100, q: 'cache miss' }),
    );

    await fireEvent.click(screen.getByRole('switch', { name: 'Errors only' }));
    await waitFor(() =>
      expect(listRequests).toHaveBeenLastCalledWith({
        limit: 100,
        q: 'cache miss',
        has_errors: true,
      }),
    );

    await fireEvent.click(screen.getByRole('radio', { name: 'CLI' }));
    await waitFor(() =>
      expect(listRequests).toHaveBeenLastCalledWith({
        limit: 100,
        q: 'cache miss',
        has_errors: true,
        sapi: 'cli',
      }),
    );
    expect(await screen.findByText('No matching requests')).toBeInTheDocument();
  });

  it('does not add live requests that the filter may not match', async () => {
    const { transport, emit } = feed([
      { items: [request(1)], next_cursor: null },
      { items: [request(1, { error_count: 1 })], next_cursor: null },
    ]);
    render(RequestList, { transport });
    await screen.findAllByRole('listitem');
    await fireEvent.click(screen.getByRole('switch', { name: 'Errors only' }));
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(1));

    emit({ event: 'request_created', request: request(9) });
    emit({ event: 'request_created', request: request(10, { error_count: 3 }) });
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    expect(screen.queryByText('GET /item/9')).not.toBeInTheDocument();
  });

  it('unsubscribes from the stream when removed', async () => {
    const { transport, unsubscribe } = feed([{ items: [], next_cursor: null }]);
    const { unmount } = render(RequestList, { transport });
    await screen.findByText('No requests yet');
    unmount();
    expect(unsubscribe).toHaveBeenCalled();
  });
});
