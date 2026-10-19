import { render, screen } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import App from './App.svelte';

describe('App', () => {
  it('renders the product name', () => {
    render(App);
    expect(screen.getByRole('heading', { name: 'wapp-debugger' })).toBeInTheDocument();
  });
});
