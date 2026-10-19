<script lang="ts">
  import { ApiError } from '../lib/api/client';
  import type { ServerInput } from '../lib/api/types';
  import { i18n } from '../lib/i18n/locale.svelte';
  import Dialog from './Dialog.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Switch from './Switch.svelte';

  type Environment = 'dev' | 'stage' | 'prod';

  let {
    open,
    onsave,
    onclose,
  }: { open: boolean; onsave: (input: ServerInput) => Promise<void>; onclose: () => void } =
    $props();

  let name = $state('');
  let host = $state('');
  let user = $state('');
  let port = $state('');
  let jumpHosts = $state('');
  let environment = $state<Environment>('dev');
  let readOnly = $state(false);
  let saving = $state(false);
  let failure = $state('');

  $effect(() => {
    if (open) {
      name = host = user = port = jumpHosts = failure = '';
      environment = 'dev';
      readOnly = false;
    }
  });

  const portValid = $derived(port.trim() === '' || /^\d{1,5}$/.test(port.trim()));
  const complete = $derived(name.trim() !== '' && host.trim() !== '' && portValid);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!complete || saving) {
      return;
    }
    // Only what was filled in: the rest comes from ~/.ssh/config
    const config: NonNullable<ServerInput['config']> = { host: host.trim() };
    if (user.trim() !== '') {
      config.user = user.trim();
    }
    if (port.trim() !== '') {
      config.port = Number(port.trim());
    }
    const jumps = jumpHosts
      .split(',')
      .map((jump) => jump.trim())
      .filter((jump) => jump !== '');
    if (jumps.length > 0) {
      config.jump_hosts = jumps;
    }
    const input: ServerInput = { name: name.trim(), kind: 'ssh', environment, config };
    if (readOnly) {
      input.permissions = { read_only: true, allow_delete: false, allow_control: false };
    }
    saving = true;
    failure = '';
    try {
      await onsave(input);
    } catch (error) {
      failure = error instanceof ApiError && error.detail ? error.detail : String(error);
    } finally {
      saving = false;
    }
  }
</script>

<Dialog {open} title={i18n.t('server.newTitle')} {onclose}>
  <form id="server-form" class="form" onsubmit={submit}>
    <label class="field">
      <span>{i18n.t('server.name')}</span>
      <input bind:value={name} placeholder="stage" autocomplete="off" />
    </label>
    <label class="field">
      <span>{i18n.t('server.host')}</span>
      <input bind:value={host} placeholder="stage-web" autocomplete="off" />
    </label>
    <div class="row">
      <label class="field">
        <span>{i18n.t('server.user')}</span>
        <input bind:value={user} placeholder="deploy" autocomplete="off" />
      </label>
      <label class="field port">
        <span>{i18n.t('server.port')}</span>
        <input
          bind:value={port}
          placeholder="22"
          inputmode="numeric"
          autocomplete="off"
          aria-invalid={!portValid}
        />
      </label>
    </div>
    <label class="field">
      <span>{i18n.t('server.jumpHosts')}</span>
      <input bind:value={jumpHosts} placeholder={i18n.t('server.jumpHint')} autocomplete="off" />
    </label>
    <div class="field">
      <span>{i18n.t('server.environment')}</span>
      <SegmentedControl
        label={i18n.t('server.environment')}
        options={[
          { value: 'dev', label: 'dev' },
          { value: 'stage', label: 'stage' },
          { value: 'prod', label: 'prod' },
        ]}
        value={environment}
        onchange={(value: Environment) => (environment = value)}
      />
    </div>
    <div class="switch-field">
      <Switch
        label={i18n.t('server.readOnly')}
        checked={readOnly}
        onchange={(value) => (readOnly = value)}
      />
      <span aria-hidden="true">{i18n.t('server.readOnly')}</span>
      <span class="hint">{i18n.t('server.readOnlyHint')}</span>
    </div>
    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
  </form>
  {#snippet footer()}
    <button type="button" class="button" onclick={onclose}>{i18n.t('dialog.cancel')}</button>
    <button type="submit" form="server-form" class="button primary" disabled={!complete || saving}>
      {i18n.t('project.save')}
    </button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: var(--space-3);
  }

  .row {
    display: grid;
    grid-template-columns: 1fr 8ch;
    gap: var(--space-3);
  }

  .field {
    display: grid;
    gap: var(--space-1);
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

  input[aria-invalid='true'] {
    border-color: var(--level-error);
  }

  input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }

  .switch-field {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    align-items: center;
    font-size: var(--font-size-small);
  }

  .hint {
    color: var(--text-muted);
  }

  .failure {
    margin: 0;
    color: var(--level-error);
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

  .button:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .primary {
    border-color: var(--accent);
  }

  .button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
