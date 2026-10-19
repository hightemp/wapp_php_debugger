<script lang="ts">
  import { i18n } from '../lib/i18n/locale.svelte';
  import { locales, type Locale } from '../lib/i18n/messages';
  import SegmentedControl from './SegmentedControl.svelte';

  // Language names are shown in their own language
  const options = $derived(
    locales.map((locale) => ({ value: locale, label: i18n.t(`language.${locale}`) })),
  );
</script>

<SegmentedControl
  label={i18n.t('language.label')}
  {options}
  value={i18n.locale}
  onchange={(locale: Locale) => i18n.set(locale)}
/>
