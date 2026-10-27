<script lang="ts" module>
  // Open dialogs, the top one last: only it answers Escape and Tab
  const stack: symbol[] = [];
</script>

<script lang="ts">
  // Modal dialog instead of native ones: labelled, traps Tab, closes on Escape, returns the focus.
  // Escape and Tab work wherever the focus is: a button that disappears takes the focus to <body>.
  import type { Component, Snippet } from 'svelte';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { X } from '../lib/icons';

  let {
    open,
    title,
    role = 'dialog',
    wide = false,
    icon,
    onclose,
    children,
    footer,
  }: {
    open: boolean;
    title: string;
    role?: 'dialog' | 'alertdialog';
    /** Room for an editor next to its preview. */
    wide?: boolean;
    /** An icon before the title. */
    icon?: Component<{ size?: number }>;
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
    // The first field of a form, else the first control after the close button
    queueMicrotask(() =>
      (
        panel?.querySelector<HTMLElement>(
          '.body :is(input, textarea, select):not([disabled], [type="file"])',
        ) ??
        focusable()[1] ??
        focusable()[0]
      )?.focus(),
    );
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
      class:wide
      {role}
      aria-modal="true"
      aria-labelledby={titleId}
      tabindex="-1"
    >
      <header class="header">
        {#if icon}
          {@const Icon = icon}
          <span class="title-icon"><Icon size={16} /></span>
        {/if}
        <h2 id={titleId} class="title">{title}</h2>
        <button
          type="button"
          class="icon-btn close"
          aria-label={i18n.t('dialog.close')}
          title={i18n.t('dialog.close')}
          onclick={onclose}><X size={16} /></button
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
    background: var(--backdrop);
  }

  .panel {
    display: flex;
    flex-direction: column;
    width: min(560px, 100%);
    /* The window, not the grid area: a percentage of an auto row would not hold the panel */
    max-height: calc(100dvh - 2 * var(--space-6));
    border: 1px solid var(--border);
    border-radius: var(--radius-large);
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-large);
  }

  .panel.wide {
    width: min(1040px, 100%);
  }

  .header {
    display: flex;
    flex: none;
    gap: var(--space-3);
    align-items: center;
    padding: var(--space-3) var(--space-3) var(--space-3) var(--space-5);
    border-bottom: 1px solid var(--border);
  }

  .title-icon {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    border-radius: var(--radius);
    background: var(--accent-soft);
    color: var(--accent);
  }

  .title {
    flex: 1;
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }

  .body {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    padding: var(--space-5);
  }

  .footer {
    display: flex;
    flex: none;
    flex-wrap: wrap;
    gap: var(--space-3);
    justify-content: flex-end;
    padding: var(--space-3) var(--space-5);
    border-top: 1px solid var(--border);
  }

  @media (max-width: 600px) {
    .backdrop {
      padding: var(--space-3);
    }

    .panel {
      max-height: calc(100dvh - 2 * var(--space-3));
    }

    .body {
      padding: var(--space-4);
    }
  }
</style>
