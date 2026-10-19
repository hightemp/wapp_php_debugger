<script lang="ts">
  import { ApiError, type Transport } from '../lib/api/client';
  import type { Level, Message, RequestDetail } from '../lib/api/types';
  import { formatDuration, formatTime, requestTitle } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';

  let {
    transport,
    requestId,
    selectedMessageId,
    onselectmessage,
    onselectrequest,
  }: {
    transport: Transport;
    requestId: number;
    selectedMessageId?: number;
    onselectmessage: (id: number) => void;
    onselectrequest: (id: number) => void;
  } = $props();

  const levels: Level[] = ['error', 'warning', 'info', 'message', 'debug'];
  // Messages of one request rarely exceed this; paging inside a request comes later
  const messageLimit = 500;

  let request = $state<RequestDetail | null>(null);
  let messages = $state<Message[]>([]);
  let activeLevels = $state<Level[]>([]);
  let status = $state<'loading' | 'ready' | 'not-found' | 'error'>('loading');
  let errorMessage = $state('');

  async function loadRequest(id: number) {
    try {
      request = await transport.getRequest(id);
      status = 'ready';
    } catch (error) {
      request = null;
      status = error instanceof ApiError && error.status === 404 ? 'not-found' : 'error';
      errorMessage = error instanceof Error ? error.message : String(error);
    }
  }

  const searchDelay = 250;
  let searchInput = $state('');
  let query = $state('');
  let searchTimer: ReturnType<typeof setTimeout> | undefined;

  function onSearchInput(value: string) {
    searchInput = value;
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => (query = value.trim()), searchDelay);
  }

  async function loadMessages(id: number, filter: Level[], search: string) {
    try {
      const page = await transport.listMessages(id, {
        ...(filter.length > 0 ? { level: filter } : {}),
        ...(search === '' ? {} : { q: search }),
        limit: messageLimit,
      });
      messages = page.items;
    } catch {
      messages = [];
    }
  }

  $effect(() => {
    status = 'loading';
    void loadRequest(requestId);
  });

  $effect(() => {
    void loadMessages(requestId, activeLevels, query);
  });

  $effect(() =>
    transport.subscribe((event) => {
      if (event.event === 'request_updated' && event.request?.id === requestId) {
        void loadRequest(requestId);
        void loadMessages(requestId, activeLevels, query);
      }
    }),
  );

  function toggleLevel(level: Level) {
    activeLevels = activeLevels.includes(level)
      ? activeLevels.filter((item) => item !== level)
      : [...activeLevels, level];
  }

  function outcome(r: RequestDetail): string {
    if (!r.finished) {
      return i18n.t('request.notFinished');
    }
    if (r.sapi === 'cli') {
      return r.exit_code === null || r.exit_code === undefined
        ? i18n.t('request.cli')
        : i18n.t('request.exitCode', { code: r.exit_code });
    }
    return r.status_code ? String(r.status_code) : '—';
  }

  function location(message: Message): string {
    return message.source?.file ? `${message.source.file}:${message.source.line ?? ''}` : '';
  }
</script>

<section class="detail" aria-labelledby="request-title">
  {#if status === 'not-found'}
    <p class="state">{i18n.t('detail.notFound')}</p>
  {:else if status === 'error'}
    <p class="state error" role="alert">{errorMessage}</p>
  {:else if !request}
    <p class="state">{i18n.t('detail.loading')}</p>
  {:else}
    <header class="header">
      <h2 id="request-title" class="title">{requestTitle(request)}</h2>
      <p class="meta">
        <span>{outcome(request)}</span>
        <span>{formatDuration(request.duration_ms, i18n.locale)}</span>
        <span>{formatTime(request.started_at)}</span>
        {#if request.php_version}<span
            >{i18n.t('detail.php', { version: request.php_version })}</span
          >{/if}
        {#if request.hostname}<span>{request.hostname}</span>{/if}
        {#if request.app_user}<span>{request.app_user}</span>{/if}
      </p>
      {#if request.argv && request.argv.length > 0}
        <p class="argv">{request.argv.join(' ')}</p>
      {/if}
    </header>

    {#if request.children && request.children.length > 0}
      <div class="children">
        <h3 class="subtitle">{i18n.t('detail.children')}</h3>
        <ul class="plain">
          {#each request.children as child (child.id)}
            <li>
              <button type="button" class="link" onclick={() => onselectrequest(child.id)}>
                {requestTitle(child)}
              </button>
            </li>
          {/each}
        </ul>
      </div>
    {/if}

    <div class="filters" role="group" aria-label={i18n.t('detail.filterLabel')}>
      <input
        class="search"
        type="search"
        aria-label={i18n.t('detail.search')}
        placeholder={i18n.t('detail.search')}
        value={searchInput}
        oninput={(event) => onSearchInput(event.currentTarget.value)}
      />
      {#each levels as level (level)}
        <button
          type="button"
          class="chip"
          data-level={level}
          aria-pressed={activeLevels.includes(level)}
          onclick={() => toggleLevel(level)}
        >
          {i18n.t(`level.${level}`)}
        </button>
      {/each}
    </div>

    {#if messages.length === 0}
      <p class="state">{i18n.t('detail.noMessages')}</p>
    {:else}
      <ul class="messages">
        {#each messages as message (message.id)}
          <li
            class="message"
            data-level={message.level}
            aria-label={i18n.t('detail.messageLabel', { seq: message.seq })}
            aria-current={message.id === selectedMessageId ? 'true' : undefined}
          >
            <button
              type="button"
              class="message-button"
              onclick={() => onselectmessage(message.id)}
            >
              <span class="level">{i18n.t(`levelName.${message.level}`)}</span>
              <span class="text">{message.message}</span>
              <span class="where">{location(message)}</span>
              {#if message.type === 'timer'}
                <span class="duration">{formatDuration(message.duration_ms, i18n.locale)}</span>
              {/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</section>

<style>
  .detail {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .header {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
  }

  .title {
    margin: 0;
    overflow-wrap: anywhere;
    font-family: var(--font-mono);
    font-size: 14px;
    font-weight: 600;
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    margin: var(--space-2) 0 0;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .argv {
    margin: var(--space-2) 0 0;
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  .children {
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--border);
  }

  .subtitle {
    margin: 0 0 var(--space-2);
    color: var(--text-muted);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .plain {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .filters {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--border);
  }

  .search {
    flex: 1 1 16ch;
    padding: 2px var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
    font-size: var(--font-size-small);
  }

  .search:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }

  .chip {
    padding: 2px var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .chip[aria-pressed='true'] {
    border-color: var(--accent);
    color: var(--text);
  }

  .messages {
    margin: 0;
    padding: 0;
    overflow: auto;
    list-style: none;
  }

  .message {
    border-bottom: 1px solid var(--border);
  }

  .message[aria-current='true'] {
    background: var(--surface-hover);
  }

  .message-button {
    display: grid;
    grid-template-columns: 8ch 1fr auto auto;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-2) var(--space-4);
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .message-button:hover {
    background: var(--surface-hover);
  }

  .message-button:focus-visible,
  .chip:focus-visible,
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .level {
    color: var(--level-message);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  [data-level='error'] .level {
    color: var(--level-error);
  }

  [data-level='warning'] .level {
    color: var(--level-warning);
  }

  [data-level='info'] .level {
    color: var(--level-info);
  }

  .text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .where,
  .duration {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    white-space: nowrap;
  }

  .state {
    padding: var(--space-6) var(--space-4);
    color: var(--text-muted);
  }

  .error {
    color: var(--level-error);
  }
</style>
