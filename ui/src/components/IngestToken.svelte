<script lang="ts">
  // Pushed logs of a project: the token the library (push_token) sends to the receiver.
  import type { Transport } from '../lib/api/client';
  import type { Project } from '../lib/api/types';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { libraryConfig, pushUrl } from '../lib/ingest';
  import { toasts } from '../lib/toasts.svelte';

  let {
    transport,
    project,
    onchange,
  }: { transport: Transport; project: Project; onchange: (enabled: boolean) => void } = $props();

  // svelte-ignore state_referenced_locally (the starting state; the buttons change it here)
  let enabled = $state(project.ingest_enabled);
  let token = $state<string | null>(null);
  let ingestAddr = $state<string | null | undefined>(undefined);
  let busy = $state(false);

  $effect(() => {
    transport
      .getVersion()
      .then((version) => (ingestAddr = version.ingest_addr ?? null))
      .catch(() => (ingestAddr = null));
  });

  const config = $derived(
    token ? libraryConfig(pushUrl(ingestAddr, window.location.hostname), token) : '',
  );

  async function make() {
    busy = true;
    try {
      token = (await transport.createIngestToken(project.id)).token;
      enabled = true;
      onchange(true);
    } catch (error) {
      toasts.push({ kind: 'error', text: error instanceof Error ? error.message : String(error) });
    } finally {
      busy = false;
    }
  }

  async function stop() {
    busy = true;
    try {
      await transport.deleteIngestToken(project.id);
      token = null;
      enabled = false;
      onchange(false);
    } catch (error) {
      toasts.push({ kind: 'error', text: error instanceof Error ? error.message : String(error) });
    } finally {
      busy = false;
    }
  }
</script>

<section class="ingest" aria-labelledby="ingest-title">
  <h3 id="ingest-title">{i18n.t('ingest.title')}</h3>
  <p class="state">{i18n.t(enabled ? 'ingest.on' : 'ingest.off')}</p>
  <p class="hint">{i18n.t('ingest.hint')}</p>
  {#if ingestAddr === null}
    <p class="hint warning">{i18n.t('ingest.receiverOff')}</p>
  {/if}
  <div class="buttons">
    <button type="button" class="action" disabled={busy} onclick={() => void make()}
      >{i18n.t(enabled ? 'ingest.renew' : 'ingest.make')}</button
    >
    {#if enabled}
      <button type="button" class="action" disabled={busy} onclick={() => void stop()}
        >{i18n.t('ingest.stop')}</button
      >
    {/if}
  </div>
  {#if token}
    <label class="config">
      <span>{i18n.t('ingest.config')}</span>
      <textarea readonly rows="2" aria-label={i18n.t('ingest.config')} value={config}></textarea>
    </label>
    <p class="hint">{i18n.t('ingest.once')}</p>
  {/if}
</section>

<style>
  .ingest {
    display: grid;
    gap: var(--space-2);
    padding-top: var(--space-3);
    border-top: 1px solid var(--border);
  }

  h3 {
    margin: 0;
    font-size: var(--font-size-base, 1rem);
  }

  .state,
  .hint {
    margin: 0;
  }

  .hint {
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .warning {
    color: var(--level-warning);
  }

  .buttons {
    display: flex;
    gap: var(--space-2);
  }

  .action {
    padding: 2px var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .action:focus-visible,
  textarea:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .config {
    display: grid;
    gap: var(--space-1);
    font-size: var(--font-size-small);
  }

  textarea {
    padding: var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    resize: vertical;
  }
</style>
