<script lang="ts">
  // Replaces <input type="checkbox"> for on/off settings.
  let {
    label,
    checked,
    disabled = false,
    onchange,
  }: {
    label: string;
    checked: boolean;
    disabled?: boolean;
    onchange: (checked: boolean) => void;
  } = $props();
</script>

<button
  type="button"
  class="switch"
  role="switch"
  aria-checked={checked}
  aria-label={label}
  {disabled}
  onclick={() => onchange(!checked)}
>
  <span class="thumb"></span>
</button>

<style>
  .switch {
    position: relative;
    width: 34px;
    height: 20px;
    padding: 0;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface-sunken);
    cursor: pointer;
  }

  .switch[aria-checked='true'] {
    border-color: var(--accent);
    background: var(--accent);
  }

  .switch:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .thumb {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--text-muted);
    transition: transform 0.12s;
  }

  .switch[aria-checked='true'] .thumb {
    background: var(--surface);
    transform: translateX(14px);
  }

  .switch:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
</style>
