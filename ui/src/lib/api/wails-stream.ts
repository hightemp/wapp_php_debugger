// В десктопе asset handler Wails буферизует ответ целиком, и SSE до окна не доходит
// (п. 10.27 плана, проверено самопроверкой). События идут через Wails Stream "events";
// адаптер делает из него EventSourceLike, поэтому остальной UI не знает о разнице.
import type { EventSourceLike } from './client';

/** Нужная часть JSONStream из /wails/runtime.js. */
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

/** Имя потока на стороне Go: desktop.StreamName. */
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
      // Как Last-Event-ID у EventSource: сервер повторит пропущенное или пришлёт resync
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

/** Страница открыта в окне Wails: у него своё происхождение на каждой ОС. */
export function isDesktop(
  location: Pick<Location, 'protocol' | 'hostname'> = globalThis.location,
): boolean {
  return location.protocol === 'wails:' || location.hostname === 'wails.localhost';
}

/** JSONStream из рантайма Wails; путь существует только внутри окна десктопа. */
export async function openWailsStream(name: string): Promise<JSONStreamLike> {
  const runtimeUrl = '/wails/runtime.js';
  const runtime = (await import(/* @vite-ignore */ runtimeUrl)) as {
    JSONStream: (name: string) => JSONStreamLike;
  };
  return runtime.JSONStream(name);
}
