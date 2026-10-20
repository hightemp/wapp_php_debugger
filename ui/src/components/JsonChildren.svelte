<script lang="ts">
  // Children of a JSON node, a part at a time: a context with thousands of keys stays fast.
  import { i18n } from '../lib/i18n/locale.svelte';
  import { pathKey } from '../lib/jsonPath';
  import JsonNode from './JsonNode.svelte';

  let {
    entries,
    depth,
    expandDepth,
    class: className,
    path,
    indexes,
    array,
    filter,
    query,
  }: {
    entries: [string, unknown][];
    depth: number;
    expandDepth: number;
    class: string;
    /** Keys from the root to these children's parent, and which of them are array indexes */
    path: string[];
    indexes: boolean[];
    /** The parent is an array: the children's keys are indexes */
    array: boolean;
    /** While searching: the nodes to show; null shows everything */
    filter: Set<string> | null;
    query: string;
  } = $props();

  const part = 200;
  let shown = $state(part);
  const visible = $derived(
    filter ? entries.filter(([name]) => filter.has(pathKey([...path, name]))) : entries,
  );
  const rest = $derived(visible.length - shown);
</script>

<ul class={className}>
  {#each visible.slice(0, shown) as [name, child] (name)}
    <JsonNode
      {name}
      value={child}
      {depth}
      {expandDepth}
      path={[...path, name]}
      indexes={[...indexes, array]}
      {filter}
      {query}
    />
  {/each}
  {#if rest > 0}
    <li class="more">
      <button type="button" onclick={() => (shown += part)}
        >{i18n.t('json.more', { count: Math.min(part, rest), total: rest })}</button
      >
    </li>
  {/if}
</ul>

<style>
  .more {
    list-style: none;
  }

  button {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    cursor: pointer;
  }

  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
