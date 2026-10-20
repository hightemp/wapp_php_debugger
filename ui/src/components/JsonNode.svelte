<script lang="ts">
  import { i18n } from '../lib/i18n/locale.svelte';
  import { copyText, phpPath } from '../lib/jsonPath';
  import { toasts } from '../lib/toasts.svelte';
  import JsonChildren from './JsonChildren.svelte';
  import JsonValue from './JsonValue.svelte';

  let {
    name,
    value,
    depth,
    expandDepth,
    path,
    indexes,
    filter,
    query,
  }: {
    name: string;
    value: unknown;
    depth: number;
    expandDepth: number;
    path: string[];
    indexes: boolean[];
    filter: Set<string> | null;
    query: string;
  } = $props();

  // The part of the key that matches the search, to mark it
  const match = $derived.by(() => {
    const at = query === '' ? -1 : name.toLowerCase().indexOf(query.toLowerCase());
    return at < 0
      ? null
      : {
          before: name.slice(0, at),
          hit: name.slice(at, at + query.length),
          after: name.slice(at + query.length),
        };
  });

  function copy(text: string) {
    navigator.clipboard
      ?.writeText(text)
      .then(() =>
        toasts.push({
          text: i18n.t('toasts.copied', {
            text: text.length > 60 ? `${text.slice(0, 60)}…` : text,
          }),
        }),
      )
      .catch(() => {});
  }

  const isContainer = $derived(value !== null && typeof value === 'object');
  const entries = $derived(
    isContainer ? Object.entries(value as Record<string, unknown>) : ([] as [string, unknown][]),
  );
  // svelte-ignore state_referenced_locally
  let expanded = $state(depth < expandDepth);
  // A search opens the way to what it found
  const open = $derived(filter !== null || expanded);

  const summary = $derived(
    Array.isArray(value)
      ? i18n.t('json.items', { count: entries.length })
      : i18n.t('json.keys', { count: entries.length }),
  );
</script>

{#snippet key()}
  <span class="key"
    >{#if match}{match.before}<mark>{match.hit}</mark>{match.after}{:else}{name}{/if}</span
  >
{/snippet}

{#snippet actions()}
  <span class="actions">
    <button
      type="button"
      class="copy"
      aria-label={i18n.t('json.copyPath', { name })}
      title={phpPath(path, indexes)}
      onclick={() => copy(phpPath(path, indexes))}>{i18n.t('json.path')}</button
    >
    <button
      type="button"
      class="copy"
      aria-label={i18n.t('json.copyValue', { name })}
      onclick={() => copy(copyText(value))}>{i18n.t('json.value')}</button
    >
  </span>
{/snippet}

<li class="node">
  {#if isContainer && entries.length > 0}
    <span class="row">
      <button
        class="toggle"
        type="button"
        aria-expanded={open}
        onclick={() => (expanded = !expanded)}
      >
        <span class="arrow" aria-hidden="true">{open ? '▾' : '▸'}</span>{@render key()}
      </button>
      {#if !open}
        <span class="summary">{summary}</span>
      {/if}
      {@render actions()}
    </span>
    {#if open}
      <JsonChildren
        class="children"
        {entries}
        depth={depth + 1}
        {expandDepth}
        {path}
        {indexes}
        array={Array.isArray(value)}
        {filter}
        {query}
      />
    {/if}
  {:else}
    <span class="row leaf"
      >{@render key()}<span class="colon">:</span> <JsonValue {value} />{@render actions()}</span
    >
  {/if}
</li>

<style>
  .node {
    list-style: none;
  }

  .node :global(.children) {
    margin: 0;
    padding-left: var(--space-4);
    border-left: 1px solid var(--border);
  }

  .toggle {
    display: inline-flex;
    gap: var(--space-1);
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .toggle:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .arrow {
    width: 1ch;
    color: var(--text-muted);
  }

  .leaf {
    padding-left: calc(1ch + var(--space-1));
  }

  .key {
    color: var(--text);
  }

  .colon,
  .summary {
    color: var(--text-muted);
  }

  .summary {
    margin-left: var(--space-2);
  }

  mark {
    border-radius: 2px;
    background: var(--level-warning-soft);
    color: var(--text);
  }

  /* Faded, not hidden: the buttons stay in the Tab order and show when focused */
  .actions {
    margin-left: var(--space-2);
    opacity: 0;
  }

  .row:hover .actions,
  .row:focus-within .actions {
    opacity: 1;
  }

  .copy {
    padding: 0 var(--space-1);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: 11px;
    cursor: pointer;
  }

  .copy:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
