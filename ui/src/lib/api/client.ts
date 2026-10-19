// Viewer API client (spec/openapi.yaml). Transport hides how the UI talks to the core:
// HTTP + SSE in the browser; in the desktop app the same API goes through the Wails asset handler.
import type {
  Level,
  MessageDetail,
  MessagePage,
  MessageType,
  Problem,
  Project,
  ProjectInput,
  ProjectPatch,
  RequestDetail,
  Server,
  ServerCheck,
  ServerInput,
  ServerPatch,
  RequestPage,
  StreamEvent,
} from './types';

export interface Version {
  version: string;
  commit: string;
  date: string;
}

export interface ListRequestsParams {
  project_id?: number;
  q?: string;
  has_errors?: boolean;
  sapi?: 'web' | 'cli';
  since?: number;
  until?: number;
  cursor?: string;
  limit?: number;
}

export interface ListMessagesParams {
  level?: Level[];
  type?: MessageType;
  tag?: string;
  q?: string;
  cursor?: string;
  limit?: number;
}

export interface Transport {
  getVersion(): Promise<Version>;
  listRequests(params: ListRequestsParams): Promise<RequestPage>;
  getRequest(id: number): Promise<RequestDetail>;
  listMessages(requestId: number, params: ListMessagesParams): Promise<MessagePage>;
  getMessage(id: number): Promise<MessageDetail>;
  listServers(): Promise<{ items: Server[] }>;
  createServer(input: ServerInput): Promise<Server>;
  updateServer(id: number, patch: ServerPatch): Promise<Server>;
  deleteServer(id: number): Promise<void>;
  /** Connects to the server now; an unknown host key comes back as host_key_fingerprint. */
  checkServer(id: number): Promise<ServerCheck>;
  /** Trusts the host key once the user compared its fingerprint with the server. */
  trustHostKey(id: number, fingerprint: string): Promise<void>;
  listProjects(): Promise<{ items: Project[] }>;
  createProject(input: ProjectInput): Promise<Project>;
  updateProject(id: number, patch: ProjectPatch): Promise<Project>;
  deleteProject(id: number): Promise<void>;
  getDebuggerState(id: number): Promise<{ enabled: boolean }>;
  setDebuggerState(id: number, enabled: boolean): Promise<{ enabled: boolean }>;
  clearProjectLogs(id: number): Promise<{ removed_files: number }>;
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

  async function request<T>(
    method: string,
    path: string,
    params: object = {},
    body?: unknown,
  ): Promise<T> {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== null) {
        // Arrays use the form style without explode: level=error,warning
        query.set(key, Array.isArray(value) ? value.join(',') : String(value));
      }
    }
    const url = `${options.baseUrl}${path}${query.size > 0 ? `?${query}` : ''}`;
    const init: RequestInit =
      body === undefined
        ? { method, headers: { Accept: 'application/json' } }
        : {
            method,
            headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
          };

    let response: Response;
    try {
      response = await fetchImpl(url, method === 'GET' ? { headers: init.headers } : init);
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
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  }

  const getJSON = <T>(path: string, params?: object) => request<T>('GET', path, params);

  const subscribers = new Set<(event: StreamEvent) => void>();
  let source: EventSourceLike | null = null;

  return {
    getVersion: () => getJSON<Version>('/api/v1/version'),
    listRequests: (params) => getJSON<RequestPage>('/api/v1/requests', params),
    getRequest: (id) => getJSON<RequestDetail>(`/api/v1/requests/${id}`),
    listMessages: (requestId, params) =>
      getJSON<MessagePage>(`/api/v1/requests/${requestId}/messages`, params),
    getMessage: (id) => getJSON<MessageDetail>(`/api/v1/messages/${id}`),
    listServers: () => getJSON<{ items: Server[] }>('/api/v1/servers'),
    createServer: (input) => request<Server>('POST', '/api/v1/servers', {}, input),
    updateServer: (id, patch) => request<Server>('PATCH', `/api/v1/servers/${id}`, {}, patch),
    deleteServer: async (id) => {
      await request<unknown>('DELETE', `/api/v1/servers/${id}`);
    },
    checkServer: (id) => request<ServerCheck>('POST', `/api/v1/servers/${id}/check`),
    trustHostKey: async (id, fingerprint) => {
      await request<unknown>('POST', `/api/v1/servers/${id}/host-key`, {}, { fingerprint });
    },
    listProjects: () => getJSON<{ items: Project[] }>('/api/v1/projects'),
    createProject: (input) => request<Project>('POST', '/api/v1/projects', {}, input),
    updateProject: (id, patch) => request<Project>('PATCH', `/api/v1/projects/${id}`, {}, patch),
    deleteProject: async (id) => {
      await request<unknown>('DELETE', `/api/v1/projects/${id}`);
    },
    getDebuggerState: (id) => getJSON<{ enabled: boolean }>(`/api/v1/projects/${id}/debugger`),
    setDebuggerState: (id, enabled) =>
      request<{ enabled: boolean }>('PUT', `/api/v1/projects/${id}/debugger`, {}, { enabled }),
    clearProjectLogs: (id) =>
      request<{ removed_files: number }>('DELETE', `/api/v1/projects/${id}/logs`),
    subscribe(onEvent) {
      // One EventSource for all subscribers; it reconnects by itself and sends Last-Event-ID
      subscribers.add(onEvent);
      if (!source) {
        const opened = createEventSource(`${options.baseUrl}/api/v1/stream`);
        for (const name of streamEventNames) {
          opened.addEventListener(name, (message) => {
            const event = JSON.parse(String(message.data)) as StreamEvent;
            for (const subscriber of [...subscribers]) {
              subscriber(event);
            }
          });
        }
        source = opened;
      }
      return () => {
        subscribers.delete(onEvent);
        if (subscribers.size === 0 && source) {
          source.close();
          source = null;
        }
      };
    },
  };
}
