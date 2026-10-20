import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { openTabs } from '../lib/tabs.svelte';
import { fakeTransport, sampleRequest } from '../test/transport';
import RequestTabs from './RequestTabs.svelte';

describe('RequestTabs', () => {
  beforeEach(() => {
    i18n.set('en');
    openTabs.clear();
  });

  it('keeps opened requests as tabs to switch, compare and close', async () => {
    const { transport } = fakeTransport({
      getRequest: vi.fn(async (id: number) => ({
        ...sampleRequest(id),
        lang: 'php' as const,
        children: [],
      })),
    });
    const onselect = vi.fn();
    const oncompare = vi.fn();
    const view = render(RequestTabs, { transport, currentId: 1, onselect, oncompare });
    await view.rerender({ transport, currentId: 2, onselect, oncompare });
    const tabs = screen.getByRole('navigation', { name: 'Open requests' });
    const first = await within(tabs).findByRole('button', { name: 'GET /item/1' });
    expect(within(tabs).getByRole('button', { name: 'GET /item/2' })).toHaveAttribute(
      'aria-current',
      'page',
    );

    await fireEvent.click(first);
    expect(onselect).toHaveBeenCalledWith(1);
    await fireEvent.click(within(tabs).getByRole('button', { name: 'Compare with GET /item/1' }));
    expect(oncompare).toHaveBeenCalledWith(1);
    expect(
      within(tabs).queryByRole('button', { name: 'Compare with GET /item/2' }),
    ).not.toBeInTheDocument();

    // Closing the open one goes to its neighbour
    await fireEvent.click(within(tabs).getByRole('button', { name: 'Close GET /item/2' }));
    expect(onselect).toHaveBeenLastCalledWith(1);
    expect(within(tabs).queryByRole('button', { name: 'GET /item/2' })).not.toBeInTheDocument();
  });

  it('shows no tabs for a single request', async () => {
    const { transport } = fakeTransport();
    render(RequestTabs, { transport, currentId: 1, onselect: vi.fn(), oncompare: vi.fn() });
    expect(screen.queryByRole('navigation', { name: 'Open requests' })).not.toBeInTheDocument();
  });
});
