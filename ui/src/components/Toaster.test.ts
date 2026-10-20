import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { toasts } from '../lib/toasts.svelte';
import Toaster from './Toaster.svelte';

describe('Toaster', () => {
  beforeEach(() => {
    i18n.set('en');
    vi.useFakeTimers();
    toasts.clear();
  });
  afterEach(() => vi.useRealTimers());

  it('shows notices for a while, with a link when there is one', async () => {
    render(Toaster);
    toasts.push({ text: 'Copied' });
    toasts.push({
      text: 'Error in shop: GET /x',
      kind: 'error',
      link: { label: 'Open', href: '#/projects/1/requests/9' },
    });
    await vi.advanceTimersByTimeAsync(0);
    const region = screen.getByRole('region', { name: 'Notices' });
    expect(region).toHaveTextContent('Copied');
    expect(screen.getByRole('link', { name: 'Open' })).toHaveAttribute(
      'href',
      '#/projects/1/requests/9',
    );
    await vi.advanceTimersByTimeAsync(6000);
    expect(region).not.toHaveTextContent('Copied');
  });

  it('closes a notice at once', async () => {
    render(Toaster);
    toasts.push({ text: 'Copied' });
    await vi.advanceTimersByTimeAsync(0);
    await fireEvent.click(screen.getByRole('button', { name: 'Close the notice' }));
    expect(screen.queryByText('Copied')).not.toBeInTheDocument();
  });
});
