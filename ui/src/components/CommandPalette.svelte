<script lang="ts">
  // Ctrl+K: type to find a command, arrows to choose, Enter to run. Typed text that is no command
  // can still filter the feed.
  import { i18n } from '../lib/i18n/locale.svelte';
  import Dialog from './Dialog.svelte';

  export interface PaletteCommand {
    id: string;
    label: string;
    hint?: string;
    run: () => void;
  }

  let {
    open,
    commands,
    onfilter,
    onclose,
  }: {
    open: boolean;
    commands: readonly PaletteCommand[];
    /** Runs "filter the feed" with what was typed */
    onfilter: (text: string) => void;
    onclose: () => void;
  } = $props();

  let text = $state('');
  let active = $state(0);

  $effect(() => {
    if (open) {
      text = '';
      active = 0;
    }
  });

  const matching = $derived.by((): PaletteCommand[] => {
    const needle = text.trim().toLowerCase();
    const found = commands.filter((command) => command.label.toLowerCase().includes(needle));
    if (needle === '') {
      return found;
    }
    const filter: PaletteCommand = {
      id: 'filter',
      label: i18n.t('palette.filter', { text: text.trim() }),
      run: () => onfilter(text.trim()),
    };
    return [...found, filter];
  });

  function run(command: PaletteCommand | undefined) {
    if (command) {
      onclose();
      command.run();
    }
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const step = event.key === 'ArrowDown' ? 1 : -1;
      active = (active + step + matching.length) % Math.max(1, matching.length);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      run(matching[active]);
    }
  }
</script>

<Dialog {open} title={i18n.t('palette.title')} {onclose}>
  <input
    class="input"
    type="text"
    role="combobox"
    aria-label={i18n.t('palette.input')}
    aria-expanded={matching.length > 0}
    aria-controls="palette-list"
    aria-activedescendant={matching.length > 0 ? `palette-${active}` : undefined}
    autocomplete="off"
    placeholder={i18n.t('palette.hint')}
    value={text}
    oninput={(event) => {
      text = event.currentTarget.value;
      active = 0;
    }}
    onkeydown={onKeydown}
  />
  <ul class="list" role="listbox" id="palette-list" aria-label={i18n.t('palette.title')}>
    {#each matching as command, index (command.id)}
      <!-- svelte-ignore a11y_click_events_have_key_events (the input handles the keyboard) -->
      <li
        id={`palette-${index}`}
        role="option"
        aria-selected={index === active}
        onmousedown={(event) => event.preventDefault()}
        onclick={() => run(command)}
        onmouseenter={() => (active = index)}
      >
        <span>{command.label}</span>
        {#if command.hint}<span class="hint">{command.hint}</span>{/if}
      </li>
    {/each}
  </ul>
</Dialog>

<style>
  .input {
    box-sizing: border-box;
    width: 100%;
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
  }

  .input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 0;
  }

  .list {
    max-height: 50vh;
    margin: var(--space-2) 0 0;
    padding: 0;
    overflow: auto;
    list-style: none;
  }

  li {
    display: flex;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius);
    cursor: pointer;
  }

  li[aria-selected='true'] {
    background: var(--surface-hover);
  }

  .hint {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }
</style>
