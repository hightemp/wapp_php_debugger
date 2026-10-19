<script lang="ts">
  import { onMount } from 'svelte';
  import type { Transport } from '../lib/api/client';
  import type { Request } from '../lib/api/types';
  import { formatDuration, formatTime, requestTitle } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { applyStreamEvent } from '../lib/requests';

  let { transport }: { transport: Transport } = $props();

  const pageSize = 100;
  let items = $state<Request[]>([]);
  let nextCursor = $state<string | null>(null);
  let status = $state<'loading' | 'ready' | 'error'>('loading');
  let errorMessage = $state('');
  let loadingMore = $state(false);

  async function load() {
    status = 'loading';
    try {
      const page = await transport.listRequests({ limit: pageSize });
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
      const page = await transport.listRequests({ limit: pageSize, cursor: nextCursor });
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

  onMount(() => {
    void load();
    return transport.subscribe((event) => {
      const update = applyStreamEvent(items, event);
      items = update.items;
      if (update.reload) {
        void load();
      }
    });
  });

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

  {#if status === 'loading' && items.length === 0}
    <p class="state">{i18n.t('feed.loading')}</p>
  {:else if status === 'error'}
    <div class="state error" role="alert">
      <p>{i18n.t('feed.error', { message: errorMessage })}</p>
      <button type="button" class="action" onclick={load}>{i18n.t('feed.retry')}</button>
    </div>
  {:else if items.length === 0}
    <div class="state">
      <p class="state-title">{i18n.t('feed.emptyTitle')}</p>
      <p class="hint">{i18n.t('feed.emptyHint')}</p>
    </div>
  {:else}
    <ul class="list">
      {#each items as request (request.id)}
        <li class="row" data-kind={outcomeKind(request)}>
          <span class="outcome">{outcome(request)}</span>
          <span class="title" title={request.url ?? request.request_key}
            >{requestTitle(request)}</span
          >
          <span class="meta">
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
            <span class="count">{i18n.t('request.messages', { count: request.message_count })}</span
            >
            <span class="duration">{formatDuration(request.duration_ms, i18n.locale)}</span>
            <time datetime={new Date(request.started_at).toISOString()}
              >{formatTime(request.started_at)}</time
            >
          </span>
        </li>
      {/each}
    </ul>
    {#if nextCursor}
      <button type="button" class="action more" onclick={loadMore} disabled={loadingMore}>
        {i18n.t('feed.loadMore')}
      </button>
    {/if}
  {/if}
</section>

<style>
  .feed {
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .list {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .row {
    display: grid;
    grid-template-columns: 11ch 1fr auto;
    gap: var(--space-3);
    align-items: center;
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--border);
    font-variant-numeric: tabular-nums;
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
