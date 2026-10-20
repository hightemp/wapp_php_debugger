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
    confirmText,
    onconfirm,
    oncancel,
  }: {
    open: boolean;
    title: string;
    message: string;
    confirmLabel: string;
    danger?: boolean;
    /** Text to type before the action is allowed: the project name for changes on stage or prod. */
    confirmText?: string;
    onconfirm: () => void;
    oncancel: () => void;
  } = $props();

  let typed = $state('');

  $effect(() => {
    if (open) {
      typed = '';
    }
  });

  const allowed = $derived(confirmText === undefined || typed === confirmText);
</script>

<Dialog {open} {title} role="alertdialog" onclose={oncancel}>
  <p class="message">{message}</p>
  {#if confirmText !== undefined}
    <label class="field">
      <span>{i18n.t('dialog.typeToConfirm', { text: confirmText })}</span>
      <input bind:value={typed} autocomplete="off" spellcheck="false" />
    </label>
  {/if}
  {#snippet footer()}
    <button type="button" class="button" onclick={oncancel}>{i18n.t('dialog.cancel')}</button>
    <button
      type="button"
      class="button primary"
      class:danger
      disabled={!allowed}
      onclick={() => allowed && onconfirm()}>{confirmLabel}</button
    >
  {/snippet}
</Dialog>

<style>
  .message {
    margin: 0;
  }

  .field {
    display: grid;
    gap: var(--space-1);
    margin-top: var(--space-3);
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  input {
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
    font-family: var(--font-mono);
    font-size: var(--font-size);
  }

  input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }

  .button:disabled {
    cursor: not-allowed;
    opacity: 0.5;
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
