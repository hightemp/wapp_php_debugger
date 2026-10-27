<script lang="ts" generics="T extends string">
  // A radio group of buttons with arrow-key navigation: replaces native <select> and radios.
  import type { Component } from 'svelte';

  let {
    label,
    options,
    value,
    compact = false,
    onchange,
  }: {
    label: string;
    options: readonly {
      value: T;
      label: string;
      icon?: Component<{ size?: number }>;
      /** Shown instead of the label when compact. */
      short?: string;
    }[];
    value: T;
    /** Only the icon or the short label shows; the label is the name and the tooltip. */
    compact?: boolean;
    onchange: (value: T) => void;
  } = $props();

  const buttons: HTMLButtonElement[] = [];

  function onKeydown(event: KeyboardEvent, index: number) {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key];
    if (step === undefined) {
      return;
    }
    event.preventDefault();
    const nextIndex = (index + step + options.length) % options.length;
    const next = options[nextIndex];
    if (next) {
      onchange(next.value);
      buttons[nextIndex]?.focus();
    }
  }
</script>

<div class="segmented" class:compact role="radiogroup" aria-label={label}>
  {#each options as option, index (option.value)}
    <button
      bind:this={buttons[index]}
      type="button"
      role="radio"
      aria-checked={value === option.value}
      tabindex={value === option.value ? 0 : -1}
      onclick={() => onchange(option.value)}
      onkeydown={(event) => onKeydown(event, index)}
      aria-label={compact ? option.label : undefined}
      title={compact ? option.label : undefined}
    >
      {#if option.icon}
        {@const Icon = option.icon}
        <Icon size={14} />
      {/if}
      {#if !compact}{option.label}{:else if !option.icon}{option.short ?? option.label}{/if}
    </button>
  {/each}
</div>

<style>
  .segmented {
    display: inline-flex;
    gap: 2px;
    padding: 2px;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
  }

  button {
    display: inline-flex;
    gap: var(--space-2);
    align-items: center;
    justify-content: center;
    min-width: 26px;
    padding: 4px 10px;
    border: 0;
    border-radius: calc(var(--radius) - 2px);
    background: transparent;
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .compact button {
    padding: 4px 6px;
  }

  button:hover {
    color: var(--text);
  }

  button[aria-checked='true'] {
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-small);
  }

  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
