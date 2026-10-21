<script lang="ts">
  import type { Transport } from '../lib/api/client';
  import { onCommand } from '../lib/commands';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { appearance, saveSettings, type AppearanceStore } from '../lib/settings.svelte';
  import { themePreferences, type ThemePreference } from '../lib/theme';
  import SegmentedControl from './SegmentedControl.svelte';

  let { transport, store = appearance }: { transport: Transport; store?: AppearanceStore } =
    $props();

  const options = $derived(
    themePreferences.map((preference) => ({
      value: preference,
      label: i18n.t(`theme.${preference}`),
    })),
  );

  function choose(theme: string) {
    void saveSettings(transport, { theme }, store);
  }

  $effect(() =>
    onCommand((command) => {
      if (command.type === 'theme') {
        choose(command.theme);
      }
    }),
  );
</script>

<!-- A custom theme is chosen in the appearance settings; then no built-in one is checked -->
<SegmentedControl
  label={i18n.t('theme.label')}
  {options}
  value={store.settings.theme as ThemePreference}
  onchange={choose}
/>
