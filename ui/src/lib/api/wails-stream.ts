// In the desktop app the Wails asset handler buffers whole responses, so SSE never reaches the window
// (found by the self-test). Events go through the Wails stream "events";
// this adapter turns it into an EventSourceLike, so the rest of the UI sees no difference.
import type { EventSourceLike } from './client';

/** The part of JSONStream from /wails/runtime.js that is needed. */
export interface JSONStreamLike {
  onopen: (() => void) | null;
  onclose: (() => void) | null;
  onmessage: ((event: { data: unknown }) => void) | null;
  send(value: unknown): void;
  close(): void;
}

interface Frame {
  id: number;
  name: string;
  data: unknown;
}

/** Stream name on the Go side: desktop.StreamName. */
export const wailsStreamName = 'events';

export function createWailsEventSource(
  openStream: (name: string) => Promise<JSONStreamLike>,
  options: { reconnectDelayMs?: number } = {},
): (url: string) => EventSourceLike {
  const reconnectDelayMs = options.reconnectDelayMs ?? 1000;

  return () => {
    const listeners = new Map<string, ((event: MessageEvent) => void)[]>();
    let lastEventId = 0;
    let closed = false;
    let current: JSONStreamLike | null = null;

    const connect = async () => {
      const stream = await openStream(wailsStreamName);
      if (closed) {
        stream.close();
        return;
      }
      current = stream;
      // Like Last-Event-ID in EventSource: the server replays what was missed or sends resync
      stream.onopen = () => stream.send({ last_event_id: lastEventId });
      stream.onmessage = ({ data }) => {
        const frame = data as Frame;
        lastEventId = frame.id;
        const event = new MessageEvent(frame.name, { data: JSON.stringify(frame.data) });
        for (const listener of listeners.get(frame.name) ?? []) {
          listener(event);
        }
      };
      stream.onclose = () => {
        current = null;
        if (!closed) {
          setTimeout(() => void connect(), reconnectDelayMs);
        }
      };
    };
    void connect();

    return {
      addEventListener(name, listener) {
        listeners.set(name, [...(listeners.get(name) ?? []), listener]);
      },
      close() {
        closed = true;
        current?.close();
      },
    };
  };
}

/** The page runs inside a Wails window: Wails uses its own origin on each OS. */
export function isDesktop(
  location: Pick<Location, 'protocol' | 'hostname'> = globalThis.location,
): boolean {
  return location.protocol === 'wails:' || location.hostname === 'wails.localhost';
}

/** JSONStream from the Wails runtime; the path exists only inside the desktop window. */
export async function openWailsStream(name: string): Promise<JSONStreamLike> {
  const runtimeUrl = '/wails/runtime.js';
  const runtime = (await import(/* @vite-ignore */ runtimeUrl)) as {
    JSONStream: (name: string) => JSONStreamLike;
  };
  return runtime.JSONStream(name);
}
