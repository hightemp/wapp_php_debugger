<script lang="ts">
  // Modal dialog instead of native ones: labelled, traps Tab, closes on Escape, returns the focus.
  import type { Snippet } from 'svelte';
  import { i18n } from '../lib/i18n/locale.svelte';

  let {
    open,
    title,
    role = 'dialog',
    onclose,
    children,
    footer,
  }: {
    open: boolean;
    title: string;
    role?: 'dialog' | 'alertdialog';
    onclose: () => void;
    children: Snippet;
    footer?: Snippet;
  } = $props();

  let panel = $state<HTMLElement>();
  const titleId = `dialog-title-${Math.random().toString(36).slice(2)}`;

  const focusable = () =>
    [
      ...(panel?.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
      ) ?? []),
    ].filter((element) => !element.hasAttribute('disabled'));

  $effect(() => {
    if (!open) {
      return;
    }
    const previous = document.activeElement as HTMLElement | null;
    queueMicrotask(() => (focusable()[1] ?? focusable()[0])?.focus());
    return () => previous?.focus();
  });

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      // A dialog opened from another one: only the top one closes
      event.stopPropagation();
      onclose();
      return;
    }
    if (event.key !== 'Tab') {
      return;
    }
    const elements = focusable();
    const first = elements[0];
    const last = elements[elements.length - 1];
    if (!first || !last) {
      return;
    }
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }
</script>

{#if open}
  <div
    class="backdrop"
    role="presentation"
    onclick={(event) => event.target === event.currentTarget && onclose()}
  >
    <div
      bind:this={panel}
      class="panel"
      {role}
      aria-modal="true"
      aria-labelledby={titleId}
      tabindex="-1"
      onkeydown={onKeydown}
    >
      <header class="header">
        <h2 id={titleId} class="title">{title}</h2>
        <button type="button" class="close" aria-label={i18n.t('dialog.close')} onclick={onclose}
          >×</button
        >
      </header>
      <div class="body">{@render children()}</div>
      {#if footer}
        <footer class="footer">{@render footer()}</footer>
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 10;
    display: grid;
    place-items: center;
    padding: var(--space-6);
    background: rgb(0 0 0 / 45%);
  }

  .panel {
    width: min(560px, 100%);
    max-height: 100%;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-small);
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
  }

  .title {
    margin: 0;
    font-size: 15px;
  }

  .close {
    padding: 0 var(--space-2);
    border: 0;
    background: none;
    color: var(--text-muted);
    font-size: 18px;
    cursor: pointer;
  }

  .body {
    padding: var(--space-4);
  }

  .footer {
    display: flex;
    gap: var(--space-3);
    justify-content: flex-end;
    padding: var(--space-3) var(--space-4);
    border-top: 1px solid var(--border);
  }

  .close:focus-visible {
    outline: 2px solid var(--accent);
  }
</style>
