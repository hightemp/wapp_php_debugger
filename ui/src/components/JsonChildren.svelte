<script lang="ts">
  // Children of a JSON node, a part at a time: a context with thousands of keys stays fast.
  import { i18n } from '../lib/i18n/locale.svelte';
  import JsonNode from './JsonNode.svelte';

  let {
    entries,
    depth,
    expandDepth,
    class: className,
  }: { entries: [string, unknown][]; depth: number; expandDepth: number; class: string } = $props();

  const part = 200;
  let shown = $state(part);
  const rest = $derived(entries.length - shown);
</script>

<ul class={className}>
  {#each entries.slice(0, shown) as [name, child] (name)}
    <JsonNode {name} value={child} {depth} {expandDepth} />
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
