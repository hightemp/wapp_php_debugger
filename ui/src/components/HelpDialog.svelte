<script lang="ts">
  import type { Component } from 'svelte';
  import { help, helpSpans, type HelpTopic } from '../lib/help';
  import {
    BookOpen,
    Check,
    Copy,
    Help,
    Keyboard,
    Lightbulb,
    ListChecks,
    Radio,
    Rocket,
    Search,
    Server,
  } from '../lib/icons';
  import { i18n } from '../lib/i18n/locale.svelte';
  import Dialog from './Dialog.svelte';

  let {
    open,
    topic = 'overview',
    onclose,
    onshortcuts = () => {},
  }: {
    open: boolean;
    /** The topic to open on. */
    topic?: HelpTopic;
    onclose: () => void;
    /** The full list of keyboard shortcuts. */
    onshortcuts?: () => void;
  } = $props();

  const icons: Record<HelpTopic, Component<{ size?: number }>> = {
    overview: Lightbulb,
    library: Rocket,
    projects: ListChecks,
    servers: Server,
    push: Radio,
    reading: Search,
    shortcuts: Keyboard,
  };

  let current = $state<HelpTopic>('overview');
  let copied = $state<number | undefined>(undefined);

  $effect(() => {
    if (open) {
      current = topic;
      copied = undefined;
    }
  });

  const sections = $derived(help[i18n.locale]);
  // Every locale has every topic (help.test.ts)
  const section = $derived(
    sections.find((item) => item.id === current) ?? (sections[0] as (typeof sections)[number]),
  );

  async function copy(text: string, index: number) {
    await navigator.clipboard.writeText(text);
    copied = index;
    setTimeout(() => (copied = copied === index ? undefined : copied), 2000);
  }
</script>

{#snippet rich(text: string)}
  {#each helpSpans(text) as span, index (index)}
    {#if span.kind === 'code'}<code>{span.text}</code>{:else if span.kind === 'strong'}<strong
        >{span.text}</strong
      >{:else}{span.text}{/if}
  {/each}
{/snippet}

<Dialog {open} title={i18n.t('help.title')} icon={Help} wide {onclose}>
  <div class="help">
    <nav class="topics" aria-label={i18n.t('help.topics')}>
      {#each sections as item (item.id)}
        {@const Icon = icons[item.id]}
        <button
          type="button"
          class="topic"
          aria-current={item.id === current ? 'true' : undefined}
          onclick={() => {
            current = item.id;
            copied = undefined;
          }}
        >
          <Icon size={16} />
          <span class="topic-text">
            <span class="topic-title">{item.title}</span>
            <span class="topic-summary">{item.summary}</span>
          </span>
        </button>
      {/each}
    </nav>

    <article class="content">
      <h3 class="heading">{section.title}</h3>
      {#each section.blocks as block, index (index)}
        {#if block.kind === 'text'}
          <p>{@render rich(block.text)}</p>
        {:else if block.kind === 'tip'}
          <p class="tip"><Lightbulb size={16} /><span>{@render rich(block.text)}</span></p>
        {:else if block.kind === 'steps'}
          <ol class="steps">
            {#each block.items as item, step (step)}
              <li><span class="step">{step + 1}</span><span>{@render rich(item)}</span></li>
            {/each}
          </ol>
        {:else if block.kind === 'list'}
          <ul class="list">
            {#each block.items as item, at (at)}
              <li>{@render rich(item)}</li>
            {/each}
          </ul>
        {:else}
          <div class="code">
            <pre><code>{block.text}</code></pre>
            <button
              type="button"
              class="icon-btn copy"
              aria-label={i18n.t(copied === index ? 'help.copied' : 'help.copy')}
              title={i18n.t(copied === index ? 'help.copied' : 'help.copy')}
              onclick={() => void copy(block.text, index)}
            >
              {#if copied === index}<Check size={14} />{:else}<Copy size={14} />{/if}
            </button>
          </div>
        {/if}
      {/each}
      {#if section.id === 'shortcuts'}
        <button type="button" class="btn" onclick={onshortcuts}>
          <Keyboard size={14} />{i18n.t('help.allShortcuts')}
        </button>
      {/if}
      {#if section.id === 'library'}
        <p class="more">
          <BookOpen size={14} />
          <a
            href="https://github.com/hightemp/wapp_php_debugger#readme"
            target="_blank"
            rel="noopener noreferrer">README</a
          >
        </p>
      {/if}
    </article>
  </div>
</Dialog>

<style>
  .help {
    display: grid;
    grid-template-columns: minmax(200px, 260px) minmax(0, 1fr);
    gap: var(--space-5);
    min-height: min(560px, 70vh);
  }

  .topics {
    display: grid;
    align-content: start;
    gap: var(--space-1);
    padding-right: var(--space-4);
    border-right: 1px solid var(--border);
  }

  .topic {
    display: flex;
    gap: var(--space-3);
    align-items: flex-start;
    padding: var(--space-2) var(--space-3);
    border: 0;
    border-radius: var(--radius);
    background: none;
    color: var(--text-muted);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .topic :global(.lucide-icon) {
    margin-top: 2px;
  }

  .topic:hover {
    background: var(--surface-hover);
    color: var(--text);
  }

  .topic[aria-current='true'] {
    background: var(--accent-soft);
    color: var(--text);
  }

  .topic:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .topic-text {
    display: grid;
  }

  .topic-title {
    color: var(--text);
    font-weight: 600;
  }

  .topic-summary {
    font-size: var(--font-size-small);
  }

  .content {
    min-width: 0;
    line-height: 1.55;
  }

  .heading {
    margin: 0 0 var(--space-4);
    font-size: 18px;
  }

  p {
    margin: 0 0 var(--space-4);
  }

  code {
    padding: 0 var(--space-1);
    border-radius: 4px;
    background: var(--surface-sunken);
    font-family: var(--font-mono);
    font-size: 0.92em;
  }

  .tip {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius);
    background: var(--accent-soft);
  }

  .tip :global(.lucide-icon) {
    margin-top: 3px;
    color: var(--accent);
  }

  .steps {
    display: grid;
    gap: var(--space-3);
    margin: 0 0 var(--space-4);
    padding: 0;
    list-style: none;
  }

  .steps li {
    display: flex;
    gap: var(--space-3);
  }

  .step {
    display: grid;
    flex: none;
    place-items: center;
    width: 22px;
    height: 22px;
    border-radius: 50%;
    background: var(--accent);
    color: var(--accent-contrast);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .list {
    display: grid;
    gap: var(--space-2);
    margin: 0 0 var(--space-4);
    padding-left: var(--space-5);
  }

  .code {
    position: relative;
    margin: 0 0 var(--space-4);
  }

  pre {
    margin: 0;
    padding: var(--space-3) var(--space-8) var(--space-3) var(--space-4);
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
  }

  pre code {
    padding: 0;
    background: none;
    white-space: pre;
  }

  .copy {
    position: absolute;
    top: var(--space-2);
    right: var(--space-2);
  }

  .more {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }

  a {
    color: var(--accent);
  }

  @media (max-width: 760px) {
    .help {
      grid-template-columns: 1fr;
    }

    .topics {
      grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
      padding: 0 0 var(--space-4);
      border-right: 0;
      border-bottom: 1px solid var(--border);
    }

    .topic-summary {
      display: none;
    }
  }
</style>
