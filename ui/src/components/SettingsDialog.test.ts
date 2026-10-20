import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DesktopSettings } from '../lib/api/types';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport, sampleProject } from '../test/transport';
import SettingsDialog from './SettingsDialog.svelte';

const defaults: DesktopSettings = {
  notifications: { enabled: true, min_level: 'error', project_ids: [] },
  autostart: false,
  close_to_tray: true,
  update_check: true,
};

function setup() {
  const fake = fakeTransport({
    getDesktopSettings: vi.fn(async () => structuredClone(defaults)),
    updateDesktopSettings: vi.fn(async (settings: DesktopSettings) => settings),
    listProjects: vi.fn(async () => ({
      items: [sampleProject(1, { name: 'shop' }), sampleProject(2, { name: 'blog' })],
    })),
  });
  render(SettingsDialog, { open: true, transport: fake.transport, onclose: vi.fn() });
  return fake;
}

describe('SettingsDialog', () => {
  beforeEach(() => i18n.set('en'));

  it('saves a switched setting at once', async () => {
    const { transport } = setup();
    const autostart = await screen.findByRole('switch', { name: 'Start at login' });
    expect(autostart).toHaveAttribute('aria-checked', 'false');
    await fireEvent.click(autostart);
    await waitFor(() =>
      expect(transport.updateDesktopSettings).toHaveBeenLastCalledWith({
        ...defaults,
        autostart: true,
      }),
    );
  });

  it('notifies about warnings too, for chosen projects', async () => {
    const { transport } = setup();
    await fireEvent.click(await screen.findByRole('radio', { name: 'Errors and warnings' }));
    const projects = screen.getByRole('group', { name: 'Projects to notify about' });
    await fireEvent.click(within(projects).getByRole('switch', { name: 'All projects' }));
    // Off, every project is chosen one by one; blog is then left out
    await fireEvent.click(within(projects).getByRole('switch', { name: 'blog' }));
    await waitFor(() =>
      expect(transport.updateDesktopSettings).toHaveBeenLastCalledWith({
        ...defaults,
        notifications: { enabled: true, min_level: 'warning', project_ids: [1] },
      }),
    );
  });

  it('hides the notification options while notifications are off', async () => {
    setup();
    await fireEvent.click(await screen.findByRole('switch', { name: 'Notify about new errors' }));
    expect(screen.queryByRole('radio', { name: 'Errors and warnings' })).not.toBeInTheDocument();
  });
});
