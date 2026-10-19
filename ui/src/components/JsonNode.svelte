<script lang="ts">
  import { i18n } from '../lib/i18n/locale.svelte';
  import JsonNode from './JsonNode.svelte';
  import JsonValue from './JsonValue.svelte';

  let {
    name,
    value,
    depth,
    expandDepth,
  }: { name: string; value: unknown; depth: number; expandDepth: number } = $props();

  const isContainer = $derived(value !== null && typeof value === 'object');
  const entries = $derived(
    isContainer ? Object.entries(value as Record<string, unknown>) : ([] as [string, unknown][]),
  );
  // svelte-ignore state_referenced_locally
  let expanded = $state(depth < expandDepth);

  const summary = $derived(
    Array.isArray(value)
      ? i18n.t('json.items', { count: entries.length })
      : i18n.t('json.keys', { count: entries.length }),
  );
</script>

<li class="node">
  {#if isContainer && entries.length > 0}
    <button
      class="toggle"
      type="button"
      aria-expanded={expanded}
      onclick={() => (expanded = !expanded)}
    >
      <span class="arrow" aria-hidden="true">{expanded ? '▾' : '▸'}</span><span class="key"
        >{name}</span
      >
    </button>
    {#if expanded}
      <ul class="children">
        {#each entries as [childName, child] (childName)}
          <JsonNode name={childName} value={child} depth={depth + 1} {expandDepth} />
        {/each}
      </ul>
    {:else}
      <span class="summary">{summary}</span>
    {/if}
  {:else}
    <span class="leaf"
      ><span class="key">{name}</span><span class="colon">:</span> <JsonValue {value} /></span
    >
  {/if}
</li>

<style>
  .node {
    list-style: none;
  }

  .children {
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
</style>
