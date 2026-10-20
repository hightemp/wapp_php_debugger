import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport, sampleProject, sampleServer } from '../test/transport';
import ProjectsPanel from './ProjectsPanel.svelte';

function setup(selectedId?: number) {
  let projects = [sampleProject(1, { name: 'shop' }), sampleProject(2, { name: 'blog' })];
  const fake = fakeTransport({
    listServers: vi.fn(async () => ({
      items: [
        {
          id: 7,
          name: 'local',
          kind: 'local' as const,
          environment: 'local' as const,
          config: {},
          permissions: { read_only: false, allow_delete: true, allow_control: true },
          connection: { state: 'online' as const },
          created_at: 0,
          updated_at: 0,
        },
      ],
    })),
    listProjects: vi.fn(async () => ({ items: projects })),
    createProject: vi.fn(async (input) => {
      const created = sampleProject(3, { name: input.name });
      projects = [...projects, created];
      return created;
    }),
    deleteProject: vi.fn(async (id: number) => {
      projects = projects.filter((project) => project.id !== id);
    }),
  });
  const onselect = vi.fn();
  render(ProjectsPanel, { transport: fake.transport, selectedId, onselect });
  return { ...fake, onselect };
}

describe('ProjectsPanel', () => {
  beforeEach(() => i18n.set('en'));

  it('lists projects and selects one or all', async () => {
    const { onselect } = setup();
    await fireEvent.click(await screen.findByRole('button', { name: 'blog' }));
    expect(onselect).toHaveBeenCalledWith(2);
    await fireEvent.click(screen.getByRole('button', { name: 'All projects' }));
    expect(onselect).toHaveBeenLastCalledWith(undefined);
  });

  it('switches the debugger of the selected project', async () => {
    const { transport } = setup(1);
    const toggle = await screen.findByRole('switch', { name: 'Debugger' });
    await waitFor(() => expect(transport.getDebuggerState).toHaveBeenCalledWith(1));
    await fireEvent.click(toggle);
    await waitFor(() => expect(transport.setDebuggerState).toHaveBeenCalledWith(1, true));
    expect(toggle).toHaveAttribute('aria-checked', 'true');
  });

  it('adds a project on the local server', async () => {
    const { transport } = setup();
    await fireEvent.click(await screen.findByRole('button', { name: 'Add project' }));
    const dialog = screen.getByRole('dialog', { name: 'New project' });
    await fireEvent.input(within(dialog).getByLabelText('Name'), { target: { value: 'api' } });
    await fireEvent.input(within(dialog).getByLabelText('Project root'), {
      target: { value: '/srv/api' },
    });
    await fireEvent.input(within(dialog).getByLabelText('Logs directory'), {
      target: { value: '/srv/api/logs' },
    });
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(transport.createProject).toHaveBeenCalledWith(
        expect.objectContaining({ server_id: 7, name: 'api' }),
      ),
    );
    expect(await screen.findByRole('button', { name: 'api' })).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('clears logs only after confirmation', async () => {
    const { transport } = setup(1);
    await fireEvent.click(await screen.findByRole('button', { name: 'Clear logs' }));
    const confirm = screen.getByRole('alertdialog', { name: 'Clear logs?' });
    expect(transport.clearProjectLogs).not.toHaveBeenCalled();
    await fireEvent.click(within(confirm).getByRole('button', { name: 'Clear' }));
    await waitFor(() => expect(transport.clearProjectLogs).toHaveBeenCalledWith(1));
  });

  it('removes the selected project after confirmation and resets the filter', async () => {
    const { transport, onselect } = setup(1);
    await fireEvent.click(await screen.findByRole('button', { name: 'Remove project' }));
    await fireEvent.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Remove' }),
    );
    await waitFor(() => expect(transport.deleteProject).toHaveBeenCalledWith(1));
    await waitFor(() => expect(onselect).toHaveBeenCalledWith(undefined));
    expect(screen.queryByRole('button', { name: 'shop' })).not.toBeInTheDocument();
  });

  it('opens the servers', async () => {
    setup();
    await fireEvent.click(await screen.findByRole('button', { name: 'Servers' }));
    expect(screen.getByRole('dialog', { name: 'Servers' })).toBeInTheDocument();
  });
});

describe('ProjectsPanel with several servers', () => {
  beforeEach(() => i18n.set('en'));

  function setupServers(selectedId?: number, onselectserver = vi.fn()) {
    const fake = fakeTransport({
      listServers: vi.fn(async () => ({
        items: [
          sampleServer(1),
          sampleServer(2, { name: 'stage', environment: 'stage' }),
          sampleServer(3, {
            name: 'prod-web',
            environment: 'prod',
            connection: { state: 'online' },
          }),
        ],
      })),
      listProjects: vi.fn(async () => ({
        items: [
          sampleProject(10, { name: 'local-shop', server_id: 1 }),
          sampleProject(20, { name: 'stage-shop', server_id: 2 }),
          sampleProject(30, { name: 'shop', server_id: 3 }),
        ],
      })),
    });
    const onselect = vi.fn();
    render(ProjectsPanel, { transport: fake.transport, selectedId, onselect, onselectserver });
    return { ...fake, onselect };
  }

  const group = async (name: string) => (await screen.findByRole('group', { name })) as HTMLElement;

  it('groups projects by server with the environment and the connection', async () => {
    setupServers();
    const prod = await group('prod-web');
    expect(within(prod).getByRole('button', { name: 'shop' })).toBeInTheDocument();
    expect(prod).toHaveTextContent('prod');
    expect(prod).toHaveTextContent('Online');
    expect(
      within(await group('stage')).getByRole('button', { name: 'stage-shop' }),
    ).toBeInTheDocument();
    expect(
      within(await group('local')).getByRole('button', { name: 'local-shop' }),
    ).toBeInTheDocument();
  });

  it('selects a server: its feed', async () => {
    const onselectserver = vi.fn();
    setupServers(undefined, onselectserver);
    await fireEvent.click(
      within(await group('prod-web')).getByRole('button', { name: 'prod-web' }),
    );
    expect(onselectserver).toHaveBeenCalledWith(3);
  });

  it('follows the connection of a server live', async () => {
    const { emit } = setupServers();
    const stage = await group('stage');
    emit({
      event: 'server_status',
      server_id: 2,
      connection: { state: 'offline', error: 'refused' },
    });
    await waitFor(() => expect(stage).toHaveTextContent('Offline'));
  });

  it('marks a project on prod', async () => {
    setupServers(30);
    expect(await screen.findByRole('status', { name: 'Environment' })).toHaveTextContent(
      'prod · prod-web',
    );
  });

  it('asks for the project name before clearing logs on prod', async () => {
    const { transport } = setupServers(30);
    await fireEvent.click(await screen.findByRole('button', { name: 'Clear logs' }));
    const confirm = screen.getByRole('alertdialog', { name: 'Clear logs?' });
    const clear = within(confirm).getByRole('button', { name: 'Clear' });
    expect(clear).toBeDisabled();
    const name = within(confirm).getByLabelText('Type shop to confirm');
    await fireEvent.input(name, { target: { value: 'sho' } });
    expect(clear).toBeDisabled();
    await fireEvent.input(name, { target: { value: 'shop' } });
    await fireEvent.click(clear);
    await waitFor(() => expect(transport.clearProjectLogs).toHaveBeenCalledWith(30));
  });

  it('asks for the project name before removing it from stage', async () => {
    setupServers(20);
    await fireEvent.click(await screen.findByRole('button', { name: 'Remove project' }));
    expect(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Remove' }),
    ).toBeDisabled();
  });

  it('confirms the debugger switch on prod', async () => {
    const { transport } = setupServers(30);
    await fireEvent.click(await screen.findByRole('switch', { name: 'Debugger' }));
    const confirm = screen.getByRole('alertdialog', { name: 'Turn the debugger on on prod?' });
    expect(transport.setDebuggerState).not.toHaveBeenCalled();
    await fireEvent.click(within(confirm).getByRole('button', { name: 'Turn on' }));
    await waitFor(() => expect(transport.setDebuggerState).toHaveBeenCalledWith(30, true));
  });

  it('switches the debugger of a local project at once', async () => {
    const { transport } = setupServers(10);
    await fireEvent.click(await screen.findByRole('switch', { name: 'Debugger' }));
    await waitFor(() => expect(transport.setDebuggerState).toHaveBeenCalledWith(10, true));
  });
});
