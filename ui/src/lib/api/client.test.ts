import { describe, expect, it, vi } from 'vitest';
import { ApiError, createHttpTransport, type EventSourceLike } from './client';

function jsonResponse(body: unknown, status = 200, type = 'application/json'): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': type } });
}

describe('createHttpTransport', () => {
  it('requests the feed with query parameters and parses the page', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ items: [], next_cursor: null }));
    const transport = createHttpTransport({ baseUrl: 'http://127.0.0.1:8820', fetch: fetchMock });

    const page = await transport.listRequests({ limit: 50, cursor: 'abc' });

    expect(page).toEqual({ items: [], next_cursor: null });
    expect(fetchMock).toHaveBeenCalledWith(
      'http://127.0.0.1:8820/api/v1/requests?limit=50&cursor=abc',
      expect.objectContaining({ headers: { Accept: 'application/json' } }),
    );
  });

  it('skips undefined parameters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ items: [], next_cursor: null }));
    await createHttpTransport({ baseUrl: '', fetch: fetchMock }).listRequests({});
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/requests');
  });

  it('turns Problem Details into ApiError', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        jsonResponse(
          { title: 'Bad Request', status: 400, detail: 'invalid cursor' },
          400,
          'application/problem+json',
        ),
      );
    const error = await createHttpTransport({ baseUrl: '', fetch: fetchMock })
      .listRequests({ cursor: 'x' })
      .catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ status: 400, title: 'Bad Request', detail: 'invalid cursor' });
  });

  it('reports network failures as ApiError with status 0', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));
    const error = await createHttpTransport({ baseUrl: '', fetch: fetchMock })
      .getVersion()
      .catch((e: unknown) => e);
    expect(error).toMatchObject({ status: 0, title: 'Network error' });
  });

  it('gets a request, its messages with filters and a message', async () => {
    const fetchMock = vi.fn().mockImplementation(async () => jsonResponse({}));
    const transport = createHttpTransport({ baseUrl: '', fetch: fetchMock });

    await transport.getRequest(42);
    await transport.listMessages(42, { level: ['error', 'warning'], cursor: 'c', limit: 50 });
    await transport.getMessage(1001);

    expect(fetchMock.mock.calls.map((call) => call[0])).toEqual([
      '/api/v1/requests/42',
      '/api/v1/requests/42/messages?level=error%2Cwarning&cursor=c&limit=50',
      '/api/v1/messages/1001',
    ]);
  });

  it('manages projects and the debugger with JSON bodies', async () => {
    const fetchMock = vi.fn().mockImplementation(async () => jsonResponse({}));
    const transport = createHttpTransport({ baseUrl: '', fetch: fetchMock });

    await transport.listServers();
    await transport.listProjects();
    await transport.createProject({
      server_id: 1,
      name: 'shop',
      root_path: '/r',
      logs_dir: '/r/logs',
    });
    await transport.updateProject(3, { local_path: '/home/dev/shop' });
    await transport.getDebuggerState(3);
    await transport.setDebuggerState(3, true);
    await transport.clearProjectLogs(3);

    const calls = fetchMock.mock.calls.map(([url, init]) => [
      url,
      init?.method ?? 'GET',
      init?.body ?? null,
    ]);
    expect(calls).toEqual([
      ['/api/v1/servers', 'GET', null],
      ['/api/v1/projects', 'GET', null],
      [
        '/api/v1/projects',
        'POST',
        '{"server_id":1,"name":"shop","root_path":"/r","logs_dir":"/r/logs"}',
      ],
      ['/api/v1/projects/3', 'PATCH', '{"local_path":"/home/dev/shop"}'],
      ['/api/v1/projects/3/debugger', 'GET', null],
      ['/api/v1/projects/3/debugger', 'PUT', '{"enabled":true}'],
      ['/api/v1/projects/3/logs', 'DELETE', null],
    ]);
  });

  it('manages servers and their host keys', async () => {
    const fetchMock = vi.fn().mockImplementation(async () => jsonResponse({}));
    const transport = createHttpTransport({ baseUrl: '', fetch: fetchMock });

    await transport.createServer({ name: 'stage', kind: 'ssh', config: { host: 'stage-web' } });
    await transport.updateServer(2, { config: { host: 'stage-web-2' } });
    await transport.checkServer(2);
    await transport.trustHostKey(2, 'SHA256:abc');
    await transport.discoverProjects(2, ['/var/www'], 3);
    await transport.deleteServer(2);

    const calls = fetchMock.mock.calls.map(([url, init]) => [
      url,
      init?.method,
      init?.body ?? null,
    ]);
    expect(calls).toEqual([
      ['/api/v1/servers', 'POST', '{"name":"stage","kind":"ssh","config":{"host":"stage-web"}}'],
      ['/api/v1/servers/2', 'PATCH', '{"config":{"host":"stage-web-2"}}'],
      ['/api/v1/servers/2/check', 'POST', null],
      ['/api/v1/servers/2/host-key', 'POST', '{"fingerprint":"SHA256:abc"}'],
      ['/api/v1/servers/2/discover', 'POST', '{"roots":["/var/www"],"max_depth":3}'],
      ['/api/v1/servers/2', 'DELETE', null],
    ]);
  });

  it('imports a request with the name of its file and manages push tokens', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ request_id: 9, created: true }))
      .mockResolvedValueOnce(jsonResponse({ token: 'tok' }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const transport = createHttpTransport({ baseUrl: '', fetch: fetchMock });
    await expect(transport.importRequest(3, 'line\n', 'laravel 1.log')).resolves.toEqual({
      request_id: 9,
      created: true,
    });
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/projects/3/import?name=laravel%201.log');
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: 'POST',
      body: 'line\n',
      headers: { 'Content-Type': 'application/x-ndjson' },
    });
    await expect(transport.createIngestToken(3)).resolves.toEqual({ token: 'tok' });
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/v1/projects/3/ingest-token');
    await transport.deleteIngestToken(3);
    expect(fetchMock.mock.calls[2]?.[1]).toMatchObject({ method: 'DELETE' });
  });

  it('exports and imports the config as YAML text', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response('version: 1\nservers: []\n', {
          status: 200,
          headers: { 'Content-Type': 'application/yaml' },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          servers_created: ['stage'],
          projects_created: [],
          skipped: [],
          needs_token: [],
        }),
      );
    const transport = createHttpTransport({ baseUrl: '', fetch: fetchMock });

    expect(await transport.exportConfig()).toBe('version: 1\nservers: []\n');
    const result = await transport.importConfig('version: 1\n');
    expect(result.servers_created).toEqual(['stage']);
    const [, importInit] = fetchMock.mock.calls[1] ?? [];
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/v1/config');
    expect(importInit?.method).toBe('POST');
    expect(importInit?.body).toBe('version: 1\n');
    expect(new Headers(importInit?.headers).get('Content-Type')).toBe('application/yaml');
  });

  it('gets and saves the UI settings', async () => {
    const settings = {
      theme: 'dark',
      accent_color: null,
      density: 'compact' as const,
      ui_font_size: 14,
      mono_font: null,
      env_frame: true,
      locale: null,
      custom_themes: [],
    };
    const fetchMock = vi.fn().mockImplementation(async () => jsonResponse(settings));
    const transport = createHttpTransport({ baseUrl: '', fetch: fetchMock });

    await expect(transport.getSettings()).resolves.toEqual(settings);
    await transport.updateSettings(settings);
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/v1/settings');
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({
      method: 'PUT',
      body: JSON.stringify(settings),
    });
  });

  it('accepts 204 No Content', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    await expect(
      createHttpTransport({ baseUrl: '', fetch: fetchMock }).deleteProject(3),
    ).resolves.toBeUndefined();
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe('DELETE');
  });

  it('subscribes to the stream and parses named events', () => {
    const listeners = new Map<string, (event: MessageEvent) => void>();
    const source: EventSourceLike = {
      addEventListener: (name, listener) => listeners.set(name, listener),
      close: vi.fn(),
    };
    const createEventSource = vi.fn().mockReturnValue(source);
    const transport = createHttpTransport({ baseUrl: '', fetch: vi.fn(), createEventSource });

    const received: unknown[] = [];
    const unsubscribe = transport.subscribe((event) => received.push(event));
    listeners.get('request_created')?.(
      new MessageEvent('request_created', { data: '{"event":"request_created","request_id":7}' }),
    );
    listeners.get('settings_updated')?.(
      new MessageEvent('settings_updated', { data: '{"event":"settings_updated"}' }),
    );
    listeners.get('resync')?.(new MessageEvent('resync', { data: '{"event":"resync"}' }));
    unsubscribe();

    expect(createEventSource).toHaveBeenCalledWith('/api/v1/stream');
    expect(received).toEqual([
      { event: 'request_created', request_id: 7 },
      { event: 'settings_updated' },
      { event: 'resync' },
    ]);
    expect(source.close).toHaveBeenCalled();
  });

  it('tells subscribers to reload once the stream first opens', () => {
    // What happened between the first load and the connection has no Last-Event-ID to come back by
    const listeners = new Map<string, (event: MessageEvent) => void>();
    const source: EventSourceLike = {
      addEventListener: (name, listener) => listeners.set(name, listener),
      close: vi.fn(),
    };
    const transport = createHttpTransport({
      baseUrl: '',
      fetch: vi.fn(),
      createEventSource: () => source,
    });
    const received: unknown[] = [];
    transport.subscribe((event) => received.push(event));
    listeners.get('open')?.(new MessageEvent('open'));
    // A reconnection sends Last-Event-ID and the server sends what was missed
    listeners.get('open')?.(new MessageEvent('open'));
    expect(received).toEqual([{ event: 'resync' }]);
  });

  it('shares one stream between subscribers and closes it after the last one leaves', () => {
    const listeners = new Map<string, (event: MessageEvent) => void>();
    const source: EventSourceLike = {
      addEventListener: (name, listener) => listeners.set(name, listener),
      close: vi.fn(),
    };
    const createEventSource = vi.fn().mockReturnValue(source);
    const transport = createHttpTransport({ baseUrl: '', fetch: vi.fn(), createEventSource });

    const first: unknown[] = [];
    const second: unknown[] = [];
    const leaveFirst = transport.subscribe((event) => first.push(event));
    const leaveSecond = transport.subscribe((event) => second.push(event));
    listeners.get('resync')?.(new MessageEvent('resync', { data: '{"event":"resync"}' }));

    expect(createEventSource).toHaveBeenCalledTimes(1);
    expect(first).toHaveLength(1);
    expect(second).toHaveLength(1);

    leaveFirst();
    expect(source.close).not.toHaveBeenCalled();
    leaveSecond();
    expect(source.close).toHaveBeenCalledTimes(1);
  });
});
