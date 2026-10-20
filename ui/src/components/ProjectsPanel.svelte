<script lang="ts">
  import { onMount } from 'svelte';
  import type { Transport } from '../lib/api/client';
  import type { Project, Server } from '../lib/api/types';
  import { clockSkew } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import ProjectDialog, { type ProjectFields } from './ProjectDialog.svelte';
  import ServersDialog from './ServersDialog.svelte';
  import Switch from './Switch.svelte';

  let {
    transport,
    selectedId,
    selectedServerId,
    onselect,
    onselectserver = () => {},
  }: {
    transport: Transport;
    selectedId?: number;
    selectedServerId?: number;
    onselect: (id: number | undefined) => void;
    /** A server header shows the feed of that server. */
    onselectserver?: (id: number) => void;
  } = $props();

  let projects = $state<Project[]>([]);
  let servers = $state<Server[]>([]);
  let serversOpen = $state(false);
  let debuggerOn = $state(false);
  let dialog = $state<'none' | 'create' | 'edit'>('none');
  let confirm = $state<'none' | 'clear' | 'remove' | 'debugger'>('none');
  let pendingDebugger = $state(false);

  const selected = $derived(projects.find((project) => project.id === selectedId));
  const selectedServer = $derived(servers.find((server) => server.id === selected?.server_id));
  // Changes on stage and prod are confirmed; clearing and removing — by typing the project name
  const risky = $derived(
    selectedServer?.environment === 'stage' || selectedServer?.environment === 'prod',
  );

  // Projects under their servers; with only the local server the list stays flat
  const grouped = $derived(servers.length > 1);
  const groups = $derived(
    servers
      .map((server) => ({
        server,
        projects: projects.filter((project) => project.server_id === server.id),
      }))
      .filter((group) => group.projects.length > 0),
  );
  const ungrouped = $derived(
    grouped
      ? projects.filter((project) => !servers.some((server) => server.id === project.server_id))
      : projects,
  );

  async function load() {
    const [serverList, list] = await Promise.all([
      transport.listServers(),
      transport.listProjects(),
    ]);
    servers = serverList.items;
    projects = list.items;
  }

  onMount(() => {
    void load().catch(() => {});
  });

  // The connection state of servers follows the stream
  $effect(() =>
    transport.subscribe((event) => {
      if (event.event === 'server_status' && event.server_id !== undefined && event.connection) {
        const connection = event.connection;
        servers = servers.map((server) =>
          server.id === event.server_id ? { ...server, connection } : server,
        );
      }
    }),
  );

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

  function toggleDebugger(enabled: boolean) {
    if (risky) {
      pendingDebugger = enabled;
      confirm = 'debugger';
      return;
    }
    void setDebugger(enabled);
  }

  async function setDebugger(enabled: boolean) {
    confirm = 'none';
    if (selectedId === undefined) {
      return;
    }
    const state = await transport.setDebuggerState(selectedId, enabled);
    debuggerOn = state.enabled;
  }

  async function save(fields: ProjectFields) {
    if (dialog === 'edit' && selected) {
      await transport.updateProject(selected.id, fields);
    } else {
      const { server_id: chosen, ...rest } = fields;
      const serverId = chosen ?? servers.find((server) => server.kind === 'local')?.id;
      if (serverId === undefined) {
        return;
      }
      const created = await transport.createProject({ ...rest, server_id: serverId });
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
    <button type="button" class="add" onclick={() => (serversOpen = true)}
      >{i18n.t('servers.open')}</button
    >
    <button type="button" class="add" onclick={() => (dialog = 'create')}
      >{i18n.t('projects.add')}</button
    >
  </div>

  <ul class="list">
    <li>
      <button
        type="button"
        class="item"
        aria-current={selectedId === undefined && selectedServerId === undefined
          ? 'true'
          : undefined}
        onclick={() => onselect(undefined)}
      >
        {i18n.t('projects.all')}
      </button>
    </li>
    {#each ungrouped as project (project.id)}
      <li>{@render projectButton(project)}</li>
    {/each}
    {#if grouped}
      {#each groups as { server, projects: serverProjects } (server.id)}
        <li class="group-item">
          <div class="group" role="group" aria-label={server.name}>
            <div class="server">
              <button
                type="button"
                class="server-name"
                aria-current={server.id === selectedServerId ? 'true' : undefined}
                onclick={() => onselectserver(server.id)}>{server.name}</button
              >
              {#if server.environment !== 'local'}
                <span class="environment" data-environment={server.environment}
                  >{server.environment}</span
                >
              {/if}
              <span class="state" data-state={server.connection.state}
                >{i18n.t(`connection.${server.connection.state}`)}</span
              >
              {#if clockSkew(server.connection.clock_offset_ms)}
                <span class="clock" title={i18n.t('servers.clockHint')}
                  >{i18n.t('servers.clock', {
                    offset: clockSkew(server.connection.clock_offset_ms) ?? '',
                  })}</span
                >
              {/if}
            </div>
            <ul class="list nested">
              {#each serverProjects as project (project.id)}
                <li>{@render projectButton(project)}</li>
              {/each}
            </ul>
          </div>
        </li>
      {/each}
    {/if}
  </ul>

  {#if selected && selectedServer && selectedServer.environment !== 'local'}
    <div
      class="banner"
      role="status"
      aria-label={i18n.t('projects.environment')}
      data-environment={selectedServer.environment}
    >
      {selectedServer.environment} · {selectedServer.name}
    </div>
  {/if}

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

{#snippet projectButton(project: Project)}
  <button
    type="button"
    class="item"
    aria-current={project.id === selectedId ? 'true' : undefined}
    onclick={() => onselect(project.id)}
  >
    {project.name}
  </button>
{/snippet}

<ServersDialog
  open={serversOpen}
  {transport}
  onclose={() => (serversOpen = false)}
  onchange={() => void load().catch(() => {})}
/>
<ProjectDialog
  {servers}
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
  confirmText={risky ? selected?.name : undefined}
  onconfirm={clearLogs}
  oncancel={() => (confirm = 'none')}
/>
<ConfirmDialog
  open={confirm === 'remove'}
  title={i18n.t('projects.removeTitle', { name: selected?.name ?? '' })}
  message={i18n.t('projects.removeMessage')}
  confirmLabel={i18n.t('projects.removeConfirm')}
  danger
  confirmText={risky ? selected?.name : undefined}
  onconfirm={removeProject}
  oncancel={() => (confirm = 'none')}
/>
<ConfirmDialog
  open={confirm === 'debugger'}
  title={i18n.t(pendingDebugger ? 'projects.debuggerOnTitle' : 'projects.debuggerOffTitle', {
    environment: selectedServer?.environment ?? '',
  })}
  message={i18n.t(pendingDebugger ? 'projects.debuggerOnMessage' : 'projects.debuggerOffMessage', {
    server: selectedServer?.name ?? '',
  })}
  confirmLabel={i18n.t(pendingDebugger ? 'projects.turnOn' : 'projects.turnOff')}
  danger
  onconfirm={() => void setDebugger(pendingDebugger)}
  oncancel={() => (confirm = 'none')}
/>

<style>
  .clock {
    color: var(--level-warning);
    font-size: var(--font-size-small);
  }

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

  .group-item {
    flex-basis: 100%;
  }

  .group {
    display: grid;
    gap: var(--space-2);
  }

  .server {
    display: flex;
    gap: var(--space-3);
    align-items: baseline;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .server-name {
    padding: 0;
    border: 0;
    border-bottom: 1px dashed transparent;
    background: none;
    color: var(--text);
    font: inherit;
    font-weight: 600;
    cursor: pointer;
  }

  .server-name:hover,
  .server-name[aria-current='true'] {
    border-bottom-color: currentcolor;
  }

  .server-name:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .environment,
  .banner {
    font-family: var(--font-mono);
  }

  [data-environment='dev'] {
    color: var(--env-dev);
  }

  [data-environment='stage'] {
    color: var(--env-stage);
  }

  [data-environment='prod'] {
    color: var(--env-prod);
  }

  .state[data-state='online'] {
    color: var(--level-info);
  }

  .state[data-state='offline'] {
    color: var(--level-error);
  }

  .list.nested {
    padding: 0;
  }

  .banner {
    margin: 0 var(--space-4) var(--space-2);
    padding: var(--space-1) var(--space-3);
    border-left: 4px solid currentcolor;
    background: var(--surface-sunken);
    font-size: var(--font-size-small);
    font-weight: 600;
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
