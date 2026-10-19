<script lang="ts" generics="T extends string">
  // A radio group of buttons with arrow-key navigation: replaces native <select> and radios.
  let {
    label,
    options,
    value,
    onchange,
  }: {
    label: string;
    options: readonly { value: T; label: string }[];
    value: T;
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

<div class="segmented" role="radiogroup" aria-label={label}>
  {#each options as option, index (option.value)}
    <button
      bind:this={buttons[index]}
      type="button"
      role="radio"
      aria-checked={value === option.value}
      tabindex={value === option.value ? 0 : -1}
      onclick={() => onchange(option.value)}
      onkeydown={(event) => onKeydown(event, index)}
    >
      {option.label}
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
    padding: 4px 10px;
    border: 0;
    border-radius: calc(var(--radius) - 2px);
    background: transparent;
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
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
