import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import HelpDialog from './HelpDialog.svelte';

describe('HelpDialog', () => {
  beforeEach(() => i18n.set('en'));

  it('opens on how it works and goes through the topics', async () => {
    render(HelpDialog, { open: true, onclose: vi.fn() });
    const dialog = screen.getByRole('dialog', { name: 'Help' });
    const topics = within(dialog).getByRole('navigation', { name: 'Help topics' });
    expect(within(topics).getByRole('button', { name: /How it works/ })).toHaveAttribute(
      'aria-current',
      'true',
    );
    expect(within(dialog).getByRole('heading', { name: 'How it works' })).toBeInTheDocument();

    await fireEvent.click(within(topics).getByRole('button', { name: /Servers: SSH and HTTP/ }));
    expect(
      within(dialog).getByRole('heading', { name: 'Servers: SSH and HTTP' }),
    ).toBeInTheDocument();
    // Marked parts of a text become code and strong text, not markup characters
    expect(within(dialog).getByText('~/.ssh/config').tagName).toBe('CODE');
    expect(dialog.textContent).not.toContain('**');
  });

  it('opens on the topic it is asked for', () => {
    render(HelpDialog, { open: true, topic: 'library', onclose: vi.fn() });
    expect(screen.getByRole('heading', { name: 'Connect the library' })).toBeInTheDocument();
  });

  it('copies a code sample', async () => {
    const writeText = vi.fn(async () => {});
    Object.assign(navigator, { clipboard: { writeText } });
    render(HelpDialog, { open: true, topic: 'library', onclose: vi.fn() });
    await fireEvent.click(screen.getAllByRole('button', { name: 'Copy' })[0] as HTMLElement);
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining('wapp-debugger.php'));
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument();
  });

  it('speaks Russian', () => {
    i18n.set('ru');
    render(HelpDialog, { open: true, onclose: vi.fn() });
    expect(screen.getByRole('dialog', { name: 'Справка' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Как это работает' })).toBeInTheDocument();
  });

  it('opens the list of shortcuts', async () => {
    const onshortcuts = vi.fn();
    render(HelpDialog, { open: true, topic: 'shortcuts', onclose: vi.fn(), onshortcuts });
    await fireEvent.click(screen.getByRole('button', { name: 'All shortcuts' }));
    expect(onshortcuts).toHaveBeenCalled();
  });
});
