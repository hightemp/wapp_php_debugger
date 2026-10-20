<script lang="ts">
  import { onMount } from 'svelte';
  import LanguageSwitch from './components/LanguageSwitch.svelte';
  import MessageDetail from './components/MessageDetail.svelte';
  import ProjectsPanel from './components/ProjectsPanel.svelte';
  import RequestDetail from './components/RequestDetail.svelte';
  import RequestList from './components/RequestList.svelte';
  import CommandPalette, { type PaletteCommand } from './components/CommandPalette.svelte';
  import SettingsDialog from './components/SettingsDialog.svelte';
  import ShortcutsDialog from './components/ShortcutsDialog.svelte';
  import { emitCommand } from './lib/commands';
  import { themePreferences } from './lib/theme';
  import ThemeSwitch from './components/ThemeSwitch.svelte';
  import { createHttpTransport, type Transport, type UpdateInfo } from './lib/api/client';
  import {
    createWailsEventSource,
    isDesktop,
    openInBrowser,
    openWailsStream,
  } from './lib/api/wails-stream';
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
  let update = $state<UpdateInfo | undefined>(undefined);

  // A newer version: asked now and every hour (the viewer itself asks GitHub once a day)
  function checkUpdates() {
    transport
      .getUpdates()
      .then((info) => (update = info.newer ? info : undefined))
      .catch(() => (update = undefined));
  }
  let route = $state<Route>(parseRoute(window.location.hash));

  // The feed the selection belongs to: a project, a server or everything
  const scope = $derived<Route>(
    route.projectId !== undefined ? { projectId: route.projectId } : { serverId: route.serverId },
  );

  // System notifications of the desktop app speak the language of the UI
  $effect(() => {
    const locale = i18n.locale;
    if (!desktop) {
      return;
    }
    transport
      .getDesktopSettings()
      .then((settings) =>
        settings.locale === locale
          ? undefined
          : transport.updateDesktopSettings({ ...settings, locale }),
      )
      .catch(() => {});
  });

  let paletteOpen = $state(false);
  let helpOpen = $state(false);
  let commands = $state<PaletteCommand[]>([]);

  // Commands of the palette: projects and servers as they are now, then the actions
  async function openPalette() {
    const [projects, servers] = await Promise.all([
      transport.listProjects().catch(() => ({ items: [] })),
      transport.listServers().catch(() => ({ items: [] })),
    ]);
    commands = [
      { id: 'all', label: i18n.t('palette.all'), run: () => navigate({}) },
      ...projects.items.map((project) => ({
        id: `project-${project.id}`,
        label: i18n.t('palette.project', { name: project.name }),
        run: () => navigate({ projectId: project.id }),
      })),
      ...servers.items
        .filter((server) => server.kind !== 'local')
        .map((server) => ({
          id: `server-${server.id}`,
          label: i18n.t('palette.server', { name: server.name }),
          hint: server.environment,
          run: () => navigate({ serverId: server.id }),
        })),
      {
        id: 'pinned',
        label: i18n.t('palette.pinned'),
        run: () => emitCommand({ type: 'filter', text: 'pinned:yes' }),
      },
      {
        id: 'errors',
        label: i18n.t('palette.errors'),
        run: () => emitCommand({ type: 'filter', text: 'level:error' }),
      },
      {
        id: 'servers',
        label: i18n.t('palette.servers'),
        run: () => emitCommand({ type: 'open-servers' }),
      },
      {
        id: 'add-project',
        label: i18n.t('palette.addProject'),
        run: () => emitCommand({ type: 'add-project' }),
      },
      ...themePreferences.map((theme) => ({
        id: `theme-${theme}`,
        label: i18n.t('palette.theme', { theme: i18n.t(`theme.${theme}`) }),
        run: () => emitCommand({ type: 'theme', theme }),
      })),
      { id: 'lang-en', label: 'Language: English', run: () => i18n.set('en') },
      { id: 'lang-ru', label: 'Язык: русский', run: () => i18n.set('ru') },
      ...(desktop
        ? [{ id: 'settings', label: i18n.t('settings.title'), run: () => (settingsOpen = true) }]
        : []),
      { id: 'help', label: i18n.t('keys.title'), hint: '?', run: () => (helpOpen = true) },
    ];
    paletteOpen = true;
  }

  // The rows of a list in the page, to move through them with the keyboard
  function step(
    selector: string,
    attribute: string,
    current: number | undefined,
    delta: number,
  ): number | undefined {
    const ids = [...document.querySelectorAll<HTMLElement>(selector)].map((row) =>
      Number(row.getAttribute(attribute)),
    );
    if (ids.length === 0) {
      return undefined;
    }
    const index = current === undefined ? -1 : ids.indexOf(current);
    const next = ids[Math.min(ids.length - 1, Math.max(0, index + delta))];
    document
      .querySelector(`${selector}[${attribute}="${next}"]`)
      ?.scrollIntoView?.({ block: 'nearest' });
    return next;
  }

  let pendingG = 0;

  function onKeydown(event: KeyboardEvent) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      void openPalette();
      return;
    }
    const target = event.target as HTMLElement | null;
    const typing = target?.closest('input, textarea, select, [contenteditable="true"]') !== null;
    // Dialogs answer their own keys; typing is left alone
    if (
      typing ||
      document.querySelector('[aria-modal="true"]') ||
      event.ctrlKey ||
      event.metaKey ||
      event.altKey
    ) {
      return;
    }
    const now = Date.now();
    if (event.key === 'p' && now - pendingG < 1000) {
      emitCommand({ type: 'focus-projects' });
      pendingG = 0;
      return;
    }
    pendingG = event.key === 'g' ? now : 0;
    switch (event.key) {
      case 'j':
      case 'k': {
        const next = step(
          'li.item[data-request-id]',
          'data-request-id',
          route.requestId,
          event.key === 'j' ? 1 : -1,
        );
        if (next !== undefined) {
          navigate({ ...scope, requestId: next });
        }
        break;
      }
      case 'J':
      case 'K': {
        if (route.requestId === undefined) {
          break;
        }
        const next = step(
          'li.message[data-message-id]',
          'data-message-id',
          route.messageId,
          event.key === 'J' ? 1 : -1,
        );
        if (next !== undefined) {
          navigate({ ...scope, requestId: route.requestId, messageId: next });
        }
        break;
      }
      case 'Escape':
        if (route.messageId !== undefined) {
          navigate({ ...scope, requestId: route.requestId });
        } else if (route.requestId !== undefined) {
          navigate(scope);
        }
        break;
      case '/':
        event.preventDefault();
        emitCommand({ type: 'focus-filter' });
        break;
      case '?':
        helpOpen = true;
        break;
      default:
        return;
    }
  }

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
    checkUpdates();
    const updates = setInterval(checkUpdates, 60 * 60 * 1000);
    const follow = () => (route = parseRoute(window.location.hash));
    window.addEventListener('hashchange', follow);
    return () => {
      clearInterval(updates);
      window.removeEventListener('hashchange', follow);
    };
  });
</script>

<div class="app">
  <header class="header">
    <h1 class="title">{i18n.t('app.title')}</h1>
    {#if version}
      <span class="version">{version}</span>
    {/if}
    {#if update?.url}
      {@const url = update.url}
      <a
        class="update"
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        onclick={(event) => {
          // Inside the desktop window the system browser opens it
          if (isDesktop()) {
            event.preventDefault();
            void openInBrowser(url);
          }
        }}>{i18n.t('update.available', { version: update.latest ?? '' })}</a
      >
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

<svelte:window onkeydown={onKeydown} />
<CommandPalette
  open={paletteOpen}
  {commands}
  onfilter={(text) => emitCommand({ type: 'filter', text })}
  onclose={() => (paletteOpen = false)}
/>
<ShortcutsDialog open={helpOpen} onclose={() => (helpOpen = false)} />

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

  .update {
    padding: 0 var(--space-2);
    border-radius: var(--radius);
    background: var(--level-warning-soft);
    color: var(--level-warning);
    font-size: var(--font-size-small);
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
