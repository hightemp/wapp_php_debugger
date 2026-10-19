import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../lib/api/client';
import { i18n } from '../lib/i18n/locale.svelte';
import { sampleProject } from '../test/transport';
import ProjectDialog from './ProjectDialog.svelte';

async function type(label: string, value: string) {
  await fireEvent.input(screen.getByLabelText(label), { target: { value } });
}

describe('ProjectDialog', () => {
  beforeEach(() => i18n.set('en'));

  it('creates a project from the form', async () => {
    const onsave = vi.fn(async () => {});
    render(ProjectDialog, { open: true, onsave, onclose: vi.fn() });
    const save = screen.getByRole('button', { name: 'Save' });
    expect(save).toBeDisabled();

    await type('Name', 'shop');
    await type('Project root', '/var/www/shop');
    await type('Logs directory', '/var/www/shop/tmp/logs');
    await fireEvent.click(screen.getByRole('radio', { name: 'PhpStorm' }));
    await fireEvent.click(save);

    expect(onsave).toHaveBeenCalledWith({
      name: 'shop',
      root_path: '/var/www/shop',
      logs_dir: '/var/www/shop/tmp/logs',
      editor: 'phpstorm',
    });
  });

  it('edits an existing project', async () => {
    const onsave = vi.fn(async () => {});
    const project = sampleProject(3, { name: 'shop', local_path: '/home/dev/shop' });
    render(ProjectDialog, { open: true, project, onsave, onclose: vi.fn() });
    expect(screen.getByRole('dialog', { name: 'Project settings' })).toBeInTheDocument();
    expect(screen.getByLabelText('Name')).toHaveValue('shop');
    expect(screen.getByLabelText('Local copy for IDE links')).toHaveValue('/home/dev/shop');

    await type('Local copy for IDE links', '');
    await fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(onsave).toHaveBeenCalledWith(expect.objectContaining({ name: 'shop', local_path: '' }));
  });

  it('shows the server answer when saving fails', async () => {
    const onsave = vi.fn(async () => {
      throw new ApiError({
        status: 409,
        title: 'Conflict',
        detail: 'a project with this name already exists',
      });
    });
    render(ProjectDialog, { open: true, project: sampleProject(3), onsave, onclose: vi.fn() });
    await fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'a project with this name already exists',
    );
  });
});
