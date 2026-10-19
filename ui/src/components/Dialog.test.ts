import { fireEvent, render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import ConfirmDialog from './ConfirmDialog.svelte';
import Dialog from './Dialog.svelte';
import Switch from './Switch.svelte';

const content = createRawSnippet(() => ({
  render: () => '<div><input aria-label="Name" /><button type="button">Inner</button></div>',
}));

describe('Dialog', () => {
  beforeEach(() => i18n.set('en'));

  it('is a labelled modal that takes the focus', async () => {
    render(Dialog, { open: true, title: 'Project', onclose: vi.fn(), children: content });
    const dialog = screen.getByRole('dialog', { name: 'Project' });
    expect(dialog).toHaveAttribute('aria-modal', 'true');
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(dialog.contains(document.activeElement)).toBe(true);
  });

  it('closes on Escape and on the close button', async () => {
    const onclose = vi.fn();
    render(Dialog, { open: true, title: 'Project', onclose, children: content });
    await fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
    await fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(onclose).toHaveBeenCalledTimes(2);
  });

  it('closes on Escape even when the focus is lost', async () => {
    const onclose = vi.fn();
    render(Dialog, { open: true, title: 'Project', onclose, children: content });
    (document.activeElement as HTMLElement | null)?.blur();
    await fireEvent.keyDown(document.body, { key: 'Escape' });
    expect(onclose).toHaveBeenCalledTimes(1);
  });

  it('closes only the top dialog on Escape', async () => {
    const outer = vi.fn();
    const inner = vi.fn();
    render(Dialog, { open: true, title: 'Servers', onclose: outer, children: content });
    render(Dialog, { open: true, title: 'New server', onclose: inner, children: content });
    await fireEvent.keyDown(screen.getByRole('dialog', { name: 'New server' }), { key: 'Escape' });
    expect(inner).toHaveBeenCalledTimes(1);
    expect(outer).not.toHaveBeenCalled();
  });

  it('brings a lost focus back on Tab', async () => {
    render(Dialog, { open: true, title: 'Project', onclose: vi.fn(), children: content });
    (document.activeElement as HTMLElement | null)?.blur();
    await fireEvent.keyDown(document.body, { key: 'Tab' });
    expect(screen.getByRole('dialog').contains(document.activeElement)).toBe(true);
  });

  it('keeps Tab inside the dialog', async () => {
    render(Dialog, { open: true, title: 'Project', onclose: vi.fn(), children: content });
    const close = screen.getByRole('button', { name: 'Close' });
    const inner = screen.getByRole('button', { name: 'Inner' });
    await new Promise((resolve) => setTimeout(resolve, 0)); // the dialog focuses its first field on open
    inner.focus();
    await fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Tab' });
    expect(document.activeElement).toBe(close);
  });

  it('renders nothing when closed', () => {
    render(Dialog, { open: false, title: 'Project', onclose: vi.fn(), children: content });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});

describe('ConfirmDialog', () => {
  beforeEach(() => i18n.set('en'));

  it('asks and reports the answer', async () => {
    const onconfirm = vi.fn();
    const oncancel = vi.fn();
    render(ConfirmDialog, {
      open: true,
      title: 'Delete logs?',
      message: 'All log files of shop will be deleted.',
      confirmLabel: 'Delete',
      danger: true,
      onconfirm,
      oncancel,
    });
    expect(screen.getByRole('alertdialog', { name: 'Delete logs?' })).toHaveTextContent(
      'All log files of shop will be deleted.',
    );
    await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(oncancel).toHaveBeenCalledTimes(1);
    expect(onconfirm).toHaveBeenCalledTimes(1);
  });
});

describe('Switch', () => {
  it('is a switch that toggles', async () => {
    const onchange = vi.fn();
    render(Switch, { label: 'Debugger', checked: false, onchange });
    const control = screen.getByRole('switch', { name: 'Debugger' });
    expect(control).toHaveAttribute('aria-checked', 'false');
    await fireEvent.click(control);
    expect(onchange).toHaveBeenCalledWith(true);
  });

  it('can be disabled', () => {
    render(Switch, { label: 'Debugger', checked: true, disabled: true, onchange: vi.fn() });
    expect(screen.getByRole('switch', { name: 'Debugger' })).toBeDisabled();
  });
});
