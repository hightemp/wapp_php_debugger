import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../lib/api/client';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport } from '../test/transport';
import ConfigDialog from './ConfigDialog.svelte';

const yaml = 'version: 1\nservers:\n  - name: stage\n    kind: ssh\n';

function setup() {
  const fake = fakeTransport({
    exportConfig: vi.fn(async () => yaml),
    importConfig: vi.fn(async () => ({
      servers_created: ['stage'],
      projects_created: ['stage/shop'],
      skipped: ['local/blog'],
      needs_token: ['hosting'],
    })),
  });
  const onchange = vi.fn();
  render(ConfigDialog, { open: true, transport: fake.transport, onclose: vi.fn(), onchange });
  return { ...fake, onchange };
}

describe('ConfigDialog', () => {
  beforeEach(() => i18n.set('en'));

  it('shows the export to copy or download', async () => {
    setup();
    const text = await screen.findByLabelText('Servers and projects (YAML)');
    await waitFor(() => expect(text).toHaveValue(yaml));
    expect(text).toHaveAttribute('readonly');
    const download = screen.getByRole('link', { name: 'Download' });
    expect(download).toHaveAttribute('download', 'wapp-debugger.yaml');
  });

  it('imports a pasted file and says what was added', async () => {
    const { transport, onchange } = setup();
    await fireEvent.click(screen.getByRole('radio', { name: 'Import' }));
    const input = screen.getByLabelText('A file exported from a viewer (YAML)');
    await fireEvent.input(input, { target: { value: yaml } });
    await fireEvent.click(screen.getByRole('button', { name: 'Import' }));
    await waitFor(() => expect(transport.importConfig).toHaveBeenCalledWith(yaml));
    const summary = await screen.findByRole('status');
    expect(summary).toHaveTextContent('Added: stage, stage/shop');
    expect(summary).toHaveTextContent('Already there: local/blog');
    expect(summary).toHaveTextContent('Set the token of: hosting');
    expect(onchange).toHaveBeenCalled();
  });

  it('shows why a file was rejected', async () => {
    const { transport } = setup();
    vi.mocked(transport.importConfig).mockRejectedValueOnce(
      new ApiError({ status: 400, title: 'Bad Request', detail: 'server "x": unknown kind "ftp"' }),
    );
    await fireEvent.click(screen.getByRole('radio', { name: 'Import' }));
    await fireEvent.input(screen.getByLabelText('A file exported from a viewer (YAML)'), {
      target: { value: 'x' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Import' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('unknown kind "ftp"');
  });
});
