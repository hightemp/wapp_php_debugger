import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import Select from './Select.svelte';

const options = [
  { value: 1, label: 'local' },
  { value: 2, label: 'stage', hint: 'deploy@stage-web' },
  { value: 3, label: 'prod' },
];

function setup(value = 1) {
  const onchange = vi.fn();
  render(Select, { label: 'Server', options, value, onchange });
  return { onchange, combobox: screen.getByRole('combobox', { name: 'Server' }) };
}

describe('Select', () => {
  it('shows the chosen option and stays closed', () => {
    const { combobox } = setup(2);
    expect(combobox).toHaveTextContent('stage');
    expect(combobox).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('opens on click and picks an option', async () => {
    const { combobox, onchange } = setup(1);
    await fireEvent.click(combobox);
    expect(combobox).toHaveAttribute('aria-expanded', 'true');
    const listbox = screen.getByRole('listbox', { name: 'Server' });
    expect(listbox).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /local/ })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('option', { name: /stage/ })).toHaveTextContent('deploy@stage-web');

    await fireEvent.click(screen.getByRole('option', { name: /prod/ }));
    expect(onchange).toHaveBeenCalledWith(3);
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('works from the keyboard', async () => {
    const { combobox, onchange } = setup(1);
    combobox.focus();
    await fireEvent.keyDown(combobox, { key: 'ArrowDown' });
    expect(combobox).toHaveAttribute('aria-expanded', 'true');
    const active = () =>
      document.getElementById(combobox.getAttribute('aria-activedescendant') ?? '');
    expect(active()).toHaveTextContent('local');

    await fireEvent.keyDown(combobox, { key: 'ArrowDown' });
    await fireEvent.keyDown(combobox, { key: 'ArrowDown' });
    await fireEvent.keyDown(combobox, { key: 'ArrowDown' }); // stays on the last one
    expect(active()).toHaveTextContent('prod');
    await fireEvent.keyDown(combobox, { key: 'Home' });
    expect(active()).toHaveTextContent('local');
    await fireEvent.keyDown(combobox, { key: 'End' });
    await fireEvent.keyDown(combobox, { key: 'ArrowUp' });
    await fireEvent.keyDown(combobox, { key: 'Enter' });
    expect(onchange).toHaveBeenCalledWith(2);
    expect(combobox).toHaveAttribute('aria-expanded', 'false');
  });

  it('closes on Escape without a change', async () => {
    const { combobox, onchange } = setup(1);
    await fireEvent.keyDown(combobox, { key: 'Enter' });
    await fireEvent.keyDown(combobox, { key: 'ArrowDown' });
    await fireEvent.keyDown(combobox, { key: 'Escape' });
    expect(onchange).not.toHaveBeenCalled();
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });
});
