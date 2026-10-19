<script lang="ts">
  import { onMount } from 'svelte';
  import type { Transport } from '../lib/api/client';
  import type { Project } from '../lib/api/types';
  import { i18n } from '../lib/i18n/locale.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import ProjectDialog, { type ProjectFields } from './ProjectDialog.svelte';
  import Switch from './Switch.svelte';

  let {
    transport,
    selectedId,
    onselect,
  }: { transport: Transport; selectedId?: number; onselect: (id: number | undefined) => void } =
    $props();

  let projects = $state<Project[]>([]);
  let localServerId = $state<number | undefined>(undefined);
  let debuggerOn = $state(false);
  let dialog = $state<'none' | 'create' | 'edit'>('none');
  let confirm = $state<'none' | 'clear' | 'remove'>('none');

  const selected = $derived(projects.find((project) => project.id === selectedId));

  async function load() {
    const [servers, list] = await Promise.all([transport.listServers(), transport.listProjects()]);
    localServerId = servers.items.find((server) => server.kind === 'local')?.id;
    projects = list.items;
  }

  onMount(() => {
    void load().catch(() => {});
  });

  $effect(() => {
    const id = selectedId;
    debuggerOn = false;
    if (id !== undefined) {
      transport
        .getDebuggerState(id)
        .then((state) => (debuggerOn = state.enabled))
        .catch(() => {});
    }
  });

  async function toggleDebugger(enabled: boolean) {
    if (selectedId === undefined) {
      return;
    }
    const state = await transport.setDebuggerState(selectedId, enabled);
    debuggerOn = state.enabled;
  }

  async function save(fields: ProjectFields) {
    if (dialog === 'edit' && selected) {
      await transport.updateProject(selected.id, fields);
    } else if (localServerId !== undefined) {
      const created = await transport.createProject({ ...fields, server_id: localServerId });
      onselect(created.id);
    }
    dialog = 'none';
    await load();
  }

  async function clearLogs() {
    confirm = 'none';
    if (selectedId !== undefined) {
      await transport.clearProjectLogs(selectedId);
    }
  }

  async function removeProject() {
    confirm = 'none';
    if (selectedId === undefined) {
      return;
    }
    await transport.deleteProject(selectedId);
    onselect(undefined);
    await load();
  }
</script>

<nav class="projects" aria-label={i18n.t('projects.label')}>
  <div class="head">
    <h2 class="title">{i18n.t('projects.label')}</h2>
    <button type="button" class="add" onclick={() => (dialog = 'create')}
      >{i18n.t('projects.add')}</button
    >
  </div>

  <ul class="list">
    <li>
      <button
        type="button"
        class="item"
        aria-current={selectedId === undefined ? 'true' : undefined}
        onclick={() => onselect(undefined)}
      >
        {i18n.t('projects.all')}
      </button>
    </li>
    {#each projects as project (project.id)}
      <li>
        <button
          type="button"
          class="item"
          aria-current={project.id === selectedId ? 'true' : undefined}
          onclick={() => onselect(project.id)}
        >
          {project.name}
        </button>
      </li>
    {/each}
  </ul>

  {#if selected}
    <div class="actions">
      <label class="debugger">
        <span>{i18n.t('projects.debugger')}</span>
        <Switch
          label={i18n.t('projects.debugger')}
          checked={debuggerOn}
          onchange={toggleDebugger}
        />
      </label>
      <button type="button" class="action" onclick={() => (dialog = 'edit')}
        >{i18n.t('projects.settings')}</button
      >
      <button type="button" class="action" onclick={() => (confirm = 'clear')}
        >{i18n.t('projects.clear')}</button
      >
      <button type="button" class="action danger" onclick={() => (confirm = 'remove')}>
        {i18n.t('projects.remove')}
      </button>
    </div>
  {/if}
</nav>

<ProjectDialog
  open={dialog !== 'none'}
  project={dialog === 'edit' ? selected : undefined}
  onsave={save}
  onclose={() => (dialog = 'none')}
/>
<ConfirmDialog
  open={confirm === 'clear'}
  title={i18n.t('projects.clearTitle')}
  message={i18n.t('projects.clearMessage', { name: selected?.name ?? '' })}
  confirmLabel={i18n.t('projects.clearConfirm')}
  danger
  onconfirm={clearLogs}
  oncancel={() => (confirm = 'none')}
/>
<ConfirmDialog
  open={confirm === 'remove'}
  title={i18n.t('projects.removeTitle', { name: selected?.name ?? '' })}
  message={i18n.t('projects.removeMessage')}
  confirmLabel={i18n.t('projects.removeConfirm')}
  danger
  onconfirm={removeProject}
  oncancel={() => (confirm = 'none')}
/>

<style>
  .projects {
    border-bottom: 1px solid var(--border);
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-2) var(--space-4);
  }

  .title {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .list {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    margin: 0;
    padding: 0 var(--space-4) var(--space-2);
    list-style: none;
  }

  .item,
  .add,
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

  .item[aria-current='true'] {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  .add {
    color: var(--accent);
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    align-items: center;
    padding: 0 var(--space-4) var(--space-3);
  }

  .debugger {
    display: inline-flex;
    gap: var(--space-2);
    align-items: center;
    margin-right: var(--space-2);
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .danger {
    color: var(--level-error);
  }

  .item:focus-visible,
  .add:focus-visible,
  .action:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
