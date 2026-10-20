import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { ApiError } from '../lib/api/client';
import type { RequestPage } from '../lib/api/types';
import { fakeTransport, sampleProject, sampleRequest, sampleServer } from '../test/transport';
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

    await fireEvent.input(screen.getByRole('combobox', { name: 'Search' }), {
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

  describe('with several servers', () => {
    function servers() {
      const listRequests = vi.fn(async () => ({
        items: [
          request(2, { project_id: 30, server_id: 3 }),
          request(1, { project_id: 10, server_id: 1 }),
        ],
        next_cursor: null,
      }));
      const fake = fakeTransport({
        listRequests,
        listServers: vi.fn(async () => ({
          items: [sampleServer(1), sampleServer(3, { name: 'prod-web', environment: 'prod' })],
        })),
        listProjects: vi.fn(async () => ({
          items: [
            sampleProject(10, { name: 'local-shop', server_id: 1 }),
            sampleProject(30, { name: 'shop', server_id: 3 }),
          ],
        })),
      });
      return { ...fake, listRequests };
    }

    it('names the project and the server of each request in the combined feed', async () => {
      const { transport } = servers();
      render(RequestList, { transport });
      const rows = await screen.findAllByRole('listitem');
      await waitFor(() => expect(rows[0]).toHaveTextContent('shop · prod-web'));
      expect(rows[1]).toHaveTextContent('local-shop');
    });

    it('does not repeat the project inside a project feed', async () => {
      const { transport } = servers();
      render(RequestList, { transport, projectId: 30 });
      const rows = await screen.findAllByRole('listitem');
      await new Promise((resolve) => setTimeout(resolve, 20));
      expect(rows[0]).not.toHaveTextContent('prod-web');
    });

    it('shows the feed of one server', async () => {
      const { transport, listRequests } = servers();
      render(RequestList, { transport, serverId: 3 });
      await waitFor(() =>
        expect(listRequests).toHaveBeenCalledWith(expect.objectContaining({ server_id: 3 })),
      );
    });

    it('filters the combined feed by environment', async () => {
      const { transport, listRequests } = servers();
      render(RequestList, { transport });
      const environments = await screen.findByRole('radiogroup', { name: 'Environment' });
      await fireEvent.click(within(environments).getByRole('radio', { name: 'prod' }));
      await waitFor(() =>
        expect(listRequests).toHaveBeenLastCalledWith(
          expect.objectContaining({ environment: 'prod' }),
        ),
      );
    });
  });

  it('offers no environment filter with only the local server', async () => {
    const { transport } = feed([{ items: [request(1)], next_cursor: null }]);
    render(RequestList, { transport });
    await screen.findAllByRole('listitem');
    expect(screen.queryByRole('radiogroup', { name: 'Environment' })).not.toBeInTheDocument();
  });

  describe('the filter line', () => {
    it('sends conditions as feed parameters and says what it could not read', async () => {
      const { transport, listRequests } = feed([{ items: [request(1)], next_cursor: null }]);
      render(RequestList, { transport });
      await screen.findAllByRole('listitem');
      const line = screen.getByRole('combobox', { name: 'Search' });
      await fireEvent.input(line, {
        target: { value: 'level:error status:5xx duration>1s checkout' },
      });
      await waitFor(() =>
        expect(listRequests).toHaveBeenLastCalledWith({
          limit: 100,
          level: 'error',
          status: '5xx',
          min_duration_ms: 1000,
          q: 'checkout',
        }),
      );
      await fireEvent.input(line, { target: { value: 'level:fatal' } });
      expect(await screen.findByText(/Not understood: level:fatal/)).toBeInTheDocument();
    });

    it('completes keys and values from the keyboard', async () => {
      const { transport } = feed([{ items: [request(1)], next_cursor: null }]);
      render(RequestList, { transport });
      await screen.findAllByRole('listitem');
      const line = screen.getByRole('combobox', { name: 'Search' });
      await fireEvent.input(line, { target: { value: 'checkout lev' } });
      expect(screen.getByRole('option', { name: 'level:' })).toBeInTheDocument();
      await fireEvent.keyDown(line, { key: 'Enter' });
      expect(line).toHaveValue('checkout level:');
      await fireEvent.input(line, { target: { value: 'checkout level:' } });
      await fireEvent.keyDown(line, { key: 'ArrowDown' });
      expect(screen.getByRole('option', { name: 'level:warning' })).toHaveAttribute(
        'aria-selected',
        'true',
      );
      await fireEvent.keyDown(line, { key: 'Tab' });
      expect(line).toHaveValue('checkout level:warning ');
    });

    it('checks live requests against the conditions it can see', async () => {
      const { transport, emit, listRequests } = feed([
        { items: [request(1)], next_cursor: null },
        { items: [request(1, { method: 'POST' })], next_cursor: null },
      ]);
      render(RequestList, { transport });
      await screen.findAllByRole('listitem');
      await fireEvent.input(screen.getByRole('combobox', { name: 'Search' }), {
        target: { value: 'method:post' },
      });
      await waitFor(() =>
        expect(listRequests).toHaveBeenLastCalledWith({ limit: 100, method: 'POST' }),
      );
      await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(1));
      emit({ event: 'request_created', request: request(9, { method: 'GET' }) });
      emit({ event: 'request_created', request: request(10, { method: 'POST' }) });
      await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
      expect(screen.queryByText('GET /item/9')).not.toBeInTheDocument();
    });
  });

  it('marks pinned requests and notes', async () => {
    const { transport } = feed([
      {
        items: [request(2, { pinned: true, note: 'checkout bug' }), request(1)],
        next_cursor: null,
      },
    ]);
    render(RequestList, { transport });
    const rows = await screen.findAllByRole('listitem');
    expect(rows[0]).toHaveTextContent('Pinned');
    expect(rows[0]).toHaveTextContent('checkout bug');
    expect(rows[1]).not.toHaveTextContent('Pinned');
  });

  it('pauses the live feed and shows what came in when resumed', async () => {
    const { transport, emit, listRequests } = feed([
      { items: [request(1)], next_cursor: null },
      { items: [request(11), request(10), request(1)], next_cursor: null },
    ]);
    render(RequestList, { transport });
    await screen.findAllByRole('listitem');
    await fireEvent.click(screen.getByRole('button', { name: 'Pause' }));
    emit({ event: 'request_created', request: request(10) });
    emit({ event: 'request_created', request: request(11) });
    expect(screen.getAllByRole('listitem')).toHaveLength(1);
    const resume = await screen.findByRole('button', { name: /2 new — show/ });
    await fireEvent.click(resume);
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(3));
    expect(listRequests).toHaveBeenCalledTimes(2);
    expect(screen.getByRole('button', { name: 'Pause' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('tells about a failing request the feed does not show', async () => {
    const { toasts } = await import('../lib/toasts.svelte');
    toasts.clear();
    const { transport, emit } = feed([{ items: [request(1)], next_cursor: null }]);
    render(RequestList, { transport, projectId: 1 });
    await screen.findAllByRole('listitem');
    emit({ event: 'request_created', request: request(9, { project_id: 2, error_count: 2 }) });
    emit({ event: 'request_created', request: request(10, { project_id: 2 }) });
    await waitFor(() => expect(toasts.items).toHaveLength(1));
    expect(toasts.items[0]).toMatchObject({
      kind: 'error',
      link: { href: '#/projects/2/requests/9' },
    });
    expect(toasts.items[0]?.text).toContain('GET /item/9');
  });
});
