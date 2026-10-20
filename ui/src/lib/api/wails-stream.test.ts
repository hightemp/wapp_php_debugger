import { describe, expect, it, vi } from 'vitest';
import {
  createWailsEventSource,
  isDesktop,
  openInBrowser,
  type JSONStreamLike,
} from './wails-stream';

class FakeStream implements JSONStreamLike {
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onmessage: ((event: { data: unknown }) => void) | null = null;
  sent: unknown[] = [];
  closed = false;

  send(value: unknown) {
    this.sent.push(value);
  }

  close() {
    this.closed = true;
  }
}

function setup() {
  const streams: FakeStream[] = [];
  const openStream = vi.fn(async () => {
    const stream = new FakeStream();
    streams.push(stream);
    return stream;
  });
  return { streams, openStream };
}

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe('createWailsEventSource', () => {
  it('says hello with last_event_id and dispatches frames as named events', async () => {
    const { streams, openStream } = setup();
    const source = createWailsEventSource(openStream, { reconnectDelayMs: 1 })('/api/v1/stream');
    const received: string[] = [];
    source.addEventListener('request_created', (event) => received.push(String(event.data)));
    await flush();

    const stream = streams[0] as FakeStream;
    stream.onopen?.();
    expect(stream.sent).toEqual([{ last_event_id: 0 }]);

    stream.onmessage?.({ data: { id: 5, name: 'request_created', data: { request_id: 7 } } });
    stream.onmessage?.({ data: { id: 6, name: 'other', data: {} } });
    expect(received).toEqual(['{"request_id":7}']);
    expect(openStream).toHaveBeenCalledWith('events');
  });

  it('reconnects after a drop and resumes after the last seen id', async () => {
    const { streams, openStream } = setup();
    createWailsEventSource(openStream, { reconnectDelayMs: 1 })('/api/v1/stream');
    await flush();
    const first = streams[0] as FakeStream;
    first.onopen?.();
    first.onmessage?.({ data: { id: 9, name: 'resync', data: {} } });
    first.onclose?.();

    await new Promise((resolve) => setTimeout(resolve, 10));
    const second = streams[1] as FakeStream;
    second.onopen?.();
    expect(second.sent).toEqual([{ last_event_id: 9 }]);
  });

  it('stops for good on close', async () => {
    const { streams, openStream } = setup();
    const source = createWailsEventSource(openStream, { reconnectDelayMs: 1 })('/api/v1/stream');
    await flush();
    source.close();
    (streams[0] as FakeStream).onclose?.();
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(streams).toHaveLength(1);
    expect((streams[0] as FakeStream).closed).toBe(true);
  });
});

describe('isDesktop', () => {
  it('recognises the Wails window origin on every OS', () => {
    // Linux and macOS: wails://localhost (seen in the self-test); Windows: http://wails.localhost
    expect(isDesktop({ protocol: 'wails:', hostname: 'localhost' })).toBe(true);
    expect(isDesktop({ protocol: 'http:', hostname: 'wails.localhost' })).toBe(true);
    expect(isDesktop({ protocol: 'http:', hostname: '127.0.0.1' })).toBe(false);
    expect(isDesktop({ protocol: 'https:', hostname: 'debugger.example.test' })).toBe(false);
  });
});

describe('openInBrowser', () => {
  it('asks Wails to open the system browser', async () => {
    const OpenURL = vi.fn(async () => {});
    await openInBrowser('https://example.test/release', async () => ({ Browser: { OpenURL } }));
    expect(OpenURL).toHaveBeenCalledWith('https://example.test/release');
  });
});
