<script lang="ts">
  import { onMount } from 'svelte';
  import RequestList from './components/RequestList.svelte';
  import ThemeSwitch from './components/ThemeSwitch.svelte';
  import { createHttpTransport, type Transport } from './lib/api/client';
  import { createWailsEventSource, isDesktop, openWailsStream } from './lib/api/wails-stream';

  // В десктопе REST идёт тем же fetch через asset handler Wails, а события — через Wails Stream
  let {
    transport = createHttpTransport({
      baseUrl: '',
      createEventSource: isDesktop() ? createWailsEventSource(openWailsStream) : undefined,
    }),
  }: { transport?: Transport } = $props();

  let version = $state('');

  onMount(() => {
    transport
      .getVersion()
      .then((info) => (version = info.version))
      .catch(() => (version = ''));
  });
</script>

<div class="app">
  <header class="header">
    <h1 class="title">wapp-debugger</h1>
    {#if version}
      <span class="version">{version}</span>
    {/if}
    <span class="spacer"></span>
    <ThemeSwitch />
  </header>
  <main class="main">
    <RequestList {transport} />
  </main>
</div>

<style>
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

  .main {
    overflow: auto;
    background: var(--surface);
  }
</style>
