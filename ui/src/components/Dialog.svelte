<script lang="ts" module>
  // Open dialogs, the top one last: only it answers Escape and Tab
  const stack: symbol[] = [];
</script>

<script lang="ts">
  // Modal dialog instead of native ones: labelled, traps Tab, closes on Escape, returns the focus.
  // Escape and Tab work wherever the focus is: a button that disappears takes the focus to <body>.
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
  const me = Symbol(titleId);

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
    stack.push(me);
    const previous = document.activeElement as HTMLElement | null;
    queueMicrotask(() => (focusable()[1] ?? focusable()[0])?.focus());
    window.addEventListener('keydown', onKeydown);
    return () => {
      window.removeEventListener('keydown', onKeydown);
      stack.splice(stack.indexOf(me), 1);
      previous?.focus();
    };
  });

  function onKeydown(event: KeyboardEvent) {
    if (stack[stack.length - 1] !== me || event.defaultPrevented) {
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
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
    if (!panel?.contains(document.activeElement)) {
      event.preventDefault();
      (event.shiftKey ? last : first).focus();
    } else if (event.shiftKey && document.activeElement === first) {
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
