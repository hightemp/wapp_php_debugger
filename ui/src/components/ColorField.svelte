<script lang="ts">
  // Replaces <input type="color">: a #rrggbb field and a picker of hue, saturation and lightness
  // that looks the same in every browser and theme.
  import { untrack } from 'svelte';
  import { hexToHsl, hslToHex, normalizeHex, type Hsl } from '../lib/color';
  import { i18n } from '../lib/i18n/locale.svelte';

  let {
    label,
    value,
    onchange,
  }: {
    /** The token the color is for: it names the field. */
    label: string;
    value: string;
    onchange: (value: string) => void;
  } = $props();

  let text = $state('');
  let invalid = $state(false);
  let open = $state(false);
  let hsl = $state<Hsl>({ h: 0, s: 0, l: 0 });

  // A new value from outside (reset, another theme) replaces what is typed; the echo of a typed
  // one (#abc as #aabbcc) leaves the text alone
  $effect(() => {
    if (untrack(() => normalizeHex(text)) !== value) {
      text = value;
      invalid = false;
    }
    hsl = hexToHsl(value);
  });

  function type(next: string) {
    text = next;
    const color = normalizeHex(next);
    invalid = color === undefined;
    if (color) {
      onchange(color);
    }
  }

  function slide(change: Partial<Hsl>) {
    hsl = { ...hsl, ...change };
    onchange(hslToHex(hsl));
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && open) {
      // The picker closes, not the dialog around it
      event.preventDefault();
      open = false;
    }
  }

  const sliders = $derived([
    { key: 'h' as const, label: i18n.t('themeEditor.hue'), max: 359 },
    { key: 's' as const, label: i18n.t('themeEditor.saturation'), max: 100 },
    { key: 'l' as const, label: i18n.t('themeEditor.lightness'), max: 100 },
  ]);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="color-field" onkeydown={onKeydown}>
  <button
    type="button"
    class="swatch"
    style:background={value}
    aria-label={i18n.t('themeEditor.pick', { token: label })}
    aria-expanded={open}
    onclick={() => (open = !open)}
  ></button>
  <input
    class="hex"
    type="text"
    spellcheck="false"
    autocomplete="off"
    maxlength="7"
    aria-label={i18n.t('themeEditor.color', { token: label })}
    aria-invalid={invalid}
    value={text}
    oninput={(event) => type(event.currentTarget.value)}
  />
  {#if open}
    <div class="picker">
      {#each sliders as slider (slider.key)}
        <div class="slider">
          <span aria-hidden="true">{slider.label}</span>
          <input
            type="range"
            aria-label={slider.label}
            min="0"
            max={slider.max}
            value={hsl[slider.key]}
            oninput={(event) => slide({ [slider.key]: Number(event.currentTarget.value) })}
          />
          <output aria-hidden="true">{hsl[slider.key]}</output>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .color-field {
    display: grid;
    grid-template-columns: auto 9ch;
    gap: var(--space-2);
    align-items: center;
    justify-content: start;
  }

  .swatch {
    width: 24px;
    height: 24px;
    padding: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    cursor: pointer;
  }

  .hex {
    width: 100%;
    padding: 2px var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  .hex[aria-invalid='true'] {
    border-color: var(--level-error);
  }

  .swatch:focus-visible,
  .hex:focus-visible,
  .slider input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .picker {
    grid-column: 1 / -1;
    display: grid;
    gap: var(--space-2);
    padding: var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-small);
  }

  .slider {
    display: grid;
    grid-template-columns: 11ch 1fr 4ch;
    gap: var(--space-2);
    align-items: center;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .slider input {
    accent-color: var(--accent);
  }

  output {
    color: var(--text);
    font-family: var(--font-mono);
    text-align: right;
  }
</style>
