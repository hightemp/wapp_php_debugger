import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import Showcase from './Showcase.svelte';

describe('Showcase', () => {
  beforeEach(() => i18n.set('en'));

  it('shows every component in every built-in theme', () => {
    render(Showcase);
    const columns = screen.getAllByRole('region', { name: /^Theme / });
    expect(columns.map((column) => column.dataset.theme)).toEqual([
      'light',
      'dark',
      'high-contrast',
    ]);
    for (const column of columns) {
      expect(
        within(column).getByRole('radiogroup', { name: 'Density of lists' }),
      ).toBeInTheDocument();
      expect(within(column).getByRole('switch', { name: 'On' })).toBeInTheDocument();
      expect(within(column).getByRole('combobox', { name: 'Server' })).toBeInTheDocument();
      expect(within(column).getByRole('tablist', { name: 'Request' })).toBeInTheDocument();
      expect(within(column).getByRole('textbox', { name: 'Color of accent' })).toBeInTheDocument();
      // The variables tree and the preview of the viewer
      expect(within(column).getAllByText('"Blue mug"')).toHaveLength(2);
    }
  });

  it('narrows to one theme', async () => {
    render(Showcase);
    await fireEvent.click(screen.getByRole('radio', { name: 'Dark' }));
    expect(
      screen.getAllByRole('region', { name: /^Theme / }).map((column) => column.dataset.theme),
    ).toEqual(['dark']);
  });

  it('keeps the components working', async () => {
    render(Showcase);
    const column = screen.getAllByRole('region', { name: /^Theme / })[0] as HTMLElement;
    const toggle = within(column).getByRole('switch', { name: 'On' });
    await fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-checked', 'false');
  });
});
