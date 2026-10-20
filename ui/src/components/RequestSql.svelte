<script lang="ts">
  // SQL of a request: queries in order with their time; the ones run again and again stand out.
  import type { Message } from '../lib/api/types';
  import { formatDuration } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { sqlSummary } from '../lib/insight';

  let {
    messages,
    selectedId,
    onselect,
  }: { messages: readonly Message[]; selectedId?: number; onselect: (id: number) => void } =
    $props();

  const summary = $derived(sqlSummary(messages));
</script>

{#if summary.count === 0}
  <p class="state">{i18n.t('sql.none')}</p>
{:else}
  <p class="summary">
    {i18n.t('sql.summary', {
      count: summary.count,
      time: formatDuration(summary.totalMs, i18n.locale),
    })}
  </p>
  {#each summary.repeated as group (group.sql)}
    <p class="repeated" role="note">
      {i18n.t('sql.repeated', {
        count: group.count,
        time: formatDuration(group.totalMs, i18n.locale),
      })}
      <code>{group.sql}</code>
    </p>
  {/each}
  <ol class="queries">
    {#each summary.queries as query (query.id)}
      <li aria-current={query.id === selectedId ? 'true' : undefined}>
        <button type="button" onclick={() => onselect(query.id)} data-repeated={query.repeats > 1}>
          <code class="sql">{query.sql}</code>
          {#if query.repeats > 1}<span class="repeats">×{query.repeats}</span>{/if}
          <span class="time" data-slowest={query.id === summary.slowest?.id}
            >{formatDuration(query.durationMs, i18n.locale)}</span
          >
        </button>
      </li>
    {/each}
  </ol>
{/if}

<style>
  .state,
  .summary {
    margin: 0;
    padding: var(--space-3) var(--space-4);
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .repeated {
    margin: 0 var(--space-4) var(--space-2);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius);
    background: var(--level-warning-soft);
    color: var(--level-warning);
    font-size: var(--font-size-small);
  }

  .repeated code {
    display: block;
    margin-top: var(--space-1);
    color: var(--text);
    overflow-wrap: anywhere;
  }

  .queries {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li {
    border-bottom: 1px solid var(--border);
  }

  li[aria-current='true'] {
    background: var(--surface-hover);
  }

  button {
    display: grid;
    grid-template-columns: 1fr auto auto;
    gap: var(--space-3);
    align-items: baseline;
    width: 100%;
    padding: var(--space-2) var(--space-4);
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  button:hover {
    background: var(--surface-hover);
  }

  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .sql {
    overflow: hidden;
    font-size: var(--font-size-small);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .repeats {
    color: var(--level-warning);
    font-size: var(--font-size-small);
  }

  .time {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    white-space: nowrap;
  }

  .time[data-slowest='true'] {
    color: var(--level-error);
  }
</style>
