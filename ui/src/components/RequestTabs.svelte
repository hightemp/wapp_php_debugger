<script lang="ts">
  import type { Transport } from '../lib/api/client';
  import { requestTitle } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { openTabs } from '../lib/tabs.svelte';

  let {
    transport,
    currentId,
    onselect,
    oncompare,
  }: {
    transport: Transport;
    currentId: number;
    onselect: (id: number) => void;
    /** Compare the open request with this one */
    oncompare: (id: number) => void;
  } = $props();

  $effect(() => {
    const id = currentId;
    openTabs.open(id);
    if (openTabs.items.find((tab) => tab.id === id)?.title === undefined) {
      transport
        .getRequest(id)
        .then((request) => openTabs.name(id, requestTitle(request)))
        .catch(() => {});
    }
  });

  const title = (tab: { id: number; title?: string }) => tab.title ?? `#${tab.id}`;

  function close(id: number) {
    const neighbour = openTabs.close(id);
    if (id === currentId && neighbour !== undefined) {
      onselect(neighbour);
    }
  }
</script>

{#if openTabs.items.length > 1}
  <nav class="tabs" aria-label={i18n.t('tabs.label')}>
    {#each openTabs.items as tab (tab.id)}
      <span class="tab" data-current={tab.id === currentId}>
        <button
          type="button"
          class="open"
          aria-current={tab.id === currentId ? 'page' : undefined}
          title={title(tab)}
          onclick={() => onselect(tab.id)}>{title(tab)}</button
        >
        {#if tab.id !== currentId}
          <button
            type="button"
            class="icon"
            aria-label={i18n.t('tabs.compare', { title: title(tab) })}
            title={i18n.t('tabs.compare', { title: title(tab) })}
            onclick={() => oncompare(tab.id)}>⇄</button
          >
        {/if}
        <button
          type="button"
          class="icon"
          aria-label={i18n.t('tabs.close', { title: title(tab) })}
          onclick={() => close(tab.id)}>×</button
        >
      </span>
    {/each}
  </nav>
{/if}

<style>
  .tabs {
    display: flex;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-3) 0;
    overflow-x: auto;
    border-bottom: 1px solid var(--border);
  }

  .tab {
    display: inline-flex;
    flex: none;
    align-items: center;
    max-width: 22ch;
    border: 1px solid var(--border);
    border-bottom: none;
    border-radius: var(--radius) var(--radius) 0 0;
    background: var(--surface-sunken);
    font-size: var(--font-size-small);
  }

  .tab[data-current='true'] {
    background: var(--surface);
  }

  .open {
    overflow: hidden;
    padding: var(--space-1) var(--space-2);
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    font-family: var(--font-mono);
    text-overflow: ellipsis;
    white-space: nowrap;
    cursor: pointer;
  }

  .icon {
    padding: 0 var(--space-1);
    border: 0;
    background: none;
    color: var(--text-muted);
    font: inherit;
    cursor: pointer;
  }

  .icon:hover {
    color: var(--text);
  }

  .open:focus-visible,
  .icon:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
</style>
