<script lang="ts">
  import { onMount } from 'svelte';
  import LanguageSwitch from './components/LanguageSwitch.svelte';
  import MessageDetail from './components/MessageDetail.svelte';
  import ProjectsPanel from './components/ProjectsPanel.svelte';
  import RequestDetail from './components/RequestDetail.svelte';
  import RequestList from './components/RequestList.svelte';
  import SettingsDialog from './components/SettingsDialog.svelte';
  import ThemeSwitch from './components/ThemeSwitch.svelte';
  import { createHttpTransport, type Transport } from './lib/api/client';
  import { createWailsEventSource, isDesktop, openWailsStream } from './lib/api/wails-stream';
  import { i18n } from './lib/i18n/locale.svelte';
  import { formatRoute, parseRoute, type Route } from './lib/route';

  // In the desktop app REST uses the same fetch through the Wails asset handler, events use a Wails stream
  let {
    transport = createHttpTransport({
      baseUrl: '',
      createEventSource: isDesktop() ? createWailsEventSource(openWailsStream) : undefined,
    }),
  }: { transport?: Transport } = $props();

  let version = $state('');
  let desktop = $state(false);
  let settingsOpen = $state(false);
  let route = $state<Route>(parseRoute(window.location.hash));

  // The feed the selection belongs to: a project, a server or everything
  const scope = $derived<Route>(
    route.projectId !== undefined ? { projectId: route.projectId } : { serverId: route.serverId },
  );

  function navigate(next: Route) {
    route = next;
    const hash = formatRoute(next);
    if (window.location.hash !== hash) {
      window.location.hash = hash;
    }
  }

  onMount(() => {
    transport
      .getVersion()
      .then((info) => {
        version = info.version;
        desktop = info.app === 'desktop';
      })
      .catch(() => (version = ''));
    const follow = () => (route = parseRoute(window.location.hash));
    window.addEventListener('hashchange', follow);
    return () => window.removeEventListener('hashchange', follow);
  });
</script>

<div class="app">
  <header class="header">
    <h1 class="title">{i18n.t('app.title')}</h1>
    {#if version}
      <span class="version">{version}</span>
    {/if}
    <span class="spacer"></span>
    <LanguageSwitch />
    <ThemeSwitch />
    {#if desktop}
      <button type="button" class="settings" onclick={() => (settingsOpen = true)}
        >{i18n.t('settings.title')}</button
      >
    {/if}
  </header>
  <main class="panes">
    <div class="pane feed">
      <ProjectsPanel
        {transport}
        selectedId={route.projectId}
        selectedServerId={route.serverId}
        onselect={(id) => navigate({ projectId: id })}
        onselectserver={(id) => navigate({ serverId: id })}
      />
      <RequestList
        {transport}
        projectId={route.projectId}
        serverId={route.serverId}
        selectedId={route.requestId}
        onselect={(id) => navigate({ ...scope, requestId: id })}
      />
    </div>
    <div class="pane">
      {#if route.requestId !== undefined}
        <RequestDetail
          {transport}
          requestId={route.requestId}
          selectedMessageId={route.messageId}
          onselectmessage={(id) =>
            navigate({ ...scope, requestId: route.requestId, messageId: id })}
          onselectrequest={(id) => navigate({ ...scope, requestId: id })}
        />
      {:else}
        <p class="hint">{i18n.t('detail.empty')}</p>
      {/if}
    </div>
    <div class="pane">
      {#if route.messageId !== undefined}
        <MessageDetail {transport} messageId={route.messageId} />
      {:else if route.requestId !== undefined}
        <p class="hint">{i18n.t('message.empty')}</p>
      {/if}
    </div>
  </main>
</div>

{#if desktop}
  <SettingsDialog open={settingsOpen} {transport} onclose={() => (settingsOpen = false)} />
{/if}

<style>
  .settings {
    padding: 2px var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .settings:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .app {
    display: grid;
    grid-template-rows: auto 1fr;
    height: 100vh;
  }

  .header {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }

  .title {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }

  .version {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  .spacer {
    flex: 1;
  }

  .panes {
    display: grid;
    grid-template-columns: minmax(320px, 1.1fr) minmax(320px, 1fr) minmax(320px, 1fr);
    min-height: 0;
    background: var(--surface);
  }

  .pane {
    min-width: 0;
    overflow: auto;
    border-right: 1px solid var(--border);
  }

  .pane:last-child {
    border-right: 0;
  }

  .hint {
    padding: var(--space-6) var(--space-4);
    color: var(--text-muted);
  }

  @media (max-width: 1100px) {
    .panes {
      grid-template-columns: 1fr;
      grid-auto-rows: minmax(240px, auto);
    }

    .pane {
      border-right: 0;
      border-bottom: 1px solid var(--border);
    }
  }
</style>
