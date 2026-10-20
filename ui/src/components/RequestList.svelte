<script lang="ts">
  import type { Transport } from '../lib/api/client';
  import type { Project, Request, Server } from '../lib/api/types';
  import { formatDuration, formatTime, requestTitle } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { applyStreamEvent } from '../lib/requests';
  import { onCommand } from '../lib/commands';
  import { parseFeedQuery } from '../lib/feedQuery';
  import FilterLine from './FilterLine.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Switch from './Switch.svelte';
  import VirtualList from './VirtualList.svelte';

  let {
    transport,
    projectId,
    serverId,
    selectedId,
    onselect = () => {},
  }: {
    transport: Transport;
    projectId?: number;
    /** The feed of one server; a project wins. */
    serverId?: number;
    selectedId?: number;
    onselect?: (id: number) => void;
  } = $props();

  const pageSize = 100;
  let items = $state<Request[]>([]);
  let nextCursor = $state<string | null>(null);
  let status = $state<'loading' | 'ready' | 'error'>('loading');
  let errorMessage = $state('');
  let loadingMore = $state(false);

  type Sapi = 'all' | 'web' | 'cli';
  type EnvironmentFilter = 'all' | Server['environment'];

  // Names for the combined feed: which project on which server a request comes from
  let servers = $state<Server[]>([]);
  let projects = $state<Project[]>([]);
  let environment = $state<EnvironmentFilter>('all');

  async function loadNames() {
    const [serverList, projectList] = await Promise.all([
      transport.listServers(),
      transport.listProjects(),
    ]);
    servers = serverList.items;
    projects = projectList.items;
  }

  $effect(() => {
    void loadNames().catch(() => {});
  });

  const combined = $derived(projectId === undefined);
  // Environments worth filtering by: only when there are servers besides this machine
  const environments = $derived.by((): EnvironmentFilter[] => {
    const present = [...new Set(servers.map((server) => server.environment))];
    return present.some((env) => env !== 'local') ? ['all', ...present] : [];
  });
  const scopeServer = $derived(projectId === undefined ? serverId : undefined);

  function origin(request: Request): string {
    const project = projects.find((item) => item.id === request.project_id);
    const server = servers.find((item) => item.id === request.server_id);
    if (!project) {
      return '';
    }
    return server && server.kind !== 'local' ? `${project.name} · ${server.name}` : project.name;
  }
  const searchDelay = 250;

  let searchInput = $state('');
  let query = $state('');
  let errorsOnly = $state(false);
  let sapi = $state<Sapi>('all');
  let searchTimer: ReturnType<typeof setTimeout> | undefined;

  function onSearchInput(value: string) {
    searchInput = value;
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => (query = value.trim()), searchDelay);
  }

  // The filter line: conditions go to the API as parameters, the other words to the text search
  const parsed = $derived(parseFeedQuery(query));
  const invalid = $derived(parsed.invalid);

  // Only set filters go to the API; no project means the combined feed of all projects
  const filter = $derived({
    ...(projectId === undefined ? {} : { project_id: projectId }),
    ...(scopeServer === undefined ? {} : { server_id: scopeServer }),
    ...(combined && scopeServer === undefined && environment !== 'all' ? { environment } : {}),
    ...parsed.params,
    ...(errorsOnly ? { has_errors: true } : {}),
    ...(sapi === 'all' ? {} : { sapi }),
  });
  const filtered = $derived(
    query !== '' || errorsOnly || sapi !== 'all' || (environment !== 'all' && combined),
  );

  function statusMatches(code: number | null | undefined, pattern: string): boolean {
    if (code === null || code === undefined) {
      return false;
    }
    return pattern.endsWith('xx') ? String(code)[0] === pattern[0] : String(code) === pattern;
  }

  // A live request is added when it surely matches; the text search and tags are known only to the
  // server, so with them the feed is read again instead. Requests that gain errors after they were
  // created show up on the next reload.
  function mayShow(request: Request): 'yes' | 'no' | 'ask' {
    const params = filter;
    const matches =
      (projectId === undefined || request.project_id === projectId) &&
      (scopeServer === undefined || request.server_id === scopeServer) &&
      (environment === 'all' ||
        !combined ||
        servers.find((server) => server.id === request.server_id)?.environment === environment) &&
      (!params.has_errors || request.error_count > 0) &&
      (params.sapi === undefined || request.sapi === params.sapi) &&
      (params.method === undefined || request.method?.toUpperCase() === params.method) &&
      (params.status === undefined || statusMatches(request.status_code, params.status)) &&
      (params.min_duration_ms === undefined ||
        (request.duration_ms ?? -1) >= params.min_duration_ms) &&
      (params.is_ajax === undefined || request.is_ajax === params.is_ajax) &&
      (params.pinned === undefined || request.pinned === params.pinned) &&
      (params.level !== 'error' || request.error_count > 0) &&
      (params.level !== 'warning' || request.error_count + request.warning_count > 0);
    if (!matches) {
      return 'no';
    }
    return params.q === undefined && params.tag === undefined ? 'yes' : 'ask';
  }

  // Paused, the feed holds still: what comes in is only counted until it is resumed
  let paused = $state(false);
  let waiting = $state(0);

  function togglePause() {
    paused = !paused;
    if (!paused && waiting > 0) {
      waiting = 0;
      void load();
    }
  }

  // The palette filters the feed at once
  $effect(() =>
    onCommand((command) => {
      if (command.type === 'filter') {
        clearTimeout(searchTimer);
        searchInput = command.text;
        query = command.text.trim();
      }
    }),
  );

  let reloadTimer: ReturnType<typeof setTimeout> | undefined;
  function reloadSoon() {
    clearTimeout(reloadTimer);
    reloadTimer = setTimeout(() => void load(), 1000);
  }

  async function load() {
    status = 'loading';
    try {
      const page = await transport.listRequests({ limit: pageSize, ...filter });
      items = page.items;
      nextCursor = page.next_cursor;
      status = 'ready';
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : String(error);
      status = 'error';
    }
  }

  async function loadMore() {
    if (!nextCursor || loadingMore) {
      return;
    }
    loadingMore = true;
    try {
      const page = await transport.listRequests({
        limit: pageSize,
        cursor: nextCursor,
        ...filter,
      });
      const known = new Set(items.map((item) => item.id));
      items = [...items, ...page.items.filter((item) => !known.has(item.id))];
      nextCursor = page.next_cursor;
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : String(error);
      status = 'error';
    } finally {
      loadingMore = false;
    }
  }

  $effect(() => {
    void filter; // reload when the project or a filter changes
    void load();
  });

  $effect(() =>
    transport.subscribe((event) => {
      if (paused && event.event === 'request_created') {
        if (event.request && mayShow(event.request) !== 'no') {
          waiting += 1;
        }
        return;
      }
      if (event.event === 'request_created' && event.request) {
        const verdict = mayShow(event.request);
        if (verdict === 'ask') {
          reloadSoon();
        }
        if (verdict !== 'yes') {
          return;
        }
      }
      if (
        event.event === 'request_created' &&
        event.request &&
        !projects.some((project) => project.id === event.request?.project_id)
      ) {
        void loadNames().catch(() => {}); // a project added since
      }
      const update = applyStreamEvent(items, event);
      items = update.items;
      if (update.reload) {
        void load();
      }
    }),
  );

  function outcome(request: Request): string {
    if (!request.finished) {
      return i18n.t('request.notFinished');
    }
    if (request.sapi === 'cli') {
      return request.exit_code === null || request.exit_code === undefined
        ? i18n.t('request.cli')
        : i18n.t('request.exitCode', { code: request.exit_code });
    }
    return request.status_code ? String(request.status_code) : '—';
  }

  function outcomeKind(request: Request): string {
    if (!request.finished || request.error_count > 0 || (request.status_code ?? 0) >= 500) {
      return 'bad';
    }
    if (request.warning_count > 0 || (request.status_code ?? 0) >= 400) {
      return 'warn';
    }
    return 'ok';
  }
</script>

<section class="feed" aria-labelledby="feed-title">
  <h2 id="feed-title" class="visually-hidden">{i18n.t('feed.title')}</h2>

  <div class="toolbar">
    <FilterLine
      value={searchInput}
      label={i18n.t('feed.search')}
      placeholder={i18n.t('feed.searchHint')}
      tags={['sql', 'http', 'globals', 'cache']}
      oninput={onSearchInput}
    />
    {#if invalid.length > 0}
      <p class="invalid" role="status">{i18n.t('feed.invalid', { words: invalid.join(' ') })}</p>
    {/if}
    <div class="filters">
      <SegmentedControl
        label={i18n.t('feed.sapi')}
        options={[
          { value: 'all', label: i18n.t('feed.sapiAll') },
          { value: 'web', label: i18n.t('feed.sapiWeb') },
          { value: 'cli', label: i18n.t('feed.sapiCli') },
        ]}
        value={sapi}
        onchange={(value: Sapi) => (sapi = value)}
      />
      <span class="errors-only">
        <Switch
          label={i18n.t('feed.errorsOnly')}
          checked={errorsOnly}
          onchange={(value) => (errorsOnly = value)}
        />
        <span aria-hidden="true">{i18n.t('feed.errorsOnly')}</span>
      </span>
      {#if combined && scopeServer === undefined && environments.length > 0}
        <SegmentedControl
          label={i18n.t('feed.environment')}
          options={environments.map((value) => ({
            value,
            label: value === 'all' ? i18n.t('feed.environmentAll') : value,
          }))}
          value={environment}
          onchange={(value: EnvironmentFilter) => (environment = value)}
        />
      {/if}
      <button type="button" class="pause" aria-pressed={paused} onclick={togglePause}
        >{i18n.t('feed.pause')}</button
      >
    </div>
    {#if paused && waiting > 0}
      <button type="button" class="waiting" onclick={togglePause}
        >{i18n.t('feed.waiting', { count: waiting })}</button
      >
    {/if}
  </div>

  {#if status === 'loading' && items.length === 0}
    <p class="state">{i18n.t('feed.loading')}</p>
  {:else if status === 'error'}
    <div class="state error" role="alert">
      <p>{i18n.t('feed.error', { message: errorMessage })}</p>
      <button type="button" class="action" onclick={load}>{i18n.t('feed.retry')}</button>
    </div>
  {:else if items.length === 0 && filtered}
    <p class="state">{i18n.t('feed.noMatches')}</p>
  {:else if items.length === 0}
    <div class="state">
      <p class="state-title">{i18n.t('feed.emptyTitle')}</p>
      <p class="hint">{i18n.t('feed.emptyHint')}</p>
    </div>
  {:else}
    <VirtualList {items} key={(request) => request.id} label={i18n.t('feed.title')}>
      {#snippet row(request: Request, index: number)}
        <li
          class="item"
          data-request-id={request.id}
          aria-posinset={index + 1}
          aria-setsize={items.length}
          aria-current={request.id === selectedId ? 'true' : undefined}
        >
          <button
            type="button"
            class="row"
            data-kind={outcomeKind(request)}
            onclick={() => onselect(request.id)}
          >
            <span class="outcome">{outcome(request)}</span>
            <span class="title" title={request.url ?? request.request_key}
              >{requestTitle(request)}</span
            >
            <span class="meta">
              {#if request.pinned}
                <span class="badge pinned">{i18n.t('marks.pinned')}</span>
              {/if}
              {#if request.note}
                <span class="row-note" title={request.note}>{request.note}</span>
              {/if}
              {#if combined && origin(request)}
                <span class="origin">{origin(request)}</span>
              {/if}
              {#if request.error_count > 0}
                <span class="badge error"
                  >{i18n.t('request.errors', { count: request.error_count })}</span
                >
              {/if}
              {#if request.warning_count > 0}
                <span class="badge warning"
                  >{i18n.t('request.warnings', { count: request.warning_count })}</span
                >
              {/if}
              <span class="count"
                >{i18n.t('request.messages', { count: request.message_count })}</span
              >
              <span class="duration">{formatDuration(request.duration_ms, i18n.locale)}</span>
              <time datetime={new Date(request.started_at).toISOString()}
                >{formatTime(request.started_at)}</time
              >
            </span>
          </button>
        </li>
      {/snippet}
    </VirtualList>
    {#if nextCursor}
      <button type="button" class="action more" onclick={loadMore} disabled={loadingMore}>
        {i18n.t('feed.loadMore')}
      </button>
    {/if}
  {/if}
</section>

<style>
  .pause {
    margin-left: auto;
    padding: 2px var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .pause[aria-pressed='true'] {
    border-color: var(--level-warning);
    color: var(--level-warning);
  }

  .waiting {
    width: 100%;
    margin-top: var(--space-2);
    padding: var(--space-1);
    border: 1px dashed var(--accent);
    border-radius: var(--radius);
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .pause:focus-visible,
  .waiting:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .badge.pinned {
    background: var(--surface-sunken);
    color: var(--accent);
  }

  .row-note {
    max-width: 16ch;
    overflow: hidden;
    color: var(--text);
    font-style: italic;
    text-overflow: ellipsis;
  }

  .invalid {
    margin: var(--space-1) 0 0;
    color: var(--level-warning);
    font-size: var(--font-size-small);
  }

  .origin {
    color: var(--text-muted);
    font-family: var(--font-mono);
  }

  .feed {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .toolbar {
    display: grid;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--border);
  }

  .filters {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    align-items: center;
  }

  .errors-only {
    display: inline-flex;
    gap: var(--space-2);
    align-items: center;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .item[aria-current='true'] .row {
    background: var(--surface-hover);
    box-shadow: inset 3px 0 0 var(--accent);
  }

  .row {
    width: 100%;
    border: 0;
    border-bottom: 1px solid var(--border);
    background: none;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
    display: grid;
    grid-template-columns: 11ch 1fr;
    gap: var(--space-1) var(--space-3);
    align-items: center;
    padding: var(--space-2) var(--space-4);
    font-variant-numeric: tabular-nums;
  }

  .row:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .row:hover {
    background: var(--surface-hover);
  }

  .outcome {
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    color: var(--level-info);
  }

  .row[data-kind='warn'] .outcome {
    color: var(--level-warning);
  }

  .row[data-kind='bad'] .outcome {
    color: var(--level-error);
  }

  .title {
    overflow: hidden;
    font-family: var(--font-mono);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    grid-column: 2;
    overflow: hidden;
    display: flex;
    gap: var(--space-3);
    align-items: center;
    color: var(--text-muted);
    font-size: var(--font-size-small);
    white-space: nowrap;
  }

  .badge {
    padding: 0 var(--space-2);
    border-radius: var(--radius);
  }

  .badge.error {
    background: var(--level-error-soft);
    color: var(--level-error);
  }

  .badge.warning {
    background: var(--level-warning-soft);
    color: var(--level-warning);
  }

  .state {
    padding: var(--space-6) var(--space-4);
    color: var(--text-muted);
  }

  .state-title {
    margin: 0 0 var(--space-2);
    color: var(--text);
    font-weight: 600;
  }

  .hint {
    margin: 0;
  }

  .error {
    color: var(--level-error);
  }

  .action {
    padding: var(--space-1) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    cursor: pointer;
  }

  .action:hover {
    background: var(--surface-hover);
  }

  .action:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .more {
    align-self: center;
    margin: var(--space-4);
  }
</style>
