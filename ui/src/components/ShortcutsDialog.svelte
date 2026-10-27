<script lang="ts">
  import { i18n } from '../lib/i18n/locale.svelte';
  import type { MessageKey } from '../lib/i18n/messages';
  import { Keyboard } from '../lib/icons';
  import Dialog from './Dialog.svelte';

  let { open, onclose }: { open: boolean; onclose: () => void } = $props();

  const shortcuts: [string, MessageKey][] = [
    ['j / k', 'keys.nextRequest'],
    ['Shift+J / Shift+K', 'keys.nextMessage'],
    ['Esc', 'keys.back'],
    ['/', 'keys.filter'],
    ['g p', 'keys.projects'],
    ['Ctrl+K', 'keys.palette'],
    ['?', 'keys.help'],
  ];
</script>

<Dialog icon={Keyboard} {open} title={i18n.t('keys.title')} {onclose}>
  <dl class="keys">
    {#each shortcuts as [keys, what] (keys)}
      <dt><kbd>{keys}</kbd></dt>
      <dd>{i18n.t(what)}</dd>
    {/each}
  </dl>
</Dialog>

<style>
  .keys {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-4);
    margin: 0;
  }

  dd {
    margin: 0;
  }

  kbd {
    padding: 0 var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }
</style>
