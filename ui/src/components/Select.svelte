<script lang="ts" generics="T extends string | number">
  // A select-only combobox (WAI-ARIA pattern) instead of the native <select>: the popup follows
  // the theme, options can carry a hint, and the keyboard works as in a native one.
  let {
    label,
    options,
    value,
    onchange,
  }: {
    label: string;
    options: readonly { value: T; label: string; hint?: string }[];
    value: T;
    onchange: (value: T) => void;
  } = $props();

  const id = `select-${Math.random().toString(36).slice(2)}`;
  let open = $state(false);
  let activeIndex = $state(0);
  let root = $state<HTMLElement>();

  const selected = $derived(options.find((option) => option.value === value));

  function show() {
    activeIndex = Math.max(
      0,
      options.findIndex((option) => option.value === value),
    );
    open = true;
  }

  function choose(index: number) {
    const option = options[index];
    open = false;
    if (option && option.value !== value) {
      onchange(option.value);
    }
  }

  function onKeydown(event: KeyboardEvent) {
    if (!open) {
      if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(event.key)) {
        event.preventDefault();
        show();
      }
      return;
    }
    const last = options.length - 1;
    const moves: Record<string, number> = {
      ArrowDown: Math.min(activeIndex + 1, last),
      ArrowUp: Math.max(activeIndex - 1, 0),
      Home: 0,
      End: last,
    };
    if (event.key in moves) {
      event.preventDefault();
      activeIndex = moves[event.key] ?? activeIndex;
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      choose(activeIndex);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      open = false;
    } else if (event.key === 'Tab') {
      open = false;
    }
  }

  // A click outside closes the popup
  $effect(() => {
    if (!open) {
      return;
    }
    const close = (event: MouseEvent) => {
      if (root && !root.contains(event.target as Node)) {
        open = false;
      }
    };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  });
</script>

<div class="select" bind:this={root}>
  <div
    class="combobox"
    role="combobox"
    tabindex="0"
    aria-label={label}
    aria-haspopup="listbox"
    aria-expanded={open}
    aria-controls={`${id}-listbox`}
    aria-activedescendant={open ? `${id}-${activeIndex}` : undefined}
    onclick={() => (open ? (open = false) : show())}
    onkeydown={onKeydown}
  >
    <span class="value">{selected?.label ?? ''}</span>
    <span class="arrow" aria-hidden="true">▾</span>
  </div>
  {#if open}
    <ul class="listbox" role="listbox" id={`${id}-listbox`} aria-label={label}>
      {#each options as option, index (option.value)}
        <!-- svelte-ignore a11y_click_events_have_key_events (the combobox handles the keyboard) -->
        <li
          id={`${id}-${index}`}
          class="option"
          class:active={index === activeIndex}
          role="option"
          aria-selected={option.value === value}
          onmousedown={(event) => event.preventDefault()}
          onclick={() => choose(index)}
          onmouseenter={() => (activeIndex = index)}
        >
          <span>{option.label}</span>
          {#if option.hint}<span class="hint">{option.hint}</span>{/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .select {
    position: relative;
  }

  .combobox {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    justify-content: space-between;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    cursor: pointer;
  }

  .combobox:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }

  .arrow {
    color: var(--text-muted);
  }

  .listbox {
    position: absolute;
    z-index: 20;
    top: calc(100% + 2px);
    right: 0;
    left: 0;
    max-height: 240px;
    margin: 0;
    padding: var(--space-1);
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-small);
    list-style: none;
  }

  .option {
    display: flex;
    gap: var(--space-3);
    justify-content: space-between;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius);
    color: var(--text);
    cursor: pointer;
  }

  .option.active {
    background: var(--surface-hover);
  }

  .option[aria-selected='true'] {
    font-weight: 600;
  }

  .hint {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }
</style>
