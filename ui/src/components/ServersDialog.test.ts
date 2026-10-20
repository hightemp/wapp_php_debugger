import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Server } from '../lib/api/types';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport, sampleServer } from '../test/transport';
import ServersDialog from './ServersDialog.svelte';

const fingerprint = 'SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8';

function setup(servers: Server[] = [sampleServer(1), sampleServer(2, { name: 'stage' })]) {
  let list = servers;
  const fake = fakeTransport({
    listServers: vi.fn(async () => ({ items: list })),
    createServer: vi.fn(async (input) => {
      const created = sampleServer(3, { name: input.name, config: input.config ?? {} });
      list = [...list, created];
      return created;
    }),
    deleteServer: vi.fn(async (id: number) => {
      list = list.filter((server) => server.id !== id);
    }),
  });
  const onchange = vi.fn();
  render(ServersDialog, { open: true, transport: fake.transport, onclose: vi.fn(), onchange });
  return { ...fake, onchange };
}

const row = async (name: string) => (await screen.findByRole('listitem', { name })) as HTMLElement;

describe('ServersDialog', () => {
  beforeEach(() => i18n.set('en'));

  it('lists servers with their connection', async () => {
    setup([
      sampleServer(1),
      sampleServer(2, {
        name: 'stage',
        connection: { state: 'offline', error: 'connection refused' },
      }),
    ]);
    const stage = await row('stage');
    expect(stage).toHaveTextContent('deploy@stage-2');
    expect(stage).toHaveTextContent('Offline');
    expect(stage).toHaveTextContent('connection refused');
    expect(
      within(await row('local')).queryByRole('button', { name: 'Remove' }),
    ).not.toBeInTheDocument();
  });

  it('adds an SSH server', async () => {
    const { transport, onchange } = setup();
    await fireEvent.click(await screen.findByRole('button', { name: 'Add server' }));
    const form = screen.getByRole('dialog', { name: 'New server' });
    await fireEvent.input(within(form).getByLabelText('Name'), { target: { value: 'prod' } });
    await fireEvent.input(within(form).getByLabelText('Host or ~/.ssh/config alias'), {
      target: { value: 'prod-web' },
    });
    await fireEvent.input(within(form).getByLabelText('User'), { target: { value: 'deploy' } });
    await fireEvent.input(within(form).getByLabelText('Jump hosts'), {
      target: { value: 'bastion, ops@gate:2200' },
    });
    await fireEvent.click(within(form).getByRole('radio', { name: 'prod' }));
    await fireEvent.click(within(form).getByRole('switch', { name: 'Read only' }));
    await fireEvent.click(within(form).getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(transport.createServer).toHaveBeenCalledWith({
        name: 'prod',
        kind: 'ssh',
        environment: 'prod',
        config: { host: 'prod-web', user: 'deploy', jump_hosts: ['bastion', 'ops@gate:2200'] },
        permissions: { read_only: true, allow_delete: false, allow_control: false },
      }),
    );
    expect(await row('prod')).toBeInTheDocument();
    expect(onchange).toHaveBeenCalled();
  });

  it('adds a server in the library HTTP mode, warning about plain http', async () => {
    const { transport } = setup();
    await fireEvent.click(await screen.findByRole('button', { name: 'Add server' }));
    const form = screen.getByRole('dialog', { name: 'New server' });
    await fireEvent.click(within(form).getByRole('radio', { name: 'HTTP' }));
    expect(within(form).queryByLabelText('User')).not.toBeInTheDocument();
    await fireEvent.input(within(form).getByLabelText('Name'), { target: { value: 'hosting' } });
    const address = within(form).getByLabelText('Application address');
    await fireEvent.input(address, { target: { value: 'http://shop.example.test' } });
    expect(within(form).getByText(/unencrypted/)).toBeInTheDocument();
    await fireEvent.input(address, { target: { value: 'https://shop.example.test' } });
    expect(within(form).queryByText(/unencrypted/)).not.toBeInTheDocument();

    const save = within(form).getByRole('button', { name: 'Save' });
    const token = within(form).getByLabelText('Token (http_token)');
    expect(token).toHaveAttribute('type', 'password');
    await fireEvent.input(token, { target: { value: 'short' } });
    expect(save).toBeDisabled();
    await fireEvent.input(token, { target: { value: '0123456789abcdef0123456789abcdef' } });
    await fireEvent.click(save);

    await waitFor(() =>
      expect(transport.createServer).toHaveBeenCalledWith({
        name: 'hosting',
        kind: 'http',
        environment: 'dev',
        config: { base_url: 'https://shop.example.test' },
        token: '0123456789abcdef0123456789abcdef',
      }),
    );
  });

  it('shows the address of an http server', async () => {
    setup([
      sampleServer(1),
      sampleServer(4, {
        name: 'hosting',
        kind: 'http',
        config: { base_url: 'https://shop.example.test' },
      }),
    ]);
    expect(await row('hosting')).toHaveTextContent('https://shop.example.test');
  });

  it('checks a server and trusts an unknown host key after confirmation', async () => {
    const { transport } = setup();
    vi.mocked(transport.checkServer)
      .mockResolvedValueOnce({
        ok: false,
        steps: [{ name: 'connect', ok: false, detail: 'unknown host 10.0.0.5:22' }],
        host_key_fingerprint: fingerprint,
        host_key_type: 'ecdsa-sha2-nistp256',
      })
      .mockResolvedValueOnce({
        ok: true,
        steps: [
          { name: 'connect', ok: true },
          { name: 'sftp', ok: true },
        ],
      });

    const stage = await row('stage');
    await fireEvent.click(within(stage).getByRole('button', { name: 'Check' }));
    expect(await within(stage).findByText(fingerprint)).toBeInTheDocument();
    expect(stage).toHaveTextContent('ECDSA');
    // the hint names the file of this key type on the server
    expect(stage).toHaveTextContent('ssh-keygen -lf /etc/ssh/ssh_host_ecdsa_key.pub');

    await fireEvent.click(within(stage).getByRole('button', { name: 'Trust this key' }));
    await waitFor(() => expect(transport.trustHostKey).toHaveBeenCalledWith(2, fingerprint));
    await waitFor(() => expect(transport.checkServer).toHaveBeenCalledTimes(2));
    expect(await within(stage).findByText('Connection works')).toBeInTheDocument();
  });

  it('shows the steps of each project, warnings apart from failures', async () => {
    const { transport } = setup();
    vi.mocked(transport.checkServer).mockResolvedValueOnce({
      ok: false,
      steps: [
        { name: 'connect', ok: true, detail: 'deploy@stage:22' },
        { name: 'sftp', ok: true },
        { name: 'logs_dir', ok: true, project: 'shop', detail: '/var/www/shop/logs' },
        { name: 'library_config', ok: false, optional: true, project: 'shop', detail: 'not found' },
        { name: 'logs_dir', ok: false, project: 'blog', detail: 'permission denied' },
        { name: 'php', ok: true, optional: true, detail: 'PHP 7.4.33 (cli)' },
      ],
    });
    const stage = await row('stage');
    await fireEvent.click(within(stage).getByRole('button', { name: 'Check' }));
    const step = async (text: string) =>
      (await within(stage).findByText(text, { exact: false })).closest('li');
    expect(await step('blog · Logs directory')).toHaveAttribute('data-status', 'failed');
    expect(await step('shop · Library config')).toHaveAttribute('data-status', 'warning');
    expect(await step('shop · Logs directory')).toHaveAttribute('data-status', 'ok');
    expect(await step('PHP 7.4.33 (cli)')).toHaveAttribute('data-status', 'ok');
    expect(stage).not.toHaveTextContent('Connection works');
  });

  it('keeps the steps on success, warnings included', async () => {
    const { transport } = setup();
    vi.mocked(transport.checkServer).mockResolvedValueOnce({
      ok: true,
      steps: [
        { name: 'connect', ok: true },
        { name: 'php', ok: false, optional: true, detail: 'php -v: command not found' },
      ],
    });
    const stage = await row('stage');
    await fireEvent.click(within(stage).getByRole('button', { name: 'Check' }));
    expect(await within(stage).findByText('Connection works')).toBeInTheDocument();
    expect((await within(stage).findByText(/command not found/)).closest('li')).toHaveAttribute(
      'data-status',
      'warning',
    );
  });

  it('shows a server clock that is off', async () => {
    setup([
      sampleServer(1),
      sampleServer(2, { name: 'stage', connection: { state: 'online', clock_offset_ms: -2600 } }),
    ]);
    expect(await row('stage')).toHaveTextContent('clock −3 s');
  });

  it('follows the connection live', async () => {
    const { emit } = setup();
    const stage = await row('stage');
    emit({ event: 'server_status', server_id: 2, connection: { state: 'online' } });
    await waitFor(() => expect(stage).toHaveTextContent('Online'));
  });

  it('removes a server after confirmation', async () => {
    const { transport, onchange } = setup();
    await fireEvent.click(within(await row('stage')).getByRole('button', { name: 'Remove' }));
    const confirm = screen.getByRole('alertdialog', { name: 'Remove stage?' });
    await fireEvent.click(within(confirm).getByRole('button', { name: 'Remove' }));
    await waitFor(() => expect(transport.deleteServer).toHaveBeenCalledWith(2));
    await waitFor(() =>
      expect(screen.queryByRole('listitem', { name: 'stage' })).not.toBeInTheDocument(),
    );
    expect(onchange).toHaveBeenCalled();
  });
});
