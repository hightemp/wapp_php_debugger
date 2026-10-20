<script lang="ts">
  import JsonChildren from './JsonChildren.svelte';
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
    <JsonChildren class="root" {entries} depth={1} {expandDepth} />
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
</style>
