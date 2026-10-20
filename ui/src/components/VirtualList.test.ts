import { render, screen } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';
import { describe, expect, it } from 'vitest';
import VirtualList from './VirtualList.svelte';

const row = createRawSnippet((item: () => number, index: () => number) => ({
  render: () => `<li aria-posinset="${index() + 1}">row ${item()}</li>`,
}));

describe('VirtualList', () => {
  it('renders the first rows only when there is no layout to measure', () => {
    const items = Array.from({ length: 1000 }, (_, i) => i);
    render(VirtualList<number>, {
      items,
      key: (item: number) => item,
      row,
      label: 'Rows',
      initial: 40,
    });
    const list = screen.getByRole('list', { name: 'Rows' });
    expect(list.querySelectorAll('li')).toHaveLength(40);
    expect(screen.getByText('row 0')).toBeInTheDocument();
  });

  it('renders a short list whole', () => {
    render(VirtualList<number>, { items: [1, 2, 3], key: (item: number) => item, row });
    expect(screen.getAllByText(/row/)).toHaveLength(3);
  });
});
