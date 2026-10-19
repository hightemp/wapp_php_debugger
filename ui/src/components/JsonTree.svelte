<script lang="ts">
  import JsonNode from './JsonNode.svelte';
  import JsonValue from './JsonValue.svelte';

  // expandDepth: how many levels are open initially (deeper ones start collapsed)
  let { value, expandDepth = 3 }: { value: unknown; expandDepth?: number } = $props();

  const entries = $derived(
    value !== null && typeof value === 'object'
      ? Object.entries(value as Record<string, unknown>)
      : null,
  );
</script>

<div class="json">
  {#if entries === null || entries.length === 0}
    <JsonValue {value} />
  {:else}
    <ul class="root">
      {#each entries as [name, child] (name)}
        <JsonNode {name} value={child} depth={1} {expandDepth} />
      {/each}
    </ul>
  {/if}
</div>

<style>
  .json {
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    line-height: 1.6;
  }

  .root {
    margin: 0;
    padding: 0;
  }
</style>
