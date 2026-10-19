<script lang="ts">
  import { ApiError } from '../lib/api/client';
  import type { Project } from '../lib/api/types';
  import { i18n } from '../lib/i18n/locale.svelte';
  import Dialog from './Dialog.svelte';
  import SegmentedControl from './SegmentedControl.svelte';

  type Editor = Project['editor'];
  export interface ProjectFields {
    name: string;
    root_path: string;
    logs_dir: string;
    local_path?: string;
    config_path?: string;
    editor: Editor;
  }

  let {
    open,
    project,
    onsave,
    onclose,
  }: {
    open: boolean;
    project?: Project;
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
      failure = '';
    }
  });

  const complete = $derived(name.trim() !== '' && rootPath.trim() !== '' && logsDir.trim() !== '');

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (!complete || saving) {
      return;
    }
    const fields: ProjectFields = {
      name: name.trim(),
      root_path: rootPath.trim(),
      logs_dir: logsDir.trim(),
      editor,
    };
    // A new project sends only what was filled in; an edited one sends empty values to clear them
    if (project || localPath.trim() !== '') {
      fields.local_path = localPath.trim();
    }
    if (
      (project && configPath.trim() !== project.config_path) ||
      (!project && configPath.trim() !== '')
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

<Dialog {open} title={i18n.t(project ? 'project.editTitle' : 'project.newTitle')} {onclose}>
  <form id="project-form" class="form" onsubmit={submit}>
    <label class="field">
      <span>{i18n.t('project.name')}</span>
      <input bind:value={name} autocomplete="off" />
    </label>
    <label class="field">
      <span>{i18n.t('project.rootPath')}</span>
      <input bind:value={rootPath} placeholder="/var/www/app" autocomplete="off" />
    </label>
    <label class="field">
      <span>{i18n.t('project.logsDir')}</span>
      <input bind:value={logsDir} placeholder="/var/www/app/tmp/logs" autocomplete="off" />
    </label>
    <label class="field">
      <span>{i18n.t('project.localPath')}</span>
      <input bind:value={localPath} placeholder={i18n.t('project.optional')} autocomplete="off" />
    </label>
    <label class="field">
      <span>{i18n.t('project.configPath')}</span>
      <input bind:value={configPath} placeholder={i18n.t('project.optional')} autocomplete="off" />
    </label>
    <div class="field">
      <span>{i18n.t('project.editor')}</span>
      <SegmentedControl
        label={i18n.t('project.editor')}
        options={editors.map((value) => ({ value, label: i18n.t(`editor.${value}`) }))}
        value={editor}
        onchange={(value: Editor) => (editor = value)}
      />
    </div>
    {#if failure}
      <p class="failure" role="alert">{failure}</p>
    {/if}
  </form>
  {#snippet footer()}
    <button type="button" class="button" onclick={onclose}>{i18n.t('dialog.cancel')}</button>
    <button type="submit" form="project-form" class="button primary" disabled={!complete || saving}>
      {i18n.t('project.save')}
    </button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: var(--space-3);
  }

  .field {
    display: grid;
    gap: var(--space-1);
    font-size: var(--font-size-small);
    color: var(--text-muted);
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
