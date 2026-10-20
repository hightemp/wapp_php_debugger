import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport, sampleMessage, sampleRequestDetail } from '../test/transport';
import RequestCompare from './RequestCompare.svelte';

function setup() {
  const sessions: Record<number, Record<string, unknown>> = {
    101: { step: 'cart', user: { id: 7 } },
    201: { step: 'pay', user: { id: 7 }, coupon: 'SALE' },
  };
  const get: Record<number, Record<string, unknown>> = { 102: { id: '1' }, 202: { id: '1' } };
  return fakeTransport({
    getRequest: vi.fn(async (id: number) =>
      id === 1
        ? sampleRequestDetail(1, { status_code: 200, duration_ms: 40, started_at: 1000 })
        : sampleRequestDetail(2, {
            status_code: 500,
            duration_ms: 900,
            error_count: 1,
            started_at: 2000,
          }),
    ),
    listMessages: vi.fn(async (requestId: number, params: { tag?: string }) => ({
      items:
        params.tag === 'globals'
          ? [
              sampleMessage(requestId * 100 + 1, {
                message: '$_SESSION',
                tags: ['globals'],
                has_context: true,
              }),
              sampleMessage(requestId * 100 + 2, {
                message: '$_GET',
                tags: ['globals'],
                has_context: true,
              }),
            ]
          : [],
      next_cursor: null,
    })),
    getMessage: vi.fn(async (id: number) =>
      sampleMessage(id, { context: sessions[id] ?? get[id] }),
    ),
  });
}

describe('RequestCompare', () => {
  beforeEach(() => i18n.set('en'));

  it('puts the earlier request first and shows what changed', async () => {
    const { transport } = setup();
    const onclose = vi.fn();
    // Asked the other way round: the earlier one still comes first
    render(RequestCompare, { transport, requestId: 2, otherId: 1, onclose });
    const fields = await screen.findByRole('table', { name: 'The requests' });
    await waitFor(() =>
      expect(within(fields).getByRole('row', { name: /Status/ })).toHaveTextContent('200500'),
    );
    expect(within(fields).getByRole('row', { name: /Status/ })).toHaveAttribute(
      'data-changed',
      'true',
    );
    expect(within(fields).getByRole('row', { name: /Method/ })).toHaveAttribute(
      'data-changed',
      'false',
    );

    const session = await screen.findByRole('table', { name: '$_SESSION' });
    await waitFor(() => expect(within(session).getAllByRole('row')).toHaveLength(3));
    expect(within(session).getByRole('row', { name: /coupon/ })).toHaveTextContent('added');
    expect(within(session).getByRole('row', { name: /step/ })).toHaveTextContent('"cart""pay"');
    expect(await screen.findByText('$_GET: the same')).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Close the comparison' }));
    expect(onclose).toHaveBeenCalled();
  });
});
