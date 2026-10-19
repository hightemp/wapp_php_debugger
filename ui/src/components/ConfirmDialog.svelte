<script lang="ts">
  // Replaces window.confirm(): a custom alertdialog with explicit buttons.
  import { i18n } from '../lib/i18n/locale.svelte';
  import Dialog from './Dialog.svelte';

  let {
    open,
    title,
    message,
    confirmLabel,
    danger = false,
    onconfirm,
    oncancel,
  }: {
    open: boolean;
    title: string;
    message: string;
    confirmLabel: string;
    danger?: boolean;
    onconfirm: () => void;
    oncancel: () => void;
  } = $props();
</script>

<Dialog {open} {title} role="alertdialog" onclose={oncancel}>
  <p class="message">{message}</p>
  {#snippet footer()}
    <button type="button" class="button" onclick={oncancel}>{i18n.t('dialog.cancel')}</button>
    <button type="button" class="button primary" class:danger onclick={onconfirm}
      >{confirmLabel}</button
    >
  {/snippet}
</Dialog>

<style>
  .message {
    margin: 0;
  }

  .button {
    padding: var(--space-2) var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    cursor: pointer;
  }

  .primary {
    border-color: var(--accent);
  }

  .danger {
    border-color: var(--level-error);
    color: var(--level-error);
  }

  .button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
