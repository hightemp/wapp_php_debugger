import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport, sampleProject } from '../test/transport';
import IngestToken from './IngestToken.svelte';

describe('IngestToken', () => {
  beforeEach(() => i18n.set('en'));

  it('makes a push token, shows it once with the library config, and stops it', async () => {
    const { transport } = fakeTransport({
      createIngestToken: vi.fn(async () => ({ token: 'tok_0123456789abcdef0123456789abcdef' })),
      deleteIngestToken: vi.fn(async () => {}),
    });
    const onchange = vi.fn();
    render(IngestToken, { transport, project: sampleProject(1), onchange });
    expect(screen.getByText('Pushed logs are off')).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Make a push token' }));
    await waitFor(() => expect(transport.createIngestToken).toHaveBeenCalledWith(1));
    const config = await screen.findByRole('textbox', { name: 'Library config' });
    expect((config as HTMLTextAreaElement).value).toContain(
      '"push_token": "tok_0123456789abcdef0123456789abcdef"',
    );
    expect((config as HTMLTextAreaElement).value).toContain('"push_url": "http://');
    expect(screen.getByText(/shown only now/)).toBeInTheDocument();
    expect(onchange).toHaveBeenLastCalledWith(true);

    await fireEvent.click(screen.getByRole('button', { name: 'Stop pushed logs' }));
    await waitFor(() => expect(transport.deleteIngestToken).toHaveBeenCalledWith(1));
    expect(screen.queryByRole('textbox', { name: 'Library config' })).not.toBeInTheDocument();
    expect(onchange).toHaveBeenLastCalledWith(false);
  });

  it('makes a new token for a project that has one', () => {
    const { transport } = fakeTransport();
    render(IngestToken, {
      transport,
      project: sampleProject(1, { ingest_enabled: true }),
      onchange: vi.fn(),
    });
    expect(screen.getByText('Pushed logs are on')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Make a new token' })).toBeInTheDocument();
  });
});
