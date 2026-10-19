<script lang="ts">
  import { onMount } from 'svelte';
  import {
    applyTheme,
    loadThemePreference,
    saveThemePreference,
    systemPrefersDark,
    themeLabels,
    themePreferences,
    type ThemePreference,
  } from '../lib/theme';

  let current = $state<ThemePreference>(loadThemePreference(localStorage));
  const buttons: HTMLButtonElement[] = [];

  function choose(preference: ThemePreference, focus = false) {
    current = preference;
    saveThemePreference(localStorage, preference);
    applyTheme(document.documentElement, preference, systemPrefersDark());
    if (focus) {
      buttons[themePreferences.indexOf(preference)]?.focus();
    }
  }

  function onKeydown(event: KeyboardEvent, index: number) {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key];
    if (step === undefined) {
      return;
    }
    event.preventDefault();
    const next =
      themePreferences[(index + step + themePreferences.length) % themePreferences.length];
    if (next) {
      choose(next, true);
    }
  }

  onMount(() => {
    applyTheme(document.documentElement, current, systemPrefersDark());
    // «Системная» следует за ОС и после загрузки
    const query = globalThis.matchMedia?.('(prefers-color-scheme: dark)');
    const follow = () => applyTheme(document.documentElement, current, systemPrefersDark());
    query?.addEventListener('change', follow);
    return () => query?.removeEventListener('change', follow);
  });
</script>

<div class="switch" role="radiogroup" aria-label="Тема">
  {#each themePreferences as preference, index (preference)}
    <button
      bind:this={buttons[index]}
      type="button"
      role="radio"
      aria-checked={current === preference}
      tabindex={current === preference ? 0 : -1}
      onclick={() => choose(preference)}
      onkeydown={(event) => onKeydown(event, index)}
    >
      {themeLabels[preference]}
    </button>
  {/each}
</div>

<style>
  .switch {
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
