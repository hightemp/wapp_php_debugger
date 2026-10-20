<script lang="ts">
  // Lines of the source file around a message or a frame, highlighted; the line itself is marked.
  import { ApiError, type Transport } from '../lib/api/client';
  import type { SourcePreview } from '../lib/api/types';
  import { highlight } from '../lib/highlight';
  import { i18n } from '../lib/i18n/locale.svelte';

  let { transport, messageId, frame }: { transport: Transport; messageId: number; frame?: number } =
    $props();

  let preview = $state<SourcePreview | undefined>(undefined);
  let failure = $state('');

  $effect(() => {
    preview = undefined;
    failure = '';
    transport
      .getMessageSource(messageId, frame)
      .then((loaded) => (preview = loaded))
      .catch((error: unknown) => {
        failure = error instanceof ApiError && error.detail ? error.detail : String(error);
      });
  });

  const numbers = $derived.by(() => {
    const first = preview?.first_line ?? 0;
    return (preview?.lines ?? []).map((_, index) => first + index);
  });
  const html = $derived(preview ? highlight(preview.lines.join('\n'), preview.file) : '');
</script>

{#if failure}
  <p class="state" role="status">{failure}</p>
{:else if !preview}
  <p class="state">{i18n.t('detail.loading')}</p>
{:else}
  <section class="code" aria-label={`${preview.file}:${preview.line}`}>
    <div class="numbers" aria-hidden="true">
      {#each numbers as number (number)}
        <span data-current={number === preview.line ? true : undefined}>{number}</span>
      {/each}
    </div>
    <pre
      class="lines">{#if preview.line >= preview.first_line && preview.line < preview.first_line + preview.lines.length}<span
          class="marker"
          style:top={`calc(${preview.line - preview.first_line} * var(--code-line))`}
          aria-hidden="true"
        ></span>{/if}<!-- eslint-disable-next-line svelte/no-at-html-tags -- highlight() escapes all of the code (highlight.test.ts) --><code
        >{@html html}</code
      ></pre>
  </section>
{/if}

<style>
  .state {
    margin: var(--space-2) 0;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .code {
    --code-line: 1.5em;
    display: grid;
    grid-template-columns: auto 1fr;
    margin: var(--space-2) 0;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    line-height: var(--code-line);
  }

  .numbers {
    display: grid;
    padding: var(--space-2) var(--space-2);
    border-right: 1px solid var(--border);
    color: var(--text-muted);
    text-align: right;
    user-select: none;
  }

  .numbers [data-current] {
    color: var(--text);
    font-weight: 600;
  }

  .lines {
    position: relative;
    margin: 0;
    padding: var(--space-2) var(--space-3);
    color: var(--text);
    font: inherit;
  }

  .marker {
    position: absolute;
    left: 0;
    right: 0;
    height: var(--code-line);
    margin-top: var(--space-2);
    background: var(--level-warning-soft);
    pointer-events: none;
  }

  /* The code paints over the marker of its line */
  .lines code {
    position: relative;
  }

  .lines :global(.hljs-keyword),
  .lines :global(.hljs-built_in),
  .lines :global(.hljs-type) {
    color: var(--syntax-keyword);
  }

  .lines :global(.hljs-string),
  .lines :global(.hljs-regexp) {
    color: var(--syntax-string);
  }

  .lines :global(.hljs-number),
  .lines :global(.hljs-literal),
  .lines :global(.hljs-title) {
    color: var(--syntax-number);
  }

  .lines :global(.hljs-comment),
  .lines :global(.hljs-meta) {
    color: var(--text-muted);
    font-style: italic;
  }
</style>
