<script lang="ts">
  import { ApiError, type Transport } from '../lib/api/client';
  import type { Project, Server } from '../lib/api/types';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { emitCommand } from '../lib/commands';
  import { Folder, Help } from '../lib/icons';
  import Dialog from './Dialog.svelte';
  import IngestToken from './IngestToken.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Select from './Select.svelte';

  type Editor = Project['editor'];
  export interface ProjectFields {
    server_id?: number; // only for a new project
    name: string;
    root_path: string;
    logs_dir: string;
    local_path?: string;
    config_path?: string;
    editor: Editor;
    retention?: { days: number | null; max_bytes: number | null };
  }

  let {
    open,
    project,
    servers = [],
    transport,
    onsave,
    onclose,
  }: {
    open: boolean;
    project?: Project;
    /** Servers a new project can go to; the choice shows when there is more than one. */
    servers?: Server[];
    /** For settings that apply at once, like the push token; editing only */
    transport?: Transport;
    onsave: (fields: ProjectFields) => Promise<void>;
    onclose: () => void;
  } = $props();

  const editors: Editor[] = ['vscode', 'phpstorm', 'cursor', 'sublime', 'none'];

  let name = $state('');
  let rootPath = $state('');
  let logsDir = $state('');
  let localPath = $state('');
  let configPath = $state('');
  let editor = $state<Editor>('vscode');
  let serverId = $state<number | undefined>(undefined);
  let keepDays = $state('');
  let keepMegabytes = $state('');
  let saving = $state(false);
  let failure = $state('');

  // Reset the draft every time the dialog opens
  $effect(() => {
    if (open) {
      name = project?.name ?? '';
      rootPath = project?.root_path ?? '';
      logsDir = project?.logs_dir ?? '';
      localPath = project?.local_path ?? '';
      configPath = project ? project.config_path : '';
      editor = project?.editor ?? 'vscode';
      serverId = (servers.find((server) => server.kind === 'local') ?? servers[0])?.id;
      keepDays = project?.retention.days ? String(project.retention.days) : '';
      keepMegabytes = project?.retention.max_bytes
        ? String(Math.max(1, Math.round(project.retention.max_bytes / megabyte)))
        : '';
      failure = '';
    }
  });

  // An http server is one application: its library knows the logs directory and the config
  const viaLibrary = $derived(
    servers.find((server) => server.id === (project?.server_id ?? serverId))?.kind === 'http',
  );
  const megabyte = 1024 * 1024;
  // An empty field means no limit; anything else must be a whole number above zero
  const limit = (value: string): number | null | undefined =>
    value.trim() === '' ? null : /^[1-9]\d{0,6}$/.test(value.trim()) ? Number(value) : undefined;
  const retention = $derived({ days: limit(keepDays), megabytes: limit(keepMegabytes) });
  const retentionValid = $derived(
    retention.days !== undefined && retention.megabytes !== undefined,
  );
  const complete = $derived(
    name.trim() !== '' &&
      rootPath.trim() !== '' &&
      (viaLibrary || logsDir.trim() !== '') &&
      retentionValid,
  );

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!complete || saving) {
      return;
    }
    const fields: ProjectFields = {
      name: name.trim(),
      root_path: rootPath.trim(),
      logs_dir: viaLibrary ? (project?.logs_dir ?? '/') : logsDir.trim(),
      editor,
    };
    if (!project && serverId !== undefined) {
      fields.server_id = serverId;
    }
    const days = retention.days ?? null;
    const megabytes = retention.megabytes ?? null;
    const maxBytes =
      // Unchanged megabytes keep the exact byte limit set through the API
      project?.retention.max_bytes &&
      keepMegabytes === String(Math.max(1, Math.round(project.retention.max_bytes / megabyte)))
        ? project.retention.max_bytes
        : megabytes === null
          ? null
          : megabytes * megabyte;
    if (project || days !== null || maxBytes !== null) {
      fields.retention = { days, max_bytes: maxBytes };
    }
    // A new project sends only what was filled in; an edited one sends empty values to clear them
    if (project || localPath.trim() !== '') {
      fields.local_path = localPath.trim();
    }
    if (
      !viaLibrary &&
      ((project && configPath.trim() !== project.config_path) ||
        (!project && configPath.trim() !== ''))
    ) {
      fields.config_path = configPath.trim();
    }
    saving = true;
    failure = '';
    try {
      await onsave(fields);
    } catch (error) {
      failure = error instanceof ApiError && error.detail ? error.detail : String(error);
    } finally {
      saving = false;
    }
  }
</script>

<Dialog
  {open}
  title={i18n.t(project ? 'project.editTitle' : 'project.newTitle')}
  icon={Folder}
  {onclose}
>
  <form id="project-form" class="form" onsubmit={submit}>
    {#if !project}
      <button
        type="button"
        class="btn btn-ghost help"
        onclick={() => emitCommand({ type: 'help', topic: 'projects' })}
        ><Help size={14} />{i18n.t('project.help')}</button
      >
    {/if}
    {#if !project && servers.length > 1 && serverId !== undefined}
      <div class="field">
        <span>{i18n.t('project.server')}</span>
        <Select
          label={i18n.t('project.server')}
          options={servers.map((server) => ({
            value: server.id,
            label: server.name,
            hint: server.config.host ?? server.config.base_url ?? undefined,
          }))}
          value={serverId}
          onchange={(value: number) => (serverId = value)}
        />
      </div>
    {/if}
    <label class="field">
      <span>{i18n.t('project.name')}</span>
      <input class="input mono" bind:value={name} autocomplete="off" />
    </label>
    <label class="field">
      <span>{i18n.t('project.rootPath')}</span>
      <input
        class="input mono"
        bind:value={rootPath}
        placeholder="/var/www/app"
        autocomplete="off"
      />
    </label>
    {#if !viaLibrary}
      <label class="field">
        <span>{i18n.t('project.logsDir')}</span>
        <input
          class="input mono"
          bind:value={logsDir}
          placeholder="/var/www/app/tmp/logs"
          autocomplete="off"
        />
      </label>
    {/if}
    <label class="field">
      <span>{i18n.t('project.localPath')}</span>
      <input
        class="input mono"
        bind:value={localPath}
        placeholder={i18n.t('project.optional')}
        autocomplete="off"
      />
    </label>
    {#if !viaLibrary}
      <label class="field">
        <span>{i18n.t('project.configPath')}</span>
        <input
          class="input mono"
          bind:value={configPath}
          placeholder={i18n.t('project.optional')}
          autocomplete="off"
        />
      </label>
    {/if}
    <div class="form-row">
      <label class="field">
        <span>{i18n.t('project.keepDays')}</span>
        <input
          class="input mono"
          bind:value={keepDays}
          inputmode="numeric"
          placeholder={i18n.t('project.noLimit')}
          autocomplete="off"
          aria-invalid={retention.days === undefined}
        />
      </label>
      <label class="field">
        <span>{i18n.t('project.keepMegabytes')}</span>
        <input
          class="input mono"
          bind:value={keepMegabytes}
          inputmode="numeric"
          placeholder={i18n.t('project.noLimit')}
          autocomplete="off"
          aria-invalid={retention.megabytes === undefined}
        />
      </label>
    </div>
    <p class="hint">{i18n.t('project.retentionHint')}</p>
    <div class="field">
      <span>{i18n.t('project.editor')}</span>
      <SegmentedControl
        label={i18n.t('project.editor')}
        options={editors.map((value) => ({ value, label: i18n.t(`editor.${value}`) }))}
        value={editor}
        onchange={(value: Editor) => (editor = value)}
      />
    </div>
    {#if project && transport}
      <IngestToken {transport} {project} onchange={() => {}} />
    {/if}
    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn btn-large" onclick={onclose}>{i18n.t('dialog.cancel')}</button>
    <button
      type="submit"
      form="project-form"
      class="btn btn-large btn-primary"
      disabled={!complete || saving}
    >
      {i18n.t('project.save')}
    </button>
  {/snippet}
</Dialog>

<style>
  .help {
    justify-self: start;
    padding-left: 0;
  }
</style>
