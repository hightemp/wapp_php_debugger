<script lang="ts">
  // The request on a time scale: where it spent its time (timers, SQL, HTTP) and when things happened.
  import type { Message } from '../lib/api/types';
  import { formatDuration } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { timeline } from '../lib/insight';

  let {
    request,
    messages,
    selectedId,
    onselect,
  }: {
    request: { started_at: number; duration_ms?: number | null };
    messages: readonly Message[];
    selectedId?: number;
    onselect: (id: number) => void;
  } = $props();

  const view = $derived(timeline(request, messages));
</script>

{#if view.rows.length === 0}
  <p class="state">{i18n.t('detail.noMessages')}</p>
{:else}
  <p class="scale">
    <span>0</span><span>{formatDuration(view.totalMs, i18n.locale)}</span>
  </p>
  <ol class="rows">
    {#each view.rows as row (row.id)}
      <li
        data-kind={row.kind}
        data-level={row.level}
        aria-current={row.id === selectedId ? 'true' : undefined}
      >
        <button type="button" onclick={() => onselect(row.id)}>
          <span class="label">{row.label}</span>
          <span class="track" aria-hidden="true">
            <span
              class="bar"
              class:point={row.durationMs === 0}
              style:left={`${row.startPercent}%`}
              style:width={row.durationMs === 0 ? undefined : `max(2px, ${row.widthPercent}%)`}
            ></span>
          </span>
          <span class="time"
            >{row.durationMs > 0
              ? formatDuration(row.durationMs, i18n.locale)
              : `+${formatDuration(row.startMs, i18n.locale)}`}</span
          >
        </button>
      </li>
    {/each}
  </ol>
{/if}

<style>
  .state {
    margin: 0;
    padding: var(--space-3) var(--space-4);
    color: var(--text-muted);
  }

  .scale {
    display: flex;
    justify-content: space-between;
    margin: 0;
    padding: var(--space-2) var(--space-4) 0 calc(var(--space-4) + 40%);
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  .rows {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li[aria-current='true'] {
    background: var(--surface-hover);
  }

  button {
    display: grid;
    grid-template-columns: 40% 1fr 9ch;
    gap: var(--space-3);
    align-items: center;
    width: 100%;
    padding: 3px var(--space-4);
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: var(--font-size-small);
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

  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .track {
    position: relative;
    height: 10px;
    border-radius: 2px;
    background: var(--surface-sunken);
  }

  .bar {
    position: absolute;
    top: 0;
    bottom: 0;
    border-radius: 2px;
    background: var(--level-message);
  }

  .bar.point {
    width: 2px;
    background: var(--text-muted);
  }

  [data-kind='sql'] .bar {
    background: var(--syntax-keyword);
  }

  [data-kind='http'] .bar {
    background: var(--syntax-number);
  }

  [data-kind='timer'] .bar {
    background: var(--level-info);
  }

  [data-level='error'] .bar {
    background: var(--level-error);
  }

  [data-level='warning'] .bar {
    background: var(--level-warning);
  }

  .time {
    color: var(--text-muted);
    font-family: var(--font-mono);
    text-align: right;
    white-space: nowrap;
  }
</style>
