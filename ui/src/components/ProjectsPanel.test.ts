import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport, sampleProject } from '../test/transport';
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
