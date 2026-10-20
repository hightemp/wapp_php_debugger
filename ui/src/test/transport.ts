// Test helpers: an in-memory Transport and sample DTOs.
import { vi } from 'vitest';
import type { Transport, UpdateInfo } from '../lib/api/client';
import type {
  DesktopSettings,
  DiscoveredProject,
  MessageDetail,
  Project,
  Request,
  RequestDetail,
  Server,
  StreamEvent,
} from '../lib/api/types';

export function sampleRequest(id: number, overrides: Partial<Request> = {}): Request {
  return {
    id,
    project_id: 1,
    server_id: 1,
    request_key: `k${id}`,
    format_version: 2,
    sapi: 'web',
    method: 'GET',
    url: `https://shop.test/item/${id}`,
    is_ajax: false,
    status_code: 200,
    started_at: Date.UTC(2026, 9, 19, 12, 0, id),
    duration_ms: 40,
    finished: true,
    message_count: 5,
    error_count: 0,
    warning_count: 0,
    received_at: 0,
    pinned: false,
    ...overrides,
  };
}

export function sampleRequestDetail(
  id: number,
  overrides: Partial<RequestDetail> = {},
): RequestDetail {
  return { ...sampleRequest(id), lang: 'php', php_version: '8.3.12', children: [], ...overrides };
}

export function sampleMessage(id: number, overrides: Partial<MessageDetail> = {}): MessageDetail {
  return {
    id,
    request_id: 1,
    seq: id,
    ts: Date.UTC(2026, 9, 19, 12, 0, 0, id),
    type: 'message',
    level: 'info',
    message: `message ${id}`,
    tags: [],
    has_context: false,
    size_bytes: 10,
    ...overrides,
  };
}

export function sampleProject(id: number, overrides: Partial<Project> = {}): Project {
  return {
    id,
    server_id: 1,
    name: `project-${id}`,
    root_path: `/var/www/project-${id}`,
    logs_dir: `/var/www/project-${id}/tmp/logs`,
    local_path: null,
    config_path: `/var/www/project-${id}/dws_debugger.json`,
    editor: 'vscode',
    retention: {},
    pending_files: 0,
    created_at: 0,
    updated_at: 0,
    ...overrides,
  };
}

export function sampleServer(id: number, overrides: Partial<Server> = {}): Server {
  return {
    id,
    name: id === 1 ? 'local' : `server-${id}`,
    kind: id === 1 ? 'local' : 'ssh',
    environment: id === 1 ? 'local' : 'stage',
    config: id === 1 ? {} : { host: `stage-${id}`, user: 'deploy' },
    permissions: { read_only: false, allow_delete: true, allow_control: true },
    connection: { state: id === 1 ? 'online' : 'unknown' },
    created_at: 0,
    updated_at: 0,
    ...overrides,
  };
}

/** A Transport whose methods are vi.fn() with empty answers; override what a test needs. */
export function fakeTransport(overrides: Partial<Transport> = {}) {
  let emit: (event: StreamEvent) => void = () => {};
  const unsubscribe = vi.fn();
  const transport = {
    getVersion: vi.fn(async () => ({
      app: 'server' as 'server' | 'desktop',
      version: 'v0.0.0-test',
      commit: 'abc',
      date: '',
    })),
    // The update check is off by default
    markRequest: vi.fn(async (id: number) => sampleRequest(id)),
    getMessageSource: vi.fn(async () => ({
      file: 'x.php',
      line: 1,
      first_line: 1,
      lines: [] as string[],
    })),
    getUpdates: vi.fn(async (): Promise<UpdateInfo> => ({
      enabled: false,
      current: 'v0.0.0-test',
      newer: false,
    })),
    getDesktopSettings: vi.fn(async () => ({
      notifications: { enabled: true, min_level: 'error' as const, project_ids: [] as number[] },
      autostart: false,
      close_to_tray: true,
      update_check: true,
    })),
    updateDesktopSettings: vi.fn(async (settings: DesktopSettings) => settings),
    listRequests: vi.fn(async () => ({ items: [], next_cursor: null })),
    getRequest: vi.fn(async (id: number) => sampleRequestDetail(id)),
    listMessages: vi.fn(async () => ({ items: [], next_cursor: null })),
    getMessage: vi.fn(async (id: number) => sampleMessage(id)),
    listServers: vi.fn(async () => ({ items: [] as Server[] })),
    createServer: vi.fn(async () => sampleServer(2)),
    updateServer: vi.fn(async (id: number) => sampleServer(id)),
    deleteServer: vi.fn(async () => undefined),
    checkServer: vi.fn(async () => ({
      ok: true,
      steps: [] as { name: string; ok: boolean; detail?: string }[],
    })),
    trustHostKey: vi.fn(async () => undefined),
    exportConfig: vi.fn(async () => 'version: 1\nservers: []\n'),
    importConfig: vi.fn(async () => ({
      servers_created: [] as string[],
      projects_created: [] as string[],
      skipped: [] as string[],
      needs_token: [] as string[],
    })),
    discoverProjects: vi.fn(async () => ({ items: [] as DiscoveredProject[], truncated: false })),
    listProjects: vi.fn(async () => ({ items: [] as Project[] })),
    createProject: vi.fn(async () => sampleProject(1)),
    updateProject: vi.fn(async (id: number) => sampleProject(id)),
    deleteProject: vi.fn(async () => undefined),
    getDebuggerState: vi.fn(async () => ({ enabled: false })),
    setDebuggerState: vi.fn(async (_id: number, enabled: boolean) => ({ enabled })),
    clearProjectLogs: vi.fn(async () => ({ removed_files: 0 })),
    subscribe: vi.fn((onEvent: (event: StreamEvent) => void) => {
      emit = onEvent;
      return unsubscribe;
    }),
    ...overrides,
  } satisfies Transport;
  return { transport, unsubscribe, emit: (event: StreamEvent) => emit(event) };
}
