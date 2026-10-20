import { render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../lib/api/client';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport } from '../test/transport';
import CodePreview from './CodePreview.svelte';

describe('CodePreview', () => {
  beforeEach(() => i18n.set('en'));

  it('shows the lines around the place with the line marked and PHP highlighted', async () => {
    const { transport } = fakeTransport({
      getMessageSource: vi.fn(async () => ({
        file: 'app/Item.php',
        line: 11,
        first_line: 10,
        lines: ['<?php', 'class Item { /* <b>not markup</b> */ }', 'return 1;'],
      })),
    });
    render(CodePreview, { transport, messageId: 7, frame: 2 });

    const code = await screen.findByRole('region', { name: 'app/Item.php:11' });
    expect(transport.getMessageSource).toHaveBeenCalledWith(7, 2);
    expect([...(code.querySelector('.numbers')?.children ?? [])].map((n) => n.textContent)).toEqual(
      ['10', '11', '12'],
    );
    expect(code.querySelector('[data-current]')).toHaveTextContent('11');
    expect(code.querySelector('.hljs-keyword')).toHaveTextContent('class');
    // The code is text: markup in it is not rendered
    expect(code.querySelector('b')).toBeNull();
    expect(code).toHaveTextContent('<b>not markup</b>');
  });

  it('says why there is no code', async () => {
    const { transport } = fakeTransport({
      getMessageSource: vi.fn(async () => {
        throw new ApiError({
          status: 403,
          title: 'Forbidden',
          detail: 'the file is outside the project root',
        });
      }),
    });
    render(CodePreview, { transport, messageId: 7 });
    expect(await screen.findByRole('status')).toHaveTextContent('outside the project root');
  });
});
