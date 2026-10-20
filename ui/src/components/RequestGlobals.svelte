<script lang="ts">
  // $_GET, $_POST, $_SERVER and the rest, as the library logged them at the start of the request.
  import type { Transport } from '../lib/api/client';
  import type { Message } from '../lib/api/types';
  import { i18n } from '../lib/i18n/locale.svelte';
  import JsonTree from './JsonTree.svelte';

  let { transport, messages }: { transport: Transport; messages: readonly Message[] } = $props();

  // Library 2.x tags them globals; the v1 converter and hand-written logs say superglobals
  const globals = $derived(
    messages.filter(
      (message) => message.tags.includes('globals') || message.tags.includes('superglobals'),
    ),
  );
  let contexts = $state<Record<number, unknown>>({});

  $effect(() => {
    for (const message of globals) {
      if (!(message.id in contexts) && message.has_context) {
        transport
          .getMessage(message.id)
          .then((detail) => (contexts[message.id] = detail.context ?? {}))
          .catch(() => (contexts[message.id] = null));
      }
    }
  });
</script>

{#if globals.length === 0}
  <p class="state">{i18n.t('globals.none')}</p>
{:else}
  {#each globals as message (message.id)}
    <section class="block" aria-label={message.message}>
      <h3>{message.message}</h3>
      {#if contexts[message.id] === undefined}
        <p class="state">{i18n.t('detail.loading')}</p>
      {:else}
        <JsonTree value={contexts[message.id]} expandDepth={1} />
      {/if}
    </section>
  {/each}
{/if}

<style>
  .state {
    margin: 0;
    padding: var(--space-3) var(--space-4);
    color: var(--text-muted);
  }

  .block {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
  }

  h3 {
    margin: 0 0 var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }
</style>
