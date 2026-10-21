import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import ColorField from './ColorField.svelte';

describe('ColorField', () => {
  beforeEach(() => i18n.set('en'));

  function setup(value = '#2f6fed') {
    const onchange = vi.fn();
    render(ColorField, { label: 'accent', value, onchange });
    return { onchange };
  }

  it('takes a typed color once it is one', async () => {
    const { onchange } = setup();
    const input = screen.getByRole('textbox', { name: 'Color of accent' });
    await fireEvent.input(input, { target: { value: '#d14d0' } });
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(onchange).not.toHaveBeenCalled();
    await fireEvent.input(input, { target: { value: '#D14D00' } });
    expect(input).toHaveAttribute('aria-invalid', 'false');
    expect(onchange).toHaveBeenCalledWith('#d14d00');
  });

  it('picks a color with hue, saturation and lightness', async () => {
    const { onchange } = setup('#ff0000');
    const pick = screen.getByRole('button', { name: 'Pick the color of accent' });
    expect(pick).toHaveAttribute('aria-expanded', 'false');
    await fireEvent.click(pick);
    expect(pick).toHaveAttribute('aria-expanded', 'true');

    const lightness = screen.getByRole('slider', { name: 'Lightness' });
    expect(lightness).toHaveValue('50');
    await fireEvent.input(lightness, { target: { value: '25' } });
    expect(onchange).toHaveBeenLastCalledWith('#800000');
    await fireEvent.input(screen.getByRole('slider', { name: 'Hue' }), {
      target: { value: '120' },
    });
    expect(onchange).toHaveBeenLastCalledWith('#008000');
  });

  it('closes the picker on Escape without closing a dialog around it', async () => {
    setup();
    await fireEvent.click(screen.getByRole('button', { name: 'Pick the color of accent' }));
    const event = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
    screen.getByRole('slider', { name: 'Hue' }).dispatchEvent(event);
    await Promise.resolve();
    expect(screen.queryByRole('slider', { name: 'Hue' })).not.toBeInTheDocument();
    expect(event.defaultPrevented).toBe(true);
  });
});
