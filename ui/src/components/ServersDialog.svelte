<script lang="ts">
  import type { Transport } from '../lib/api/client';
  import {
    ArrowLeftRight,
    FolderSearch,
    Globe,
    HardDrive,
    KeyRound,
    Plus,
    RefreshCw,
    Server as ServerIcon,
    Trash,
  } from '../lib/icons';
  import type { Server, ServerCheck, ServerCheckStep, ServerInput } from '../lib/api/types';
  import { clockSkew } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import type { MessageKey } from '../lib/i18n/messages';
  import ConfigDialog from './ConfigDialog.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import Dialog from './Dialog.svelte';
  import DiscoverProjects from './DiscoverProjects.svelte';
  import ServerDialog from './ServerDialog.svelte';

  let {
    open,
    transport,
    onclose,
    onchange,
  }: {
    open: boolean;
    transport: Transport;
    onclose: () => void;
    /** Servers were added or removed: the projects panel reloads. */
    onchange: () => void;
  } = $props();

  type CheckState =
    | { status: 'checking' }
    | { status: 'done'; result: ServerCheck }
    | { status: 'failed'; error: string };

  let servers = $state<Server[]>([]);
  let checks = $state<Record<number, CheckState>>({});
  let adding = $state(false);
  let removing = $state<Server | undefined>(undefined);
  // Servers with the project search open
  let searching = $state<Record<number, boolean>>({});
  let configOpen = $state(false);
  // The http server whose token is being typed: one at a time
  let tokenFor = $state<number | undefined>(undefined);
  let token = $state('');
  const minTokenLength = 32;

  function toggleToken(server: Server) {
    token = '';
    tokenFor = tokenFor === server.id ? undefined : server.id;
  }

  async function saveToken(server: Server) {
    if (token.length < minTokenLength) {
      return;
    }
    try {
      await transport.updateServer(server.id, { token });
      tokenFor = undefined;
      token = '';
      checks[server.id] = { status: 'checking' };
      await check(server);
    } catch (error) {
      checks[server.id] = {
        status: 'failed',
        error: error instanceof Error ? error.message : String(error),
      };
    }
  }

  async function load() {
    servers = (await transport.listServers()).items;
  }

  $effect(() => {
    if (open) {
      checks = {};
      searching = {};
      tokenFor = undefined;
      void load().catch(() => {});
    }
  });

  // The connection state follows the stream
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

  function address(server: Server): string {
    if (server.kind === 'local') {
      return i18n.t('servers.thisMachine');
    }
    if (server.kind === 'http') {
      return server.config.base_url ?? '';
    }
    const { host = '', user, port } = server.config;
    return `${user ? `${user}@` : ''}${host}${port ? `:${port}` : ''}`;
  }

  // Steps the viewer knows; others are shown as they come
  const stepKeys: Record<string, MessageKey> = {
    config: 'step.config',
    connect: 'step.connect',
    sftp: 'step.sftp',
    local: 'step.local',
    logs_dir: 'step.logsDir',
    logs_dir_writable: 'step.logsDirWritable',
    library_config: 'step.libraryConfig',
    php: 'step.php',
    tls: 'step.tls',
    library: 'step.library',
  };

  // The host key file on the server for a key type, to compare the fingerprint with
  function hostKey(type: string | undefined): { label: string; file: string } {
    if (type?.startsWith('ecdsa-')) {
      return { label: 'ECDSA', file: 'ssh_host_ecdsa_key.pub' };
    }
    if (type === 'ssh-rsa' || type?.startsWith('rsa-')) {
      return { label: 'RSA', file: 'ssh_host_rsa_key.pub' };
    }
    return { label: 'ED25519', file: 'ssh_host_ed25519_key.pub' };
  }

  function stepName(step: ServerCheckStep): string {
    const key = stepKeys[step.name];
    const name = key ? i18n.t(key) : step.name;
    return step.project ? `${step.project} · ${name}` : name;
  }

  // A failed optional step is a warning: the check still passes
  function stepStatus(step: ServerCheckStep): 'ok' | 'warning' | 'failed' {
    if (step.ok) {
      return 'ok';
    }
    return step.optional ? 'warning' : 'failed';
  }

  const stepMarks = { ok: '✓', warning: '!', failed: '✗' } as const;

  async function check(server: Server) {
    checks[server.id] = { status: 'checking' };
    try {
      checks[server.id] = { status: 'done', result: await transport.checkServer(server.id) };
    } catch (error) {
      checks[server.id] = {
        status: 'failed',
        error: error instanceof Error ? error.message : String(error),
      };
    }
    await load().catch(() => {});
  }

  async function trust(server: Server, fingerprint: string) {
    try {
      await transport.trustHostKey(server.id, fingerprint);
    } catch (error) {
      checks[server.id] = {
        status: 'failed',
        error: error instanceof Error ? error.message : String(error),
      };
      return;
    }
    await check(server); // the next unknown key on the way (a jump host) shows up here
  }

  async function add(input: ServerInput) {
    const created = await transport.createServer(input);
    adding = false;
    await load();
    onchange();
    void check(created);
  }

  async function remove() {
    const server = removing;
    removing = undefined;
    if (server) {
      await transport.deleteServer(server.id);
      await load();
      onchange();
    }
  }
</script>

<Dialog {open} title={i18n.t('servers.title')} icon={ServerIcon} {onclose}>
  <ul class="servers">
    {#each servers as server (server.id)}
      {@const state = checks[server.id]}
      <li class="server" aria-label={server.name}>
        <div class="head">
          <span class="kind" aria-hidden="true">
            {#if server.kind === 'local'}<HardDrive
                size={16}
              />{:else if server.kind === 'ssh'}<KeyRound size={16} />{:else}<Globe
                size={16}
              />{/if}
          </span>
          <span class="name">{server.name}</span>
          <span class="badge">{server.kind} · {server.environment}</span>
          <span class="address">{address(server)}</span>
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
          {#if server.kind !== 'http'}
            <button
              type="button"
              class="btn"
              aria-expanded={searching[server.id] ?? false}
              onclick={() => (searching[server.id] = !searching[server.id])}
              ><FolderSearch size={14} />{i18n.t('discover.open')}</button
            >
          {/if}
          {#if server.kind === 'http'}
            <button
              type="button"
              class="btn"
              aria-expanded={tokenFor === server.id}
              onclick={() => toggleToken(server)}
              ><KeyRound size={14} />{i18n.t('servers.token')}</button
            >
          {/if}
          {#if server.kind !== 'local'}
            <button type="button" class="btn" onclick={() => check(server)}
              ><RefreshCw size={14} />{i18n.t('servers.check')}</button
            >
            <button type="button" class="btn btn-danger" onclick={() => (removing = server)}>
              <Trash size={14} />{i18n.t('servers.remove')}
            </button>
          {/if}
        </div>
        {#if tokenFor === server.id}
          <form
            class="token"
            onsubmit={(event) => {
              event.preventDefault();
              void saveToken(server);
            }}
          >
            <label class="token-field">
              <span>{i18n.t('server.token')}</span>
              <input
                class="input mono"
                type="password"
                autocomplete="off"
                placeholder={i18n.t('server.tokenHint')}
                bind:value={token}
              />
            </label>
            <button type="submit" class="btn btn-primary" disabled={token.length < minTokenLength}
              >{i18n.t('project.save')}</button
            >
          </form>
        {/if}
        {#if searching[server.id]}
          <DiscoverProjects {server} {transport} onadded={onchange} />
        {/if}
        {#if server.connection.error && state?.status !== 'done'}
          <p class="error">{server.connection.error}</p>
        {/if}
        {#if state?.status === 'checking'}
          <p class="note">{i18n.t('servers.checking')}</p>
        {:else if state?.status === 'failed'}
          <p class="error" role="alert">{state.error}</p>
        {:else if state?.status === 'done'}
          {#if state.result.ok}
            <p class="ok">{i18n.t('servers.checkOk')}</p>
          {/if}
          <ul class="steps">
            {#each state.result.steps as step, index (index)}
              {@const status = stepStatus(step)}
              <li data-status={status}>
                <span aria-hidden="true">{stepMarks[status]}</span>
                {stepName(step)}{#if step.detail}: <span class="detail">{step.detail}</span>{/if}
              </li>
            {/each}
          </ul>
          {#if state.result.host_key_fingerprint}
            {@const fingerprint = state.result.host_key_fingerprint}
            {@const key = hostKey(state.result.host_key_type)}
            <div class="key">
              <p>{i18n.t('servers.unknownKey')}</p>
              <p class="fingerprint">
                <span class="key-type">{key.label}</span> <code>{fingerprint}</code>
              </p>
              <p class="note">{i18n.t('servers.keyHint', { file: key.file })}</p>
              <button
                type="button"
                class="btn btn-primary"
                onclick={() => trust(server, fingerprint)}
              >
                {i18n.t('servers.trust')}
              </button>
            </div>
          {/if}
        {/if}
      </li>
    {/each}
  </ul>
  {#snippet footer()}
    <button type="button" class="btn" onclick={() => (configOpen = true)}
      ><ArrowLeftRight size={14} />{i18n.t('config.open')}</button
    >
    <button type="button" class="btn btn-primary" onclick={() => (adding = true)}
      ><Plus size={14} />{i18n.t('servers.add')}</button
    >
  {/snippet}
</Dialog>

<ConfigDialog
  open={configOpen}
  {transport}
  onclose={() => (configOpen = false)}
  onchange={() => {
    void load().catch(() => {});
    onchange();
  }}
/>
<ServerDialog open={adding} onsave={add} onclose={() => (adding = false)} />
<ConfirmDialog
  open={removing !== undefined}
  title={i18n.t('servers.removeTitle', { name: removing?.name ?? '' })}
  message={i18n.t('servers.removeMessage')}
  confirmLabel={i18n.t('servers.remove')}
  danger
  onconfirm={remove}
  oncancel={() => (removing = undefined)}
/>

<style>
  .clock {
    color: var(--level-warning);
    font-size: var(--font-size-small);
  }

  .servers {
    display: grid;
    gap: var(--space-3);
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .server {
    padding: var(--space-3) var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius-large);
    background: var(--surface);
  }

  .kind {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text-muted);
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: currentcolor;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-3);
    align-items: center;
  }

  .name {
    font-weight: 600;
  }

  .badge,
  .address,
  .note {
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .address,
  .fingerprint,
  .detail {
    font-family: var(--font-mono);
  }

  .state {
    display: inline-flex;
    gap: var(--space-1);
    align-items: center;
    margin-left: auto;
    font-size: var(--font-size-small);
  }

  .state[data-state='online'] {
    color: var(--level-info);
  }

  .state[data-state='offline'] {
    color: var(--level-error);
  }

  .error {
    margin: var(--space-2) 0 0;
    color: var(--level-error);
    font-size: var(--font-size-small);
  }

  .ok {
    margin: var(--space-2) 0 0;
    color: var(--level-info);
    font-size: var(--font-size-small);
  }

  .note {
    margin: var(--space-2) 0 0;
  }

  .steps {
    margin: var(--space-2) 0 0;
    padding: 0;
    font-size: var(--font-size-small);
    list-style: none;
  }

  .steps [data-status='failed'] {
    color: var(--level-error);
  }

  .steps [data-status='warning'] {
    color: var(--level-warning);
  }

  .key {
    margin-top: var(--space-3);
    padding: var(--space-3);
    border: 1px solid var(--level-warning);
    border-radius: var(--radius);
    background: var(--level-warning-soft);
  }

  .key p {
    margin: 0 0 var(--space-2);
  }

  .fingerprint {
    overflow-wrap: anywhere;
  }

  .key-type {
    font-weight: 600;
  }

  .token {
    display: flex;
    gap: var(--space-2);
    align-items: end;
    margin-top: var(--space-3);
  }

  .token-field {
    display: grid;
    flex: 1;
    gap: var(--space-1);
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .token-field input {
    padding: var(--space-1) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
  }
</style>
