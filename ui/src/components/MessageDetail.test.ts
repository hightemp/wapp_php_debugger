import { render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { i18n } from '../lib/i18n/locale.svelte';
import { fakeTransport, sampleMessage } from '../test/transport';
import MessageDetail from './MessageDetail.svelte';

describe('MessageDetail', () => {
  beforeEach(() => i18n.set('en'));

  it('shows the message, its source link, context and trace', async () => {
    const { transport } = fakeTransport({
      getMessage: vi.fn(async (id: number) =>
        sampleMessage(id, {
          level: 'info',
          message: 'Loading item',
          source: {
            file: 'app/Item.php',
            line: 31,
            editor_url: 'vscode://file/home/dev/shop/app/Item.php:31',
          },
          has_context: true,
          context: { item_id: 42 },
          trace: [
            {
              file: 'app/Item.php',
              line: 31,
              function: 'show',
              class: 'App\\Item',
              call_type: '->',
              editor_url: 'vscode://x/1',
            },
            { file: 'public/index.php', line: 12, function: 'dispatch' },
          ],
        }),
      ),
    });
    render(MessageDetail, { transport, messageId: 1001 });

    expect(await screen.findByText('Loading item')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'app/Item.php:31' })).toHaveAttribute(
      'href',
      'vscode://file/home/dev/shop/app/Item.php:31',
    );
    expect(screen.getByText('item_id', { selector: '.key' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /App\\Item->show/ })).toHaveAttribute(
      'href',
      'vscode://x/1',
    );
    expect(screen.getByText('public/index.php:12')).toBeInTheDocument();
  });

  it('shows exception details and previous exceptions', async () => {
    const { transport } = fakeTransport({
      getMessage: vi.fn(async (id: number) =>
        sampleMessage(id, {
          type: 'error',
          level: 'error',
          error_kind: 'exception',
          exception_class: 'App\\TransportException',
          code: '111',
          message: 'Connection refused',
          previous: [{ exception_class: 'ErrorException', message: 'unable to connect' }],
        }),
      ),
    });
    render(MessageDetail, { transport, messageId: 7 });
    expect(await screen.findByText('App\\TransportException')).toBeInTheDocument();
    expect(screen.getByText('code 111')).toBeInTheDocument();
    expect(screen.getByText('ErrorException')).toBeInTheDocument();
    expect(screen.getByText('unable to connect')).toBeInTheDocument();
  });

  it('says when there is no context', async () => {
    const { transport } = fakeTransport();
    render(MessageDetail, { transport, messageId: 1 });
    expect(await screen.findByText('No context')).toBeInTheDocument();
  });
});
