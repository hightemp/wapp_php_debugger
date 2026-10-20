import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

interface Client {
  log(level: string, message: string, context?: Record<string, unknown>): void;
  flush(): void;
  end(): void;
}
interface Starter {
  start(options: Record<string, unknown>): Client;
  parseStack(stack: string): { file?: string; line?: number; function?: string }[];
}

declare global {
  var WappDebugger: Starter;
}

const fixture = readFileSync(resolve(__dirname, '../../../spec/fixtures/v2/js-page.jsonl'), 'utf8')
  .trim()
  .split('\n')
  .map((line) => JSON.parse(line) as Record<string, unknown>);

beforeAll(async () => {
  // @ts-expect-error a plain script for pages, not a module
  await import('../../public/debugger.js');
});

describe('debugger.js', () => {
  const clients: Client[] = [];
  afterEach(() => {
    for (const client of clients.splice(0)) {
      client.end();
    }
    vi.restoreAllMocks();
  });

  function start(sent: string[], extra: Record<string, unknown> = {}) {
    const client = WappDebugger.start({
      endpoint: 'http://viewer.lan:8821/ingest/v2',
      token: 'tok_js',
      parentId: '01JAZ7Q5X0K4M8T2V6N9P3R5S7',
      url: 'https://shop.example.test/catalog/item?id=42',
      id: () => '01JAZ7Q5X0K4M8T2V6N9P3R5SB',
      now: () => 1792400000400,
      elapsed: () => 2500,
      send: (url: string, body: string) => sent.push(`${url}\n${body}`),
      ...extra,
    });
    clients.push(client);
    return client;
  }

  it('writes the events of the page as the format wants them', () => {
    const sent: string[] = [];
    const client = start(sent);
    const error = new TypeError("Cannot read properties of undefined (reading 'price')");
    error.stack = [
      "TypeError: Cannot read properties of undefined (reading 'price')",
      '    at render (https://shop.example.test/assets/app.js:120:15)',
      '    at onClick (https://shop.example.test/assets/app.js:88:3)',
    ].join('\n');
    window.dispatchEvent(new ErrorEvent('error', { error, message: error.message }));
    const rejection = new Event('unhandledrejection') as Event & { reason: unknown };
    rejection.reason = new Error('Network down');
    window.dispatchEvent(rejection);
    vi.spyOn(console, 'info'); // untouched: only the levels asked for are taken
    console.warn('Deprecated API:', 'use fetch');
    client.end();

    expect(sent).toHaveLength(1);
    const [url, ...lines] = (sent[0] ?? '').trimEnd().split('\n');
    expect(url).toBe('http://viewer.lan:8821/ingest/v2?token=tok_js');
    expect(lines.map((line) => JSON.parse(line) as unknown)).toEqual(fixture);
  });

  it('reads the stack of Firefox and Safari too', () => {
    expect(
      WappDebugger.parseStack(
        'render@https://shop.test/app.js:120:15\n@https://shop.test/app.js:7:1',
      ),
    ).toEqual([
      { file: 'https://shop.test/app.js', line: 120, function: 'render' },
      { file: 'https://shop.test/app.js', line: 7 },
    ]);
  });

  it('sends a batch soon after the last event and nothing after the end', () => {
    vi.useFakeTimers();
    const sent: string[] = [];
    const client = start(sent);
    client.log('info', 'Cart opened', { items: 2 });
    expect(sent).toHaveLength(0);
    vi.advanceTimersByTime(600);
    expect(sent).toHaveLength(1);
    expect(sent[0]).toContain('"type":"request_start"');
    client.end();
    client.log('info', 'too late');
    client.flush();
    expect(sent).toHaveLength(2);
    vi.useRealTimers();
  });
});
