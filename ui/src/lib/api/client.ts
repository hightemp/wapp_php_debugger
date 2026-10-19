// Viewer API client (spec/openapi.yaml). Transport hides how the UI talks to the core:
// HTTP + SSE in the browser; in the desktop app the same API goes through the Wails asset handler.
import type { Problem, RequestPage, StreamEvent } from './types';

export interface Version {
  version: string;
  commit: string;
  date: string;
}

export interface ListRequestsParams {
  project_id?: number;
  cursor?: string;
  limit?: number;
}

export interface Transport {
  getVersion(): Promise<Version>;
  listRequests(params: ListRequestsParams): Promise<RequestPage>;
  /** Subscribes to /api/v1/stream; returns the unsubscribe function. */
  subscribe(onEvent: (event: StreamEvent) => void): () => void;
}

/** API error: Problem Details or a network failure (status 0). */
export class ApiError extends Error {
  readonly status: number;
  readonly title: string;
  readonly detail?: string;

  constructor(problem: Pick<Problem, 'status' | 'title' | 'detail'>) {
    super(problem.detail ? `${problem.title}: ${problem.detail}` : problem.title);
    this.name = 'ApiError';
    this.status = problem.status;
    this.title = problem.title;
    this.detail = problem.detail;
  }
}

export interface EventSourceLike {
  addEventListener(name: string, listener: (event: MessageEvent) => void): void;
  close(): void;
}

export interface HttpTransportOptions {
  baseUrl: string;
  fetch?: typeof fetch;
  createEventSource?: (url: string) => EventSourceLike;
}

const streamEventNames = [
  'request_created',
  'request_updated',
  'messages_added',
  'server_status',
  'resync',
] as const;

export function createHttpTransport(options: HttpTransportOptions): Transport {
  const fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
  const createEventSource =
    options.createEventSource ?? ((url: string) => new EventSource(url) as EventSourceLike);

  async function getJSON<T>(path: string, params: object = {}): Promise<T> {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== null) {
        query.set(key, String(value));
      }
    }
    const url = `${options.baseUrl}${path}${query.size > 0 ? `?${query}` : ''}`;

    let response: Response;
    try {
      response = await fetchImpl(url, { headers: { Accept: 'application/json' } });
    } catch (error) {
      throw new ApiError({ status: 0, title: 'Network error', detail: String(error) });
    }
    if (!response.ok) {
      const problem = (await response.json().catch(() => null)) as Problem | null;
      throw new ApiError({
        status: problem?.status ?? response.status,
        title: problem?.title ?? response.statusText,
        detail: problem?.detail,
      });
    }
    return (await response.json()) as T;
  }

  return {
    getVersion: () => getJSON<Version>('/api/v1/version'),
    listRequests: (params) => getJSON<RequestPage>('/api/v1/requests', params),
    subscribe(onEvent) {
      // EventSource reconnects by itself and sends Last-Event-ID
      const source = createEventSource(`${options.baseUrl}/api/v1/stream`);
      for (const name of streamEventNames) {
        source.addEventListener(name, (message) => {
          onEvent(JSON.parse(String(message.data)) as StreamEvent);
        });
      }
      return () => source.close();
    },
  };
}
