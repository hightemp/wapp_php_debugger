<script lang="ts">
  import { onMount } from 'svelte';
  import AppearanceDialog from './components/AppearanceDialog.svelte';
  import LanguageSwitch from './components/LanguageSwitch.svelte';
  import MessageDetail from './components/MessageDetail.svelte';
  import ProjectsPanel from './components/ProjectsPanel.svelte';
  import RequestCompare from './components/RequestCompare.svelte';
  import RequestDetail from './components/RequestDetail.svelte';
  import RequestTabs from './components/RequestTabs.svelte';
  import RequestList from './components/RequestList.svelte';
  import CommandPalette, { type PaletteCommand } from './components/CommandPalette.svelte';
  import HelpDialog from './components/HelpDialog.svelte';
  import SettingsDialog from './components/SettingsDialog.svelte';
  import ShortcutsDialog from './components/ShortcutsDialog.svelte';
  import Showcase from './components/Showcase.svelte';
  import ThemeEditor from './components/ThemeEditor.svelte';
  import Toaster from './components/Toaster.svelte';
  import { emitCommand, onCommand } from './lib/commands';
  import type { HelpTopic } from './lib/help';
  import {
    Bug,
    Command,
    Help,
    List,
    MessageSquare,
    Palette,
    PanelLeftClose,
    PanelLeftOpen,
    Settings,
    Sparkles,
  } from './lib/icons';
  import { themePreferences } from './lib/theme';
  import ThemeSwitch from './components/ThemeSwitch.svelte';
  import { createHttpTransport, type Transport, type UpdateInfo } from './lib/api/client';
  import type { Environment } from './lib/api/types';
  import { appearance, saveSettings, type AppearanceStore } from './lib/settings.svelte';
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
    store = appearance,
  }: { transport?: Transport; store?: AppearanceStore } = $props();

  let version = $state('');
  let desktop = $state(false);
  let settingsOpen = $state(false);
  let appearanceOpen = $state(false);
  let themeEditorOpen = $state(false);
  // The environment of the chosen project or server: stage and prod frame the window
  let environment = $state<Environment | undefined>(undefined);
  const envFrame = $derived(
    store.settings.env_frame && (environment === 'stage' || environment === 'prod')
      ? environment
      : undefined,
  );

  function chooseLocale(locale: 'en' | 'ru') {
    i18n.set(locale);
    void saveSettings(transport, { locale }, store);
  }
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
  let shortcutsOpen = $state(false);
  let guideOpen = $state(false);
  let guideTopic = $state<HelpTopic>('overview');

  function openGuide(topic: HelpTopic = 'overview') {
    guideTopic = topic;
    guideOpen = true;
  }

  $effect(() =>
    onCommand((command) => {
      if (command.type === 'help') {
        openGuide(command.topic);
      }
    }),
  );

  // The sidebar folds to a rail of icons; the choice stays in this browser, a narrow window starts
  // folded
  const sidebarKey = 'wapp-debugger.sidebar';
  function storedSidebar(): boolean | undefined {
    try {
      const value = localStorage.getItem(sidebarKey);
      return value === null ? undefined : value === 'collapsed';
    } catch {
      return undefined;
    }
  }
  let collapsed = $state(storedSidebar() ?? window.innerWidth < 900);

  function toggleSidebar() {
    collapsed = !collapsed;
    try {
      localStorage.setItem(sidebarKey, collapsed ? 'collapsed' : 'expanded');
    } catch {
      // Private windows may refuse storage: the sidebar still folds for now
    }
  }
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
      ...store.settings.custom_themes.map((theme) => ({
        id: `theme-custom-${theme.id}`,
        label: i18n.t('palette.theme', { theme: theme.name }),
        run: () => emitCommand({ type: 'theme', theme: `custom:${theme.id}` }),
      })),
      {
        id: 'appearance',
        label: i18n.t('palette.appearance'),
        run: () => (appearanceOpen = true),
      },
      {
        id: 'theme-editor',
        label: i18n.t('palette.themeEditor'),
        run: () => (themeEditorOpen = true),
      },
      {
        id: 'showcase',
        label: i18n.t('palette.showcase'),
        run: () => navigate({ showcase: true }),
      },
      { id: 'lang-en', label: 'Language: English', run: () => chooseLocale('en') },
      { id: 'lang-ru', label: 'Язык: русский', run: () => chooseLocale('ru') },
      ...(desktop
        ? [{ id: 'settings', label: i18n.t('settings.title'), run: () => (settingsOpen = true) }]
        : []),
      { id: 'guide', label: i18n.t('help.title'), run: () => openGuide() },
      { id: 'help', label: i18n.t('keys.title'), hint: '?', run: () => (shortcutsOpen = true) },
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
        if (route.compareId !== undefined) {
          navigate({ ...scope, requestId: route.requestId });
        } else if (route.messageId !== undefined) {
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
        shortcutsOpen = true;
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

  onMount(() => store.connect(transport));

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

{#if route.showcase}
  <Showcase />
{:else}
  <div class="app" data-env-frame={envFrame}>
    <aside class="sidebar" aria-label={i18n.t('sidebar.label')} data-collapsed={collapsed}>
      <div class="brand">
        <span class="logo" aria-hidden="true"><Bug size={16} /></span>
        <div class="brand-text" class:visually-hidden={collapsed}>
          <h1 class="title">{i18n.t('app.title')}</h1>
          {#if version}
            <span class="version">{version}</span>
          {/if}
        </div>
        <button
          type="button"
          class="icon-btn fold"
          aria-label={i18n.t(collapsed ? 'sidebar.expand' : 'sidebar.collapse')}
          title={i18n.t(collapsed ? 'sidebar.expand' : 'sidebar.collapse')}
          onclick={toggleSidebar}
        >
          {#if collapsed}<PanelLeftOpen size={16} />{:else}<PanelLeftClose size={16} />{/if}
        </button>
      </div>
      {#if update?.url}
        {@const url = update.url}
        {@const label = i18n.t('update.available', { version: update.latest ?? '' })}
        <a
          class="update"
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          title={label}
          onclick={(event) => {
            // Inside the desktop window the system browser opens it
            if (isDesktop()) {
              event.preventDefault();
              void openInBrowser(url);
            }
          }}><Sparkles size={14} /><span class:visually-hidden={collapsed}>{label}</span></a
        >
      {/if}
      <div class="nav">
        <ProjectsPanel
          {transport}
          {collapsed}
          selectedId={route.projectId}
          selectedServerId={route.serverId}
          onopenrequest={(projectId, requestId) => navigate({ projectId, requestId })}
          onselect={(id) => navigate({ projectId: id })}
          onselectserver={(id) => navigate({ serverId: id })}
          onenvironment={(value) => (environment = value)}
        />
      </div>
      <div class="footer">
        <div class="tools">
          <button
            type="button"
            class="icon-btn"
            aria-label={i18n.t('help.open')}
            title={i18n.t('help.open')}
            onclick={() => openGuide()}><Help size={16} /></button
          >
          <button
            type="button"
            class="icon-btn"
            aria-label={i18n.t('palette.title')}
            title={`${i18n.t('palette.title')} (Ctrl+K)`}
            onclick={() => void openPalette()}><Command size={16} /></button
          >
          <button
            type="button"
            class="icon-btn"
            aria-label={i18n.t('appearance.open')}
            title={i18n.t('appearance.open')}
            onclick={() => (appearanceOpen = true)}><Palette size={16} /></button
          >
          {#if desktop}
            <button
              type="button"
              class="icon-btn"
              aria-label={i18n.t('settings.title')}
              title={i18n.t('settings.title')}
              onclick={() => (settingsOpen = true)}><Settings size={16} /></button
            >
          {/if}
        </div>
        {#if !collapsed}
          <div class="switches">
            <ThemeSwitch {transport} {store} compact />
            <LanguageSwitch onchange={chooseLocale} compact />
          </div>
        {/if}
      </div>
    </aside>
    <main class="panes">
      <div class="pane feed">
        <RequestList
          {transport}
          projectId={route.projectId}
          serverId={route.serverId}
          selectedId={route.requestId}
          onselect={(id) => navigate({ ...scope, requestId: id })}
        />
      </div>
      {#if route.requestId !== undefined && route.compareId !== undefined}
        {@const requestId = route.requestId}
        <div class="pane wide">
          <RequestTabs
            {transport}
            currentId={requestId}
            onselect={(id) => navigate({ ...scope, requestId: id })}
            oncompare={(id) => navigate({ ...scope, requestId, compareId: id })}
          />
          <RequestCompare
            {transport}
            {requestId}
            otherId={route.compareId}
            onclose={() => navigate({ ...scope, requestId })}
          />
        </div>
      {:else if route.requestId !== undefined}
        {@const requestId = route.requestId}
        <div class="pane">
          <RequestTabs
            {transport}
            currentId={requestId}
            onselect={(id) => navigate({ ...scope, requestId: id })}
            oncompare={(id) => navigate({ ...scope, requestId, compareId: id })}
          />
          <RequestDetail
            {transport}
            requestId={route.requestId}
            selectedMessageId={route.messageId}
            onselectmessage={(id) =>
              navigate({ ...scope, requestId: route.requestId, messageId: id })}
            onselectrequest={(id) => navigate({ ...scope, requestId: id })}
          />
        </div>
        <div class="pane">
          {#if route.messageId !== undefined}
            <MessageDetail {transport} messageId={route.messageId} />
          {:else}
            <div class="empty">
              <span class="empty-icon"><MessageSquare size={22} /></span>
              <p class="empty-title">{i18n.t('message.empty')}</p>
              <p class="empty-text">{i18n.t('message.emptyText')}</p>
            </div>
          {/if}
        </div>
      {:else}
        <div class="pane wide">
          <div class="empty">
            <span class="empty-icon"><List size={22} /></span>
            <p class="empty-title">{i18n.t('detail.emptyTitle')}</p>
            <p class="empty-text">{i18n.t('detail.emptyText')}</p>
          </div>
        </div>
      {/if}
    </main>
  </div>
{/if}

<svelte:window onkeydown={onKeydown} />
<CommandPalette
  open={paletteOpen}
  {commands}
  onfilter={(text) => emitCommand({ type: 'filter', text })}
  onclose={() => (paletteOpen = false)}
/>
<ShortcutsDialog open={shortcutsOpen} onclose={() => (shortcutsOpen = false)} />
<HelpDialog
  open={guideOpen}
  topic={guideTopic}
  onclose={() => (guideOpen = false)}
  onshortcuts={() => {
    guideOpen = false;
    shortcutsOpen = true;
  }}
/>
<AppearanceDialog
  open={appearanceOpen}
  {transport}
  {store}
  onclose={() => (appearanceOpen = false)}
  oneditthemes={() => {
    appearanceOpen = false;
    themeEditorOpen = true;
  }}
/>
<ThemeEditor open={themeEditorOpen} {transport} {store} onclose={() => (themeEditorOpen = false)} />
<Toaster />

{#if desktop}
  <SettingsDialog open={settingsOpen} {transport} onclose={() => (settingsOpen = false)} />
{/if}

<style>
  .app {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    height: 100vh;
  }

  /* Over everything and never in the way: the frame takes no clicks */
  .app[data-env-frame]::after {
    content: '';
    position: fixed;
    inset: 0;
    z-index: 20;
    border: 3px solid var(--env-frame);
    pointer-events: none;
  }

  .app[data-env-frame='stage'] {
    --env-frame: var(--env-stage);
  }

  .app[data-env-frame='prod'] {
    --env-frame: var(--env-prod);
  }

  .sidebar {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    width: var(--sidebar-width);
    min-height: 0;
    padding: var(--space-3) 0;
    border-right: 1px solid var(--border);
    background: var(--bg);
  }

  .sidebar[data-collapsed='true'] {
    width: var(--sidebar-rail);
    align-items: center;
  }

  .brand {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    padding: 0 var(--space-3) 0 var(--space-4);
  }

  [data-collapsed='true'] .brand {
    flex-direction: column;
    gap: var(--space-2);
    padding: 0;
  }

  .logo {
    display: grid;
    flex: none;
    place-items: center;
    width: 28px;
    height: 28px;
    border-radius: var(--radius);
    background: var(--accent);
    color: var(--accent-contrast);
  }

  .brand-text {
    display: grid;
    flex: 1;
    min-width: 0;
  }

  .title {
    margin: 0;
    font-size: 14px;
    font-weight: 700;
  }

  .version {
    overflow: hidden;
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: 11px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .update {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    margin: 0 var(--space-3);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius);
    background: var(--level-warning-soft);
    color: var(--level-warning);
    font-size: var(--font-size-small);
    text-decoration: none;
  }

  [data-collapsed='true'] .update {
    margin: 0;
    padding: var(--space-2);
  }

  .nav {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }

  [data-collapsed='true'] .nav {
    width: 100%;
  }

  .footer {
    display: grid;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-3) 0;
    border-top: 1px solid var(--border);
  }

  [data-collapsed='true'] .footer {
    padding: var(--space-3) 0 0;
  }

  .tools {
    display: flex;
    gap: var(--space-1);
  }

  [data-collapsed='true'] .tools {
    flex-direction: column;
  }

  .switches {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    justify-content: space-between;
  }

  .panes {
    display: grid;
    grid-template-columns: minmax(340px, 1.15fr) minmax(320px, 1fr) minmax(320px, 1fr);
    min-width: 0;
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

  /* The comparison or the empty state take the places of the request and the message */
  .pane.wide {
    grid-column: span 2;
  }

  .feed {
    display: flex;
    flex-direction: column;
  }

  /* Narrower: the feed on the left, the request over its message on the right */
  @media (max-width: 1360px) {
    .panes {
      grid-template-columns: minmax(300px, 0.9fr) minmax(0, 1.1fr);
      grid-template-rows: minmax(0, 1.2fr) minmax(0, 1fr);
    }

    .feed {
      grid-row: span 2;
    }

    .pane.wide {
      grid-row: span 2;
      grid-column: auto;
    }

    .pane:not(.feed) {
      border-right: 0;
    }

    .pane:not(.feed, .wide):nth-child(2) {
      border-bottom: 1px solid var(--border);
    }
  }

  @media (max-width: 900px) {
    .panes {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: none;
      grid-auto-rows: minmax(280px, auto);
      overflow: auto;
    }

    .feed,
    .pane.wide {
      grid-row: auto;
    }

    .pane {
      border-bottom: 1px solid var(--border);
    }
  }
</style>
