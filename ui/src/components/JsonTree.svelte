<script lang="ts">
  import { i18n } from '../lib/i18n/locale.svelte';
  import { matchingPaths } from '../lib/jsonPath';
  import JsonChildren from './JsonChildren.svelte';
  import JsonValue from './JsonValue.svelte';

  // expandDepth: how many levels are open initially (deeper ones start collapsed);
  // searchable: a search field over the tree (the context of a message)
  let {
    value,
    expandDepth = 3,
    searchable = false,
  }: { value: unknown; expandDepth?: number; searchable?: boolean } = $props();

  const entries = $derived(
    value !== null && typeof value === 'object'
      ? Object.entries(value as Record<string, unknown>)
      : null,
  );
  let query = $state('');
  const filter = $derived(query.trim() === '' ? null : matchingPaths(value, query.trim()));
</script>

<div class="json">
  {#if entries === null || entries.length === 0}
    <JsonValue {value} />
  {:else}
    {#if searchable}
      <input
        class="search"
        type="search"
        aria-label={i18n.t('json.search')}
        placeholder={i18n.t('json.search')}
        bind:value={query}
      />
    {/if}
    {#if filter && filter.size === 0}
      <p class="none">{i18n.t('json.nothing')}</p>
    {:else}
      <JsonChildren
        class="root"
        {entries}
        depth={1}
        {expandDepth}
        path={[]}
        indexes={[]}
        array={Array.isArray(value)}
        {filter}
        query={query.trim()}
      />
    {/if}
  {/if}
</div>

<style>
  .json {
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    line-height: 1.6;
  }

  .json :global(.root) {
    margin: 0;
    padding: 0;
  }

  .search {
    width: 100%;
    margin-bottom: var(--space-2);
    padding: var(--space-1) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    font: inherit;
  }

  .search:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }

  .none {
    margin: 0;
    color: var(--text-muted);
  }
</style>
