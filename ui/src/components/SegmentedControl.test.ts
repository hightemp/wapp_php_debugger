import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import { Moon, Sun } from '../lib/icons';
import SegmentedControl from './SegmentedControl.svelte';

describe('SegmentedControl', () => {
  it('keeps the names of options that show only an icon or a short label', async () => {
    const onchange = vi.fn();
    render(SegmentedControl, {
      label: 'Theme',
      compact: true,
      options: [
        { value: 'light', label: 'Light', icon: Sun },
        { value: 'dark', label: 'Dark', icon: Moon },
        { value: 'en', label: 'English', short: 'EN' },
      ],
      value: 'light',
      onchange,
    });
    const dark = screen.getByRole('radio', { name: 'Dark' });
    expect(dark).toHaveAttribute('title', 'Dark');
    expect(dark.textContent?.trim()).toBe('');
    expect(screen.getByRole('radio', { name: 'English' }).textContent?.trim()).toBe('EN');
    await fireEvent.click(dark);
    expect(onchange).toHaveBeenCalledWith('dark');
  });

  it('shows the label next to the icon when not compact', () => {
    render(SegmentedControl, {
      label: 'Kind',
      options: [{ value: 'cli', label: 'CLI', icon: Sun }],
      value: 'cli',
      onchange: vi.fn(),
    });
    const option = screen.getByRole('radio', { name: 'CLI' });
    expect(option.textContent?.trim()).toBe('CLI');
    expect(option.querySelector('svg')).not.toBeNull();
  });
});
