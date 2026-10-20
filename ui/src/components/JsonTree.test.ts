import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import JsonTree from './JsonTree.svelte';

const value = {
  id: 42,
  name: 'Item',
  note: null,
  text_null: 'null',
  active: true,
  rows: [{ price: 9.5 }, 'second'],
  empty: {},
};

function row(key: string) {
  return screen.getByText(key, { selector: '.key' }).closest('li') as HTMLElement;
}

describe('JsonTree', () => {
  beforeEach(() => i18n.set('en'));

  it('shows keys and values with their JSON types', () => {
    render(JsonTree, { value });
    expect(within(row('id')).getByText('42')).toHaveAttribute('data-type', 'number');
    expect(within(row('name')).getByText('"Item"')).toHaveAttribute('data-type', 'string');
    expect(within(row('active')).getByText('true')).toHaveAttribute('data-type', 'boolean');
    // null and the string "null" must not look the same
    expect(within(row('note')).getByText('null')).toHaveAttribute('data-type', 'null');
    expect(within(row('text_null')).getByText('"null"')).toHaveAttribute('data-type', 'string');
    expect(within(row('empty')).getByText('{}')).toBeInTheDocument();
  });

  it('shows array items by index', () => {
    render(JsonTree, { value });
    expect(within(row('rows')).getByText('"second"')).toBeInTheDocument();
    expect(within(row('rows')).getByText('price')).toBeInTheDocument();
  });

  it('collapses and expands nested values', async () => {
    render(JsonTree, { value });
    const toggle = within(row('rows')).getByRole('button', { name: 'rows' });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');

    await fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(within(row('rows')).queryByText('"second"')).not.toBeInTheDocument();
    expect(within(row('rows')).getByText('2 items')).toBeInTheDocument();
  });

  it('starts collapsed below the given depth', () => {
    render(JsonTree, { value: { a: { b: { c: 1 } } }, expandDepth: 1 });
    expect(screen.queryByText('c', { selector: '.key' })).not.toBeInTheDocument();
    expect(screen.getByText('1 key')).toBeInTheDocument();
  });

  it('renders a scalar root', () => {
    render(JsonTree, { value: 'just text' });
    expect(screen.getByText('"just text"')).toHaveAttribute('data-type', 'string');
  });

  it('shows a big object in parts', async () => {
    const big = Object.fromEntries(Array.from({ length: 450 }, (_, i) => [`key${i}`, i]));
    render(JsonTree, { value: { big } });
    expect(screen.getByText('key199', { selector: '.key' })).toBeInTheDocument();
    expect(screen.queryByText('key200', { selector: '.key' })).not.toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Show 200 more of 250' }));
    expect(screen.getByText('key399', { selector: '.key' })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Show 50 more of 50' }));
    expect(screen.getByText('key449', { selector: '.key' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /more of/ })).not.toBeInTheDocument();
  });

  it('finds keys and values and shows the way to them', async () => {
    render(JsonTree, { value, searchable: true, expandDepth: 0 });
    const search = screen.getByRole('searchbox', { name: 'Search in the values' });
    await fireEvent.input(search, { target: { value: 'price' } });
    // The match is marked in its key; rows of what did not match are gone
    expect(screen.getByText('price', { selector: 'mark' })).toBeInTheDocument();
    expect(screen.getByText('rows', { selector: '.key' })).toBeInTheDocument();
    expect(screen.queryByText('id', { selector: '.key' })).not.toBeInTheDocument();
    await fireEvent.input(search, { target: { value: 'nothing here' } });
    expect(screen.getByText('Nothing found')).toBeInTheDocument();
    await fireEvent.input(search, { target: { value: '' } });
    expect(screen.getByText('id', { selector: '.key' })).toBeInTheDocument();
  });

  it('copies the path and the value of a node', async () => {
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    render(JsonTree, { value: { user: { id: 7 } } });
    await fireEvent.click(screen.getByRole('button', { name: 'Copy the path of id' }));
    expect(writeText).toHaveBeenLastCalledWith("['user']['id']");
    await fireEvent.click(screen.getByRole('button', { name: 'Copy the value of user' }));
    expect(writeText).toHaveBeenLastCalledWith('{\n  "id": 7\n}');
  });
});
