<script lang="ts">
  // The filter line of the feed: free text and conditions (level:error status:5xx …), with
  // completions as a combobox (WAI-ARIA): arrows choose, Enter or Tab take one, Escape closes.
  import { onCommand } from '../lib/commands';
  import { Search } from '../lib/icons';
  import { suggestions as complete } from '../lib/feedQuery';

  let {
    value,
    label,
    placeholder,
    tags = [],
    oninput,
  }: {
    value: string;
    label: string;
    placeholder: string;
    tags?: readonly string[];
    oninput: (value: string) => void;
  } = $props();

  const id = `filter-${Math.random().toString(36).slice(2)}`;
  let input = $state<HTMLInputElement>();

  // "/" and the palette bring the focus here
  $effect(() =>
    onCommand((command) => {
      if (command.type === 'focus-filter') {
        input?.focus();
      }
    }),
  );
  let open = $state(false);
  let active = $state(0);
  const options = $derived(open ? complete(value, tags) : []);

  function change(text: string) {
    open = true;
    active = 0;
    oninput(text);
  }

  // The word being typed gives way to the completion; a finished value gets a space after it
  function take(index: number) {
    const option = options[index];
    if (!option) {
      return;
    }
    const finished = !option.insert.endsWith(':') && !option.insert.endsWith('>');
    const text = value.replace(/\S*$/, option.insert) + (finished ? ' ' : '');
    open = !finished;
    active = 0;
    oninput(text);
  }

  function onKeydown(event: KeyboardEvent) {
    if (options.length === 0) {
      return;
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const step = event.key === 'ArrowDown' ? 1 : -1;
      active = (active + step + options.length) % options.length;
    } else if (event.key === 'Enter' || event.key === 'Tab') {
      event.preventDefault();
      take(active);
    } else if (event.key === 'Escape') {
      event.stopPropagation();
      open = false;
    }
  }
</script>

<div class="line">
  <span class="search-icon" aria-hidden="true"><Search size={14} /></span>
  <input
    bind:this={input}
    class="search"
    type="text"
    role="combobox"
    aria-label={label}
    aria-autocomplete="list"
    aria-expanded={options.length > 0}
    aria-controls={`${id}-list`}
    aria-activedescendant={options.length > 0 ? `${id}-${active}` : undefined}
    autocomplete="off"
    spellcheck="false"
    {placeholder}
    {value}
    oninput={(event) => change(event.currentTarget.value)}
    onkeydown={onKeydown}
    onblur={() => (open = false)}
  />
  {#if options.length > 0}
    <ul class="options" role="listbox" id={`${id}-list`} aria-label={label}>
      {#each options as option, index (option.insert)}
        <!-- svelte-ignore a11y_click_events_have_key_events (the input handles the keyboard) -->
        <li
          id={`${id}-${index}`}
          role="option"
          aria-selected={index === active}
          onmousedown={(event) => event.preventDefault()}
          onclick={() => take(index)}
        >
          {option.insert}
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .line {
    position: relative;
  }

  .search-icon {
    position: absolute;
    top: 50%;
    left: var(--space-3);
    display: flex;
    color: var(--text-muted);
    transform: translateY(-50%);
    pointer-events: none;
  }

  .search {
    box-sizing: border-box;
    width: 100%;
    padding: var(--space-2) var(--space-3) var(--space-2) calc(var(--space-3) + 20px);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
  }

  .search:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }

  .options {
    position: absolute;
    z-index: 20;
    top: calc(100% + 2px);
    left: 0;
    min-width: 12rem;
    margin: 0;
    padding: var(--space-1);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-small);
    list-style: none;
  }

  li {
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  li[aria-selected='true'] {
    background: var(--surface-hover);
  }
</style>
