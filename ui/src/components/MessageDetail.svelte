<script lang="ts">
  import { ApiError, type Transport } from '../lib/api/client';
  import type { MessageDetail } from '../lib/api/types';
  import { formatDuration, formatTime } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import {
    Braces,
    Bug,
    CalendarClock,
    CircleX,
    Clock,
    Code,
    FileCode,
    Info,
    Layers,
    MessageSquare,
    TriangleAlert,
  } from '../lib/icons';
  import CodePreview from './CodePreview.svelte';
  import JsonTree from './JsonTree.svelte';

  type Frame = NonNullable<MessageDetail['source']>;

  let { transport, messageId }: { transport: Transport; messageId: number } = $props();

  const levelIcons = {
    error: CircleX,
    warning: TriangleAlert,
    info: Info,
    message: MessageSquare,
    debug: Bug,
  } as const;

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
    {@const LevelIcon = levelIcons[message.level]}
    <header class="header" data-level={message.level}>
      <span class="level"><LevelIcon size={13} />{i18n.t(`levelName.${message.level}`)}</span>
      <span class="time"><CalendarClock size={12} />{formatTime(message.ts)}</span>
      {#if message.type === 'timer'}
        <span class="time"
          ><Clock size={12} />{formatDuration(message.duration_ms, i18n.locale)}</span
        >
      {/if}
      {#each message.tags as tag (tag)}<span class="tag">#{tag}</span>{/each}
    </header>

    {#if message.exception_class}
      <p class="exception">
        <CircleX size={14} />
        <span>{message.exception_class}</span>
        {#if message.code}<span class="code">{i18n.t('message.code', { code: message.code })}</span
          >{/if}
      </p>
    {/if}
    <pre class="text">{message.message}</pre>

    {#if message.source?.file}
      <p class="source">
        <FileCode size={13} />
        {#if message.source.editor_url}
          <a href={message.source.editor_url}>{location(message.source)}</a>
        {:else}
          <span>{location(message.source)}</span>
        {/if}
        <button
          type="button"
          class="chip code-toggle"
          aria-expanded={showCode}
          onclick={() => (showCode = !showCode)}><Code size={12} />{i18n.t('code.show')}</button
        >
      </p>
      {#if showCode}
        <CodePreview {transport} {messageId} />
      {/if}
    {/if}

    <h3 class="subtitle"><Braces size={13} />{i18n.t('message.context')}</h3>
    {#if message.context}
      <div class="block"><JsonTree value={message.context} searchable /></div>
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
      <h3 class="subtitle"><Layers size={13} />{i18n.t('message.trace')}</h3>
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
                class="chip code-toggle"
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
  .message {
    padding: var(--space-4);
    overflow: auto;
  }

  .header {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-4);
    align-items: center;
    font-size: var(--font-size-small);
  }

  .level {
    display: inline-flex;
    gap: var(--space-1);
    align-items: center;
    padding: 1px var(--space-2);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--level-message);
    font-family: var(--font-mono);
    font-weight: 600;
  }

  [data-level='error'] .level {
    background: var(--level-error-soft);
    color: var(--level-error);
  }

  [data-level='warning'] .level {
    background: var(--level-warning-soft);
    color: var(--level-warning);
  }

  [data-level='info'] .level {
    background: var(--level-info-soft);
    color: var(--level-info);
  }

  [data-level='debug'] .level {
    color: var(--level-debug);
  }

  .time {
    display: inline-flex;
    gap: var(--space-1);
    align-items: center;
  }

  .time,
  .tag,
  .muted {
    color: var(--text-muted);
  }

  .exception {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    margin: var(--space-4) 0 0;
    color: var(--level-error);
    font-family: var(--font-mono);
    font-weight: 600;
  }

  .code {
    color: var(--text-muted);
    font-weight: 400;
  }

  .text {
    margin: var(--space-3) 0;
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    overflow-wrap: anywhere;
    font-family: var(--font-mono);
    white-space: pre-wrap;
  }

  .source {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    align-items: center;
    margin: 0 0 var(--space-3);
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  .code-toggle {
    margin-left: var(--space-1);
  }

  .code-toggle[aria-expanded='true'] {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--text);
  }

  a {
    color: var(--accent);
  }

  .subtitle {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    margin: var(--space-5) 0 var(--space-2);
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
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
    gap: var(--space-1) var(--space-3);
    align-items: center;
    padding: var(--space-1) 0;
    border-bottom: 1px dashed var(--border);
  }

  .frames li:last-child {
    border-bottom: 0;
  }

  .state {
    color: var(--text-muted);
  }

  .error {
    color: var(--level-error);
  }
</style>
