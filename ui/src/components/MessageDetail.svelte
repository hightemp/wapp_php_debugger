<script lang="ts">
  import { ApiError, type Transport } from '../lib/api/client';
  import type { MessageDetail } from '../lib/api/types';
  import { formatDuration, formatTime } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import CodePreview from './CodePreview.svelte';
  import JsonTree from './JsonTree.svelte';

  type Frame = NonNullable<MessageDetail['source']>;

  let { transport, messageId }: { transport: Transport; messageId: number } = $props();

  let message = $state<MessageDetail | null>(null);
  let status = $state<'loading' | 'ready' | 'not-found' | 'error'>('loading');
  let errorMessage = $state('');
  // The code under the place of the message and under the frames the user opened
  let showCode = $state(false);
  let openFrames = $state<number[]>([]);

  function toggleFrame(index: number) {
    openFrames = openFrames.includes(index)
      ? openFrames.filter((item) => item !== index)
      : [...openFrames, index];
  }

  $effect(() => {
    void messageId;
    showCode = false;
    openFrames = [];
  });

  $effect(() => {
    const id = messageId;
    status = 'loading';
    transport
      .getMessage(id)
      .then((loaded) => {
        message = loaded;
        status = 'ready';
      })
      .catch((error: unknown) => {
        message = null;
        status = error instanceof ApiError && error.status === 404 ? 'not-found' : 'error';
        errorMessage = error instanceof Error ? error.message : String(error);
      });
  });

  function location(frame: Frame): string {
    return frame.file ? `${frame.file}:${frame.line ?? ''}` : '';
  }

  function callee(frame: Frame): string {
    return `${frame.class ?? ''}${frame.call_type ?? ''}${frame.function ?? ''}` || location(frame);
  }
</script>

<section class="message" aria-label={i18n.t('message.title')} aria-live="polite">
  {#if status === 'not-found'}
    <p class="state">{i18n.t('message.notFound')}</p>
  {:else if status === 'error'}
    <p class="state error" role="alert">{errorMessage}</p>
  {:else if !message}
    <p class="state">{i18n.t('detail.loading')}</p>
  {:else}
    <header class="header" data-level={message.level}>
      <span class="level">{i18n.t(`levelName.${message.level}`)}</span>
      <span class="time">{formatTime(message.ts)}</span>
      {#if message.type === 'timer'}
        <span class="time">{formatDuration(message.duration_ms, i18n.locale)}</span>
      {/if}
      {#each message.tags as tag (tag)}<span class="tag">#{tag}</span>{/each}
    </header>

    {#if message.exception_class}
      <p class="exception">
        <span>{message.exception_class}</span>
        {#if message.code}<span class="code">{i18n.t('message.code', { code: message.code })}</span
          >{/if}
      </p>
    {/if}
    <pre class="text">{message.message}</pre>

    {#if message.source?.file}
      <p class="source">
        {#if message.source.editor_url}
          <a href={message.source.editor_url}>{location(message.source)}</a>
        {:else}
          <span>{location(message.source)}</span>
        {/if}
        <button
          type="button"
          class="code-toggle"
          aria-expanded={showCode}
          onclick={() => (showCode = !showCode)}>{i18n.t('code.show')}</button
        >
      </p>
      {#if showCode}
        <CodePreview {transport} {messageId} />
      {/if}
    {/if}

    <h3 class="subtitle">{i18n.t('message.context')}</h3>
    {#if message.context}
      <div class="block"><JsonTree value={message.context} /></div>
    {:else}
      <p class="muted">{i18n.t('message.noContext')}</p>
    {/if}

    {#if message.previous && message.previous.length > 0}
      <h3 class="subtitle">{i18n.t('message.previous')}</h3>
      <ul class="frames">
        {#each message.previous as previous, index (index)}
          <li>
            <span class="callee">{previous.exception_class}</span>
            <span class="muted">{previous.message}</span>
          </li>
        {/each}
      </ul>
    {/if}

    {#if message.trace && message.trace.length > 0}
      <h3 class="subtitle">{i18n.t('message.trace')}</h3>
      <ol class="frames">
        {#each message.trace as frame, index (index)}
          <li>
            {#if frame.editor_url}
              <a class="callee" href={frame.editor_url}>{callee(frame)}</a>
            {:else}
              <span class="callee">{callee(frame)}</span>
            {/if}
            <span class="muted">{location(frame)}</span>
            {#if frame.file}
              <button
                type="button"
                class="code-toggle"
                aria-label={i18n.t('code.frame')}
                aria-expanded={openFrames.includes(index)}
                onclick={() => toggleFrame(index)}>{i18n.t('code.short')}</button
              >
            {/if}
            {#if openFrames.includes(index)}
              <CodePreview {transport} {messageId} frame={index} />
            {/if}
          </li>
        {/each}
      </ol>
    {/if}
  {/if}
</section>

<style>
  .code-toggle {
    margin-left: var(--space-2);
    padding: 0 var(--space-2);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .code-toggle[aria-expanded='true'] {
    border-color: var(--accent);
    color: var(--text);
  }

  .code-toggle:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .message {
    padding: var(--space-3) var(--space-4);
    overflow: auto;
  }

  .header {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    align-items: baseline;
    font-size: var(--font-size-small);
  }

  .level {
    color: var(--level-message);
    font-family: var(--font-mono);
    font-weight: 600;
  }

  [data-level='error'] .level {
    color: var(--level-error);
  }

  [data-level='warning'] .level {
    color: var(--level-warning);
  }

  [data-level='info'] .level {
    color: var(--level-info);
  }

  .time,
  .tag,
  .muted {
    color: var(--text-muted);
  }

  .exception {
    display: flex;
    gap: var(--space-3);
    margin: var(--space-3) 0 0;
    font-family: var(--font-mono);
    font-weight: 600;
  }

  .code {
    color: var(--text-muted);
    font-weight: 400;
  }

  .text {
    margin: var(--space-3) 0;
    overflow-wrap: anywhere;
    font-family: var(--font-mono);
    white-space: pre-wrap;
  }

  .source {
    margin: 0 0 var(--space-3);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  a {
    color: var(--accent);
  }

  .subtitle {
    margin: var(--space-4) 0 var(--space-2);
    color: var(--text-muted);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .block {
    padding: var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
  }

  .frames {
    margin: 0;
    padding-left: var(--space-6);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  .frames li {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    padding: var(--space-1) 0;
  }

  .state {
    color: var(--text-muted);
  }

  .error {
    color: var(--level-error);
  }
</style>
