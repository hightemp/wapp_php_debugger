<script lang="ts">
  // Finds projects on a server by their library config and adds the chosen ones.
  import type { Transport } from '../lib/api/client';
  import type { DiscoveredProject, Server } from '../lib/api/types';
  import { i18n } from '../lib/i18n/locale.svelte';

  let {
    server,
    transport,
    onadded,
  }: {
    server: Server;
    transport: Transport;
    /** A project was added: the projects panel reloads. */
    onadded: () => void;
  } = $props();

  let roots = $state('/var/www');
  let searching = $state(false);
  let failure = $state('');
  let found = $state<DiscoveredProject[] | undefined>(undefined);
  let truncated = $state(false);
  let adding = $state<string | undefined>(undefined);

  const rootList = $derived(
    roots
      .split(/[,\n]/)
      .map((root) => root.trim())
      .filter((root) => root !== ''),
  );

  async function search(event: SubmitEvent) {
    event.preventDefault();
    if (rootList.length === 0 || searching) {
      return;
    }
    searching = true;
    failure = '';
    try {
      const result = await transport.discoverProjects(server.id, rootList);
      found = result.items;
      truncated = result.truncated;
    } catch (error) {
      failure = error instanceof Error ? error.message : String(error);
    } finally {
      searching = false;
    }
  }

  async function add(project: DiscoveredProject) {
    adding = project.logs_dir;
    failure = '';
    try {
      const created = await transport.createProject({
        server_id: server.id,
        name: project.name,
        root_path: project.root_path,
        logs_dir: project.logs_dir,
        config_path: project.config_path,
      });
      found = found?.map((item) =>
        item.logs_dir === project.logs_dir ? { ...item, project_id: created.id } : item,
      );
      onadded();
    } catch (error) {
      failure = error instanceof Error ? error.message : String(error);
    } finally {
      adding = undefined;
    }
  }
</script>

<form class="search" onsubmit={search}>
  <label class="field">
    <span>{i18n.t('discover.roots')}</span>
    <input
      class="input mono"
      bind:value={roots}
      placeholder="/var/www, /home/deploy"
      autocomplete="off"
    />
  </label>
  <button type="submit" class="btn" disabled={rootList.length === 0 || searching}
    >{i18n.t('discover.search')}</button
  >
</form>
<p class="note">{i18n.t('discover.hint')}</p>
{#if searching}
  <p class="note">{i18n.t('discover.searching')}</p>
{/if}
{#if failure}
  <p class="error" role="alert">{failure}</p>
{/if}
{#if found}
  {#if found.length === 0}
    <p class="note">{i18n.t('discover.none')}</p>
  {:else}
    <ul class="found" aria-label={i18n.t('discover.found')}>
      {#each found as project (project.config_path)}
        <li aria-label={project.name}>
          <div class="line">
            <span class="name">{project.name}</span>
            <span class="state">{i18n.t(project.enabled ? 'discover.on' : 'discover.off')}</span>
            {#if project.project_id !== undefined && project.project_id !== null}
              <span class="added">{i18n.t('discover.added')}</span>
            {:else}
              <button
                type="button"
                class="btn btn-primary"
                disabled={adding !== undefined}
                onclick={() => add(project)}>{i18n.t('discover.add')}</button
              >
            {/if}
          </div>
          <div class="path">{project.logs_dir}</div>
          {#if project.problem}
            <div class="problem">{i18n.t('discover.problem', { problem: project.problem })}</div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
  {#if truncated}
    <p class="note">{i18n.t('discover.truncated')}</p>
  {/if}
{/if}

<style>
  .search {
    display: flex;
    gap: var(--space-2);
    align-items: end;
    margin-top: var(--space-3);
  }

  .field {
    display: grid;
    flex: 1;
    gap: var(--space-1);
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .note,
  .path,
  .state {
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .note,
  .error {
    margin: var(--space-2) 0 0;
  }

  .error,
  .problem {
    color: var(--level-error);
    font-size: var(--font-size-small);
  }

  .found {
    display: grid;
    gap: var(--space-2);
    margin: var(--space-2) 0 0;
    padding: 0;
    list-style: none;
  }

  .line {
    display: flex;
    gap: var(--space-3);
    align-items: center;
  }

  .name {
    font-weight: 600;
  }

  .path {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }

  .added {
    margin-left: auto;
    color: var(--level-info);
    font-size: var(--font-size-small);
  }

  .line .btn {
    margin-left: auto;
  }
</style>
