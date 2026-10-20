<script lang="ts">
  import { i18n } from '../lib/i18n/locale.svelte';
  import { toasts } from '../lib/toasts.svelte';
</script>

<section class="toasts" aria-label={i18n.t('toasts.label')} aria-live="polite">
  {#each toasts.items as toast (toast.id)}
    <div class="toast" data-kind={toast.kind}>
      <span class="text">{toast.text}</span>
      {#if toast.link}
        <a href={toast.link.href} onclick={() => toasts.close(toast.id)}>{toast.link.label}</a>
      {/if}
      <button
        type="button"
        class="close"
        aria-label={i18n.t('toasts.close')}
        onclick={() => toasts.close(toast.id)}>×</button
      >
    </div>
  {/each}
</section>

<style>
  .toasts {
    position: fixed;
    right: var(--space-4);
    bottom: var(--space-4);
    z-index: 30;
    display: grid;
    gap: var(--space-2);
    max-width: min(420px, calc(100vw - 2 * var(--space-4)));
  }

  .toast {
    display: flex;
    gap: var(--space-3);
    align-items: baseline;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-left: 4px solid var(--accent);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-small);
    font-size: var(--font-size-small);
  }

  .toast[data-kind='error'] {
    border-left-color: var(--level-error);
  }

  .text {
    flex: 1;
    overflow-wrap: anywhere;
  }

  a {
    color: var(--accent);
  }

  .close {
    padding: 0;
    border: 0;
    background: none;
    color: var(--text-muted);
    font: inherit;
    font-size: 16px;
    line-height: 1;
    cursor: pointer;
  }

  a:focus-visible,
  .close:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
