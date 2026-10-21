<script lang="ts">
  import { ApiError, type Transport } from '../lib/api/client';
  import type { Level, Message, RequestDetail } from '../lib/api/types';
  import { formatDuration, formatTime, requestTitle } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { loadAllMessages } from '../lib/messages';
  import RequestGlobals from './RequestGlobals.svelte';
  import RequestReplay from './RequestReplay.svelte';
  import RequestSql from './RequestSql.svelte';
  import RequestTimeline from './RequestTimeline.svelte';
  import Tabs from './Tabs.svelte';
  import VirtualList from './VirtualList.svelte';

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
  let loadGeneration = 0;

  let request = $state<RequestDetail | null>(null);
  let messages = $state<Message[]>([]);
  let activeLevels = $state<Level[]>([]);
  let status = $state<'loading' | 'ready' | 'not-found' | 'error'>('loading');
  let errorMessage = $state('');

  let editingNote = $state(false);

  // A pin or a note: the request comes back with them
  async function mark(patch: { pinned?: boolean; note?: string }) {
    if (!request) {
      return;
    }
    try {
      const marked = await transport.markRequest(request.id, patch);
      request = { ...request, pinned: marked.pinned, note: marked.note };
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : String(error);
    }
  }

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
    const generation = ++loadGeneration;
    const params = {
      ...(filter.length > 0 ? { level: filter } : {}),
      ...(search === '' ? {} : { q: search }),
    };
    try {
      await loadAllMessages(
        transport,
        id,
        params,
        (loaded) => (messages = loaded),
        () => generation === loadGeneration,
      );
    } catch {
      if (generation === loadGeneration) {
        messages = [];
      }
    }
  }

  // The other views need every message, whatever the filters of the list
  type View = 'messages' | 'timeline' | 'sql' | 'globals';
  let view = $state<View>('messages');
  let allMessages = $state<Message[]>([]);
  let allGeneration = 0;

  async function loadEverything(id: number) {
    const generation = ++allGeneration;
    try {
      await loadAllMessages(
        transport,
        id,
        {},
        (loaded) => (allMessages = loaded),
        () => generation === allGeneration,
      );
    } catch {
      if (generation === allGeneration) {
        allMessages = [];
      }
    }
  }

  // The SQL count shows on its tab, so the whole set loads with the request
  $effect(() => {
    allMessages = [];
    void loadEverything(requestId);
  });

  const sqlCount = $derived(allMessages.filter((message) => message.tags.includes('sql')).length);

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
        void loadEverything(requestId);
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
      <div class="marks">
        <button
          type="button"
          class="mark"
          aria-pressed={request.pinned}
          onclick={() => request && mark({ pinned: !request.pinned })}>{i18n.t('marks.pin')}</button
        >
        <button
          type="button"
          class="mark"
          aria-expanded={editingNote}
          onclick={() => (editingNote = !editingNote)}>{i18n.t('marks.note')}</button
        >
        <a class="mark" href={`/api/v1/requests/${request.id}/export`} download
          >{i18n.t('transfer.export')}</a
        >
        <RequestReplay {transport} {request} {messages} />
      </div>
      {#if editingNote}
        <textarea
          class="note-editor"
          aria-label={i18n.t('marks.note')}
          rows="2"
          placeholder={i18n.t('marks.noteHint')}
          value={request.note ?? ''}
          onblur={(event) => {
            const note = event.currentTarget.value.trim();
            if (note !== (request?.note ?? '')) {
              void mark({ note });
            }
          }}></textarea>
      {:else if request.note}
        <p class="note">{request.note}</p>
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

    <Tabs
      label={i18n.t('views.label')}
      idPrefix="request"
      tabs={[
        { id: 'messages', label: i18n.t('views.messages') },
        { id: 'timeline', label: i18n.t('views.timeline') },
        { id: 'sql', label: i18n.t('views.sql'), count: sqlCount },
        { id: 'globals', label: i18n.t('views.globals') },
      ]}
      value={view}
      onchange={(next: View) => (view = next)}
    />

    {#if view === 'timeline'}
      <div role="tabpanel" id="panel-request-timeline" aria-labelledby="tab-request-timeline">
        <RequestTimeline
          {request}
          messages={allMessages}
          selectedId={selectedMessageId}
          onselect={onselectmessage}
        />
      </div>
    {:else if view === 'sql'}
      <div role="tabpanel" id="panel-request-sql" aria-labelledby="tab-request-sql">
        <RequestSql
          messages={allMessages}
          selectedId={selectedMessageId}
          onselect={onselectmessage}
        />
      </div>
    {:else if view === 'globals'}
      <div role="tabpanel" id="panel-request-globals" aria-labelledby="tab-request-globals">
        <RequestGlobals {transport} messages={allMessages} />
      </div>
    {:else}
      <div role="tabpanel" id="panel-request-messages" aria-labelledby="tab-request-messages">
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
          <VirtualList
            items={messages}
            key={(message) => message.id}
            label={i18n.t('detail.messages')}
          >
            {#snippet row(message: Message, index: number)}
              <li
                class="message"
                data-message-id={message.id}
                aria-posinset={index + 1}
                aria-setsize={messages.length}
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
                  {#if message.duration_ms !== undefined && message.duration_ms !== null}
                    <span class="duration">{formatDuration(message.duration_ms, i18n.locale)}</span>
                  {/if}
                </button>
              </li>
            {/snippet}
          </VirtualList>
        {/if}
      </div>
    {/if}
  {/if}
</section>

<style>
  .marks {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }

  .mark {
    padding: 0 var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  a.mark {
    text-decoration: none;
  }

  .mark[aria-pressed='true'] {
    border-color: var(--accent);
    color: var(--text);
  }

  .mark:focus-visible,
  .note-editor:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .note,
  .note-editor {
    margin: var(--space-2) 0 0;
    font-size: var(--font-size-small);
  }

  .note {
    padding: var(--space-1) var(--space-2);
    border-left: 3px solid var(--accent);
    color: var(--text);
    white-space: pre-wrap;
  }

  .note-editor {
    box-sizing: border-box;
    width: 100%;
    padding: var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
    resize: vertical;
  }

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
    padding: var(--row-padding-y) var(--space-4);
    line-height: var(--row-line-height);
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
