<script lang="ts">
  import { onMount } from 'svelte';
  import type { Transport } from '../lib/api/client';
  import type { Environment, Project, Server } from '../lib/api/types';
  import { clockSkew } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { onCommand } from '../lib/commands';
  import { toasts } from '../lib/toasts.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import ProjectDialog, { type ProjectFields } from './ProjectDialog.svelte';
  import ServersDialog from './ServersDialog.svelte';
  import Switch from './Switch.svelte';
  import {
    Eraser,
    Folder,
    HardDrive,
    Layers,
    Plus,
    Server as ServerIcon,
    Settings,
    Trash,
    Upload,
  } from '../lib/icons';

  let {
    transport,
    selectedId,
    selectedServerId,
    onselect,
    onselectserver = () => {},
    onopenrequest = () => {},
    onenvironment = () => {},
    collapsed = false,
  }: {
    transport: Transport;
    selectedId?: number;
    selectedServerId?: number;
    onselect: (id: number | undefined) => void;
    /** A server header shows the feed of that server. */
    onselectserver?: (id: number) => void;
    /** Opens a request of a project: the one just imported */
    onopenrequest?: (projectId: number, requestId: number) => void;
    /** The environment of the chosen project or server feed; none for all projects. */
    onenvironment?: (environment: Environment | undefined) => void;
    /** A narrow rail: icons and initials, the names as tooltips; no project actions. */
    collapsed?: boolean;
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
  const environment = $derived(
    (selectedServer ?? servers.find((server) => server.id === selectedServerId))?.environment,
  );
  $effect(() => onenvironment(environment));
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

  // A request exported elsewhere, a log file of the library, or a log of PHP, Laravel or nginx goes
  // into the selected project
  async function importRequest(event: Event & { currentTarget: HTMLInputElement }) {
    const input = event.currentTarget;
    const file = input.files?.[0];
    const projectId = selected?.id;
    input.value = '';
    if (!file || projectId === undefined) {
      return;
    }
    try {
      const result = await transport.importRequest(projectId, await file.text(), file.name);
      if (!result.created) {
        toasts.push({ text: i18n.t('transfer.exists') });
      }
      onopenrequest(projectId, result.request_id);
    } catch (error) {
      toasts.push({
        kind: 'error',
        text: i18n.t('transfer.failed', {
          error: error instanceof Error ? error.message : String(error),
        }),
      });
    }
  }

  let panel = $state<HTMLElement>();
  $effect(() =>
    onCommand((command) => {
      if (command.type === 'open-servers') {
        serversOpen = true;
      } else if (command.type === 'add-project') {
        dialog = 'create';
      } else if (command.type === 'focus-projects') {
        panel?.querySelector<HTMLElement>('.list button')?.focus();
      }
    }),
  );

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

<nav class="projects" class:collapsed aria-label={i18n.t('projects.label')} bind:this={panel}>
  <div class="head">
    <h2 class="title" class:visually-hidden={collapsed}>{i18n.t('projects.label')}</h2>
    <button
      type="button"
      class="icon-btn"
      aria-label={i18n.t('servers.open')}
      title={i18n.t('servers.open')}
      onclick={() => (serversOpen = true)}><ServerIcon size={16} /></button
    >
    <button
      type="button"
      class="icon-btn"
      aria-label={i18n.t('projects.add')}
      title={i18n.t('projects.add')}
      onclick={() => (dialog = 'create')}><Plus size={16} /></button
    >
  </div>

  <ul class="list">
    <li>
      <button
        type="button"
        class="item"
        title={collapsed ? i18n.t('projects.all') : undefined}
        aria-current={selectedId === undefined && selectedServerId === undefined
          ? 'true'
          : undefined}
        onclick={() => onselect(undefined)}
      >
        <span class="mark" aria-hidden="true"><Layers size={16} /></span>
        <span class="name" class:visually-hidden={collapsed}>{i18n.t('projects.all')}</span>
      </button>
    </li>
    {#each ungrouped as project (project.id)}
      <li>{@render projectButton(project)}</li>
    {/each}
    {#if grouped}
      {#each groups as { server, projects: serverProjects } (server.id)}
        <li class="group-item">
          <div class="group" role="group" aria-label={server.name}>
            <div class="server" class:visually-hidden={collapsed}>
              <button
                type="button"
                class="server-name"
                aria-current={server.id === selectedServerId ? 'true' : undefined}
                onclick={() => onselectserver(server.id)}
              >
                {#if server.kind === 'local'}<HardDrive size={14} />{:else}<ServerIcon
                    size={14}
                  />{/if}
                <span>{server.name}</span>
              </button>
              {#if server.environment !== 'local'}
                <span class="environment" data-environment={server.environment}
                  >{server.environment}</span
                >
              {/if}
              <span class="state" data-state={server.connection.state}
                ><span class="dot" aria-hidden="true"></span>{i18n.t(
                  `connection.${server.connection.state}`,
                )}</span
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
                <li>{@render projectButton(project, server.environment)}</li>
              {/each}
            </ul>
          </div>
        </li>
      {/each}
    {/if}
  </ul>

  {#if selected && !collapsed}
    <section class="card" aria-labelledby="project-card-title">
      <div class="card-head">
        <Folder size={16} />
        <h3 id="project-card-title" class="card-title">{selected.name}</h3>
      </div>
      {#if selectedServer && selectedServer.environment !== 'local'}
        <div
          class="banner"
          role="status"
          aria-label={i18n.t('projects.environment')}
          data-environment={selectedServer.environment}
        >
          {selectedServer.environment} · {selectedServer.name}
        </div>
      {/if}
      <label class="debugger">
        <span>{i18n.t('projects.debugger')}</span>
        <Switch
          label={i18n.t('projects.debugger')}
          checked={debuggerOn}
          onchange={toggleDebugger}
        />
      </label>
      <div class="actions">
        <button
          type="button"
          class="icon-btn"
          aria-label={i18n.t('projects.settings')}
          title={i18n.t('projects.settings')}
          onclick={() => (dialog = 'edit')}><Settings size={16} /></button
        >
        <label class="icon-btn file-button" title={i18n.t('transfer.import')}>
          <Upload size={16} />
          <input
            class="file"
            type="file"
            accept=".jsonl,.ndjson,.log,.txt,application/x-ndjson,text/plain"
            aria-label={i18n.t('transfer.import')}
            onchange={importRequest}
          />
        </label>
        <button
          type="button"
          class="icon-btn"
          aria-label={i18n.t('projects.clear')}
          title={i18n.t('projects.clear')}
          onclick={() => (confirm = 'clear')}><Eraser size={16} /></button
        >
        <button
          type="button"
          class="icon-btn danger"
          aria-label={i18n.t('projects.remove')}
          title={i18n.t('projects.remove')}
          onclick={() => (confirm = 'remove')}><Trash size={16} /></button
        >
      </div>
    </section>
  {/if}
</nav>

{#snippet projectButton(project: Project, environment?: Environment)}
  <button
    type="button"
    class="item"
    title={collapsed ? project.name : undefined}
    aria-current={project.id === selectedId ? 'true' : undefined}
    onclick={() => onselect(project.id)}
  >
    <span class="mark initial" aria-hidden="true" data-environment={environment}
      >{project.name.slice(0, 1).toUpperCase()}</span
    >
    <span class="name" class:visually-hidden={collapsed}>{project.name}</span>
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
  {transport}
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
  .projects {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-height: 0;
  }

  .head {
    display: flex;
    gap: var(--space-1);
    align-items: center;
    padding: 0 var(--space-3) 0 var(--space-4);
  }

  .collapsed .head {
    flex-direction: column;
    padding: 0;
  }

  .title {
    flex: 1;
    margin: 0;
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .list {
    display: grid;
    gap: 1px;
    margin: 0;
    padding: 0 var(--space-2);
    list-style: none;
  }

  .list.nested {
    padding: 0;
  }

  .item {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    width: 100%;
    min-height: 32px;
    padding: var(--space-1) var(--space-3) var(--space-1) var(--space-2);
    border: 0;
    border-radius: var(--radius);
    background: none;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .collapsed .item {
    justify-content: center;
    padding: var(--space-1);
  }

  .item:hover {
    background: var(--surface-hover);
  }

  .item[aria-current='true'] {
    background: var(--accent-soft);
    box-shadow: inset 3px 0 0 var(--accent);
  }

  .item:focus-visible,
  .server-name:focus-visible,
  .file-button:focus-within {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .mark {
    display: grid;
    flex: none;
    place-items: center;
    width: 24px;
    height: 24px;
    border-radius: var(--radius);
    color: var(--text-muted);
  }

  .initial {
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .initial[data-environment='dev'] {
    border-color: var(--env-dev);
  }

  .initial[data-environment='stage'] {
    border-color: var(--env-stage);
  }

  .initial[data-environment='prod'] {
    border-color: var(--env-prod);
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .group-item {
    margin-top: var(--space-3);
  }

  .group {
    display: grid;
    gap: var(--space-1);
  }

  .server {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1) var(--space-3);
    align-items: center;
    padding: 0 var(--space-2);
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .server-name {
    display: inline-flex;
    gap: var(--space-2);
    align-items: center;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    font-weight: 600;
    cursor: pointer;
  }

  .server-name:hover span,
  .server-name[aria-current='true'] span {
    text-decoration: underline;
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

  .state {
    display: inline-flex;
    gap: var(--space-1);
    align-items: center;
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: currentcolor;
  }

  .state[data-state='online'] {
    color: var(--level-info);
  }

  .state[data-state='offline'] {
    color: var(--level-error);
  }

  .clock {
    color: var(--level-warning);
  }

  .card {
    display: grid;
    gap: var(--space-3);
    margin: var(--space-2) var(--space-3) 0;
    padding: var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius-large);
    background: var(--surface);
  }

  .card-head {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    min-width: 0;
    color: var(--text-muted);
  }

  .card-title {
    overflow: hidden;
    margin: 0;
    color: var(--text);
    font-size: var(--font-size);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .banner {
    padding: var(--space-1) var(--space-3);
    border-left: 4px solid currentcolor;
    border-radius: 0 var(--radius) var(--radius) 0;
    background: var(--surface-sunken);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .debugger {
    display: flex;
    align-items: center;
    justify-content: space-between;
    color: var(--text);
    font-size: var(--font-size-small);
  }

  .actions {
    display: flex;
    gap: var(--space-1);
    padding-top: var(--space-2);
    border-top: 1px solid var(--border);
  }

  /* The file input stays reachable from the keyboard inside its label, out of sight */
  .file-button {
    position: relative;
  }

  .file {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
  }
</style>
