<script lang="ts">
  // Export and import of servers and projects as YAML, without secrets: to move to another machine
  // or share a setup. Text, not only files, so it works the same in the desktop app.
  import { ApiError, type ImportResult, type Transport } from '../lib/api/client';
  import { i18n } from '../lib/i18n/locale.svelte';
  import Dialog from './Dialog.svelte';
  import SegmentedControl from './SegmentedControl.svelte';

  let {
    open,
    transport,
    onclose,
    onchange,
  }: {
    open: boolean;
    transport: Transport;
    onclose: () => void;
    /** Something was imported: servers and projects reload. */
    onchange: () => void;
  } = $props();

  type Mode = 'export' | 'import';
  let mode = $state<Mode>('export');
  let exported = $state('');
  let pasted = $state('');
  let importing = $state(false);
  let failure = $state('');
  let result = $state<ImportResult | undefined>(undefined);

  $effect(() => {
    if (open) {
      mode = 'export';
      pasted = failure = '';
      result = undefined;
      exported = '';
      transport
        .exportConfig()
        .then((text) => (exported = text))
        .catch((error: unknown) => (failure = String(error)));
    }
  });

  const download = $derived(`data:application/yaml;charset=utf-8,${encodeURIComponent(exported)}`);

  async function chooseFile(event: Event) {
    const file = (event.currentTarget as HTMLInputElement).files?.[0];
    if (file) {
      pasted = await file.text();
    }
  }

  async function importConfig() {
    if (pasted.trim() === '' || importing) {
      return;
    }
    importing = true;
    failure = '';
    result = undefined;
    try {
      result = await transport.importConfig(pasted);
      onchange();
    } catch (error) {
      failure = error instanceof ApiError && error.detail ? error.detail : String(error);
    } finally {
      importing = false;
    }
  }
</script>

<Dialog {open} title={i18n.t('config.title')} {onclose}>
  <div class="body">
    <SegmentedControl
      label={i18n.t('config.title')}
      options={[
        { value: 'export', label: i18n.t('config.export') },
        { value: 'import', label: i18n.t('config.import') },
      ]}
      value={mode}
      onchange={(value: Mode) => (mode = value)}
    />
    {#if mode === 'export'}
      <p class="note">{i18n.t('config.exportHint')}</p>
      <label class="field">
        <span>{i18n.t('config.exportLabel')}</span>
        <textarea readonly rows="14" value={exported}></textarea>
      </label>
      <a class="action" href={download} download="wapp-debugger.yaml">{i18n.t('config.download')}</a
      >
    {:else}
      <p class="note">{i18n.t('config.importHint')}</p>
      <label class="field">
        <span>{i18n.t('config.importLabel')}</span>
        <textarea rows="12" bind:value={pasted} spellcheck="false"></textarea>
      </label>
      <div class="row">
        <label class="action file">
          {i18n.t('config.chooseFile')}
          <input type="file" accept=".yaml,.yml,application/yaml,text/yaml" onchange={chooseFile} />
        </label>
        <button
          type="button"
          class="action primary"
          disabled={pasted.trim() === '' || importing}
          onclick={importConfig}>{i18n.t('config.import')}</button
        >
      </div>
      {#if result}
        <div class="summary" role="status">
          <p>
            {i18n.t('config.added', {
              items:
                [...result.servers_created, ...result.projects_created].join(', ') ||
                i18n.t('config.nothing'),
            })}
          </p>
          {#if result.skipped.length > 0}
            <p>{i18n.t('config.skipped', { items: result.skipped.join(', ') })}</p>
          {/if}
          {#if result.needs_token.length > 0}
            <p class="warning">
              {i18n.t('config.needsToken', { items: result.needs_token.join(', ') })}
            </p>
          {/if}
        </div>
      {/if}
    {/if}
    {#if failure}
      <p class="error" role="alert">{failure}</p>
    {/if}
  </div>
</Dialog>

<style>
  .body {
    display: grid;
    gap: var(--space-3);
  }

  .note,
  .summary p {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .summary {
    display: grid;
    gap: var(--space-1);
  }

  .summary p:first-child {
    color: var(--text);
  }

  .summary p.warning {
    color: var(--level-warning);
  }

  .error {
    margin: 0;
    color: var(--level-error);
    font-size: var(--font-size-small);
  }

  .field {
    display: grid;
    gap: var(--space-1);
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  textarea {
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    resize: vertical;
  }

  .row {
    display: flex;
    gap: var(--space-3);
    justify-content: space-between;
  }

  .action {
    justify-self: start;
    padding: 2px var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: var(--font-size-small);
    text-decoration: none;
    cursor: pointer;
  }

  .action:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .primary {
    border-color: var(--accent);
  }

  .file input {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
  }

  .file:focus-within,
  .action:focus-visible,
  textarea:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
