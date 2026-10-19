import { render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import App from './App.svelte';
import type { Transport } from './lib/api/client';

const transport: Transport = {
  getVersion: vi.fn().mockResolvedValue({ version: 'v0.0.0-test', commit: 'abc', date: '' }),
  listRequests: vi.fn().mockResolvedValue({ items: [], next_cursor: null }),
  subscribe: () => () => {},
};

describe('App', () => {
  it('shows the product name, theme switch and the request feed', async () => {
    render(App, { transport });
    expect(screen.getByRole('heading', { name: 'wapp-debugger', level: 1 })).toBeInTheDocument();
    expect(screen.getByRole('radiogroup', { name: 'Theme' })).toBeInTheDocument();
    expect(await screen.findByText('No requests yet')).toBeInTheDocument();
  });

  it('shows the viewer version', async () => {
    render(App, { transport });
    expect(await screen.findByText('v0.0.0-test')).toBeInTheDocument();
  });
});
