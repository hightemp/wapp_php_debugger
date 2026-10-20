// Viewer API client (spec/openapi.yaml). Transport hides how the UI talks to the core:
// HTTP + SSE in the browser; in the desktop app the same API goes through the Wails asset handler.
import type {
  DesktopSettings,
  DiscoveredProject,
  Level,
  MessageDetail,
  MessagePage,
  MessageType,
  Problem,
  Project,
  ProjectInput,
  ProjectPatch,
  Request,
  RequestDetail,
  Server,
  ServerCheck,
  ServerInput,
  ServerPatch,
  RequestPage,
  SourcePreview,
  StreamEvent,
} from './types';

export interface UpdateInfo {
  /** The check is on: the server with --update-check, the desktop with its setting */
  enabled: boolean;
  current: string;
  latest?: string;
  url?: string;
  newer: boolean;
  checked_at?: number;
}

export interface ImportResult {
  servers_created: string[];
  projects_created: string[];
  skipped: string[];
  needs_token: string[];
}

export interface Version {
  /** Which viewer answers: desktop settings exist only in the desktop app */
  app: 'server' | 'desktop';
  version: string;
  commit: string;
  date: string;
  /** Where the receiver of pushed logs listens; null when it is off */
  ingest_addr?: string | null;
}

export interface ListRequestsParams {
  project_id?: number;
  server_id?: number;
  environment?: Server['environment'];
  q?: string;
  has_errors?: boolean;
  sapi?: 'web' | 'cli';
  method?: string;
  /** A code (404) or a class (5xx) */
  status?: string;
  min_duration_ms?: number;
  is_ajax?: boolean;
  tag?: string;
  level?: 'error' | 'warning';
  pinned?: boolean;
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
  /** The last answer of the update check. */
  getUpdates(): Promise<UpdateInfo>;
  getDesktopSettings(): Promise<DesktopSettings>;
  updateDesktopSettings(settings: DesktopSettings): Promise<DesktopSettings>;
  listRequests(params: ListRequestsParams): Promise<RequestPage>;
  getRequest(id: number): Promise<RequestDetail>;
  listMessages(requestId: number, params: ListMessagesParams): Promise<MessagePage>;
  getMessage(id: number): Promise<MessageDetail>;
  /** Adds a request from a log format v2 stream; created=false when the project has it already. */
  importRequest(projectId: number, text: string): Promise<{ request_id: number; created: boolean }>;
  /** Pins a request or sets its note (an empty note removes it). */
  markRequest(id: number, patch: { pinned?: boolean; note?: string }): Promise<Request>;
  /** Lines of the file around the message, or around a frame of its backtrace. */
  getMessageSource(id: number, frame?: number): Promise<SourcePreview>;
  listServers(): Promise<{ items: Server[] }>;
  createServer(input: ServerInput): Promise<Server>;
  updateServer(id: number, patch: ServerPatch): Promise<Server>;
  deleteServer(id: number): Promise<void>;
  /** Connects to the server now; an unknown host key comes back as host_key_fingerprint. */
  checkServer(id: number): Promise<ServerCheck>;
  /** Trusts the host key once the user compared its fingerprint with the server. */
  trustHostKey(id: number, fingerprint: string): Promise<void>;
  /** Servers and projects as YAML, without secrets. */
  exportConfig(): Promise<string>;
  /** Adds what the file has and the viewer does not. */
  importConfig(text: string): Promise<ImportResult>;
  /** Finds projects on a server by the library config; nothing is added. */
  discoverProjects(
    id: number,
    roots: string[],
    maxDepth?: number,
  ): Promise<{ items: DiscoveredProject[]; truncated: boolean }>;
  listProjects(): Promise<{ items: Project[] }>;
  createProject(input: ProjectInput): Promise<Project>;
  updateProject(id: number, patch: ProjectPatch): Promise<Project>;
  deleteProject(id: number): Promise<void>;
  /** A new push token of the project, shown this once; the old one stops working. */
  createIngestToken(id: number): Promise<{ token: string }>;
  /** Stops pushed logs of the project. */
  deleteIngestToken(id: number): Promise<void>;
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

  // Sends a request and turns an answer other than 2xx into an ApiError
  async function exchange(method: string, url: string, init: RequestInit): Promise<Response> {
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
    return response;
  }

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
    const response = await exchange(method, url, init);
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
    getUpdates: () => getJSON<UpdateInfo>('/api/v1/updates'),
    getDesktopSettings: () => getJSON<DesktopSettings>('/api/v1/desktop/settings'),
    updateDesktopSettings: (settings) =>
      request<DesktopSettings>('PUT', '/api/v1/desktop/settings', {}, settings),
    listRequests: (params) => getJSON<RequestPage>('/api/v1/requests', params),
    getRequest: (id) => getJSON<RequestDetail>(`/api/v1/requests/${id}`),
    listMessages: (requestId, params) =>
      getJSON<MessagePage>(`/api/v1/requests/${requestId}/messages`, params),
    getMessage: (id) => getJSON<MessageDetail>(`/api/v1/messages/${id}`),
    importRequest: async (projectId, text) => {
      const response = await exchange(
        'POST',
        `${options.baseUrl}/api/v1/projects/${projectId}/import`,
        {
          method: 'POST',
          headers: { Accept: 'application/json', 'Content-Type': 'application/x-ndjson' },
          body: text,
        },
      );
      return (await response.json()) as { request_id: number; created: boolean };
    },
    markRequest: (id, patch) => request<Request>('PATCH', `/api/v1/requests/${id}`, {}, patch),
    getMessageSource: (id, frame) =>
      getJSON<SourcePreview>(`/api/v1/messages/${id}/source`, frame === undefined ? {} : { frame }),
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
    discoverProjects: (id, roots, maxDepth) =>
      request<{ items: DiscoveredProject[]; truncated: boolean }>(
        'POST',
        `/api/v1/servers/${id}/discover`,
        {},
        maxDepth === undefined ? { roots } : { roots, max_depth: maxDepth },
      ),
    exportConfig: async () => {
      const response = await exchange('GET', `${options.baseUrl}/api/v1/config`, {
        headers: { Accept: 'application/yaml' },
      });
      return response.text();
    },
    importConfig: async (text) => {
      const response = await exchange('POST', `${options.baseUrl}/api/v1/config`, {
        method: 'POST',
        headers: { Accept: 'application/json', 'Content-Type': 'application/yaml' },
        body: text,
      });
      return (await response.json()) as ImportResult;
    },
    listProjects: () => getJSON<{ items: Project[] }>('/api/v1/projects'),
    createProject: (input) => request<Project>('POST', '/api/v1/projects', {}, input),
    updateProject: (id, patch) => request<Project>('PATCH', `/api/v1/projects/${id}`, {}, patch),
    deleteProject: async (id) => {
      await request<unknown>('DELETE', `/api/v1/projects/${id}`);
    },
    createIngestToken: (id) =>
      request<{ token: string }>('POST', `/api/v1/projects/${id}/ingest-token`),
    deleteIngestToken: async (id) => {
      await request<unknown>('DELETE', `/api/v1/projects/${id}/ingest-token`);
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
