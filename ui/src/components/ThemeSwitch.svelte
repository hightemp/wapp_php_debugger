<script lang="ts">
  import { onMount } from 'svelte';
  import { onCommand } from '../lib/commands';
  import { i18n } from '../lib/i18n/locale.svelte';
  import {
    applyTheme,
    loadThemePreference,
    saveThemePreference,
    systemPrefersDark,
    themePreferences,
    type ThemePreference,
  } from '../lib/theme';
  import SegmentedControl from './SegmentedControl.svelte';

  let current = $state<ThemePreference>(loadThemePreference(localStorage));

  const options = $derived(
    themePreferences.map((preference) => ({
      value: preference,
      label: i18n.t(`theme.${preference}`),
    })),
  );

  function choose(preference: ThemePreference) {
    current = preference;
    saveThemePreference(localStorage, preference);
    applyTheme(document.documentElement, preference, systemPrefersDark());
  }

  $effect(() =>
    onCommand((command) => {
      if (command.type === 'theme') {
        choose(command.theme);
      }
    }),
  );

  onMount(() => {
    applyTheme(document.documentElement, current, systemPrefersDark());
    // "System" keeps following the OS after startup
    const query = globalThis.matchMedia?.('(prefers-color-scheme: dark)');
    const follow = () => applyTheme(document.documentElement, current, systemPrefersDark());
    query?.addEventListener('change', follow);
    return () => query?.removeEventListener('change', follow);
  });
</script>

<SegmentedControl label={i18n.t('theme.label')} {options} value={current} onchange={choose} />
