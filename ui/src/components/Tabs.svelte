<script lang="ts" generics="T extends string">
  // Tabs (WAI-ARIA): arrows move between tabs, the panel is labelled by its tab — panelId(id).
  let {
    label,
    tabs,
    value,
    idPrefix,
    onchange,
  }: {
    label: string;
    tabs: readonly { id: T; label: string; count?: number }[];
    value: T;
    /** Tabs are tab-<prefix>-<id>, their panels panel-<prefix>-<id> */
    idPrefix: string;
    onchange: (id: T) => void;
  } = $props();

  let list = $state<HTMLElement>();

  function onKeydown(event: KeyboardEvent) {
    const index = tabs.findIndex((tab) => tab.id === value);
    const moves: Record<string, number> = {
      ArrowRight: (index + 1) % tabs.length,
      ArrowLeft: (index - 1 + tabs.length) % tabs.length,
      Home: 0,
      End: tabs.length - 1,
    };
    const next = tabs[moves[event.key] ?? -1];
    if (next) {
      event.preventDefault();
      onchange(next.id);
      queueMicrotask(() =>
        list?.querySelector<HTMLElement>(`#tab-${idPrefix}-${next.id}`)?.focus(),
      );
    }
  }
</script>

<div
  class="tabs"
  role="tablist"
  aria-label={label}
  tabindex="-1"
  bind:this={list}
  onkeydown={onKeydown}
>
  {#each tabs as tab (tab.id)}
    <button
      type="button"
      role="tab"
      id={`tab-${idPrefix}-${tab.id}`}
      aria-controls={`panel-${idPrefix}-${tab.id}`}
      aria-selected={tab.id === value}
      tabindex={tab.id === value ? 0 : -1}
      onclick={() => onchange(tab.id)}
    >
      {#if tab.count !== undefined}{`${tab.label} `}<span class="count">{tab.count}</span
        >{:else}{tab.label}{/if}
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    gap: var(--space-1);
    padding: 0 var(--space-4);
    border-bottom: 1px solid var(--border);
  }

  button {
    padding: var(--space-2) var(--space-3);
    border: 0;
    border-bottom: 2px solid transparent;
    background: none;
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  button[aria-selected='true'] {
    border-bottom-color: var(--accent);
    color: var(--text);
  }

  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .count {
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }
</style>
