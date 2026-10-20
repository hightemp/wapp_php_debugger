<script lang="ts">
  // Repeat a web request: open it again (GET) or copy it as curl, with the Xdebug trigger if asked.
  import type { Transport } from '../lib/api/client';
  import type { Message, Request } from '../lib/api/types';
  import { isDesktop, openInBrowser } from '../lib/api/wails-stream';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { asCurl, withXdebug, type LoggedGlobals } from '../lib/replay';
  import { toasts } from '../lib/toasts.svelte';
  import Switch from './Switch.svelte';

  let {
    transport,
    request,
    messages,
  }: { transport: Transport; request: Request; messages: readonly Message[] } = $props();

  let xdebug = $state(false);
  const url = $derived(request.url ? (xdebug ? withXdebug(request.url) : request.url) : null);
  const reopenable = $derived((request.method ?? 'GET').toUpperCase() === 'GET');

  // $_SERVER, $_POST and $_COOKIE as the library logged them
  async function loggedGlobals(): Promise<LoggedGlobals> {
    const found: LoggedGlobals = {};
    for (const message of messages) {
      const name = message.message.replace(/^\$_/, '').toLowerCase();
      const isGlobal = message.tags.includes('globals') || message.tags.includes('superglobals');
      if (
        isGlobal &&
        message.has_context &&
        (name === 'server' || name === 'post' || name === 'cookie')
      ) {
        const detail = await transport.getMessage(message.id);
        if (detail.context && typeof detail.context === 'object') {
          found[name] = detail.context as Record<string, unknown>;
        }
      }
    }
    return found;
  }

  async function copyCurl() {
    if (!url) {
      return;
    }
    try {
      const { command, masked } = asCurl({ method: request.method, url }, await loggedGlobals());
      await navigator.clipboard.writeText(command);
      toasts.push({ text: i18n.t(masked ? 'replay.copiedMasked' : 'replay.copied') });
    } catch (error) {
      toasts.push({ kind: 'error', text: error instanceof Error ? error.message : String(error) });
    }
  }
</script>

{#if url}
  <span class="replay">
    <button type="button" class="action" onclick={() => void copyCurl()}
      >{i18n.t('replay.curl')}</button
    >
    {#if reopenable}
      <a
        class="action"
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        onclick={(event) => {
          // Inside the desktop window the system browser opens it
          if (isDesktop()) {
            event.preventDefault();
            void openInBrowser(url);
          }
        }}>{i18n.t('replay.open')}</a
      >
    {/if}
    <span class="xdebug">
      <Switch
        label={i18n.t('replay.xdebug')}
        checked={xdebug}
        onchange={(value) => (xdebug = value)}
      />
      <span aria-hidden="true">Xdebug</span>
    </span>
  </span>
{/if}

<style>
  .replay {
    display: inline-flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    align-items: center;
  }

  .action {
    padding: 0 var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    text-decoration: none;
    cursor: pointer;
  }

  .action:hover {
    color: var(--text);
  }

  .action:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .xdebug {
    display: inline-flex;
    gap: var(--space-1);
    align-items: center;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }
</style>
