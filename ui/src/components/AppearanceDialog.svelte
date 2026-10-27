<script lang="ts">
  // Appearance settings: saved on the viewer, the same in every browser and in the desktop app,
  // and applied at once (no Save button).
  import type { Transport } from '../lib/api/client';
  import type { Settings } from '../lib/api/types';
  import { customThemeOf } from '../lib/appearance';
  import { contrast } from '../lib/contrast';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { appearance, saveSettings, type AppearanceStore } from '../lib/settings.svelte';
  import { themePreferences } from '../lib/theme';
  import { builtInTokens } from '../lib/themeTokens';
  import ColorField from './ColorField.svelte';
  import { Palette } from '../lib/icons';
  import Dialog from './Dialog.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Select from './Select.svelte';
  import Switch from './Switch.svelte';

  let {
    open,
    transport,
    store = appearance,
    onclose,
    oneditthemes,
  }: {
    open: boolean;
    transport: Transport;
    store?: AppearanceStore;
    onclose: () => void;
    oneditthemes: () => void;
  } = $props();

  const fontSizes = [12, 13, 14, 15, 16, 17, 18];
  // The same rule as the viewer applies (spec/openapi.yaml, Settings.mono_font)
  const fontFamily = /^[A-Za-z0-9 ,'_-]{1,200}$/;

  const settings = $derived(store.settings);
  const themes = $derived([
    ...themePreferences.map((value) => ({
      value: value as string,
      label: i18n.t(`theme.${value}`),
    })),
    ...settings.custom_themes.map((theme) => ({
      value: `custom:${theme.id}`,
      label: theme.name,
      hint: i18n.t('appearance.customThemes'),
    })),
  ]);

  // The colors the accent sits on: of the custom theme in use, or of the built-in one
  const colors = $derived({
    ...builtInTokens[store.appearance.theme],
    ...customThemeOf(settings)?.tokens,
  });
  const accent = $derived(settings.accent_color ?? colors.accent ?? '#000000');
  const accentContrast = $derived(contrast(accent, colors.surface ?? '#ffffff'));

  let monoFont = $state('');
  let monoInvalid = $state(false);
  $effect(() => {
    if (open) {
      monoFont = store.settings.mono_font ?? '';
      monoInvalid = false;
    }
  });

  function save(change: Partial<Settings>) {
    void saveSettings(transport, change, store);
  }

  function saveMonoFont() {
    const value = monoFont.trim();
    monoInvalid = value !== '' && !fontFamily.test(value);
    if (!monoInvalid) {
      save({ mono_font: value === '' ? null : value });
    }
  }
</script>

<Dialog icon={Palette} {open} title={i18n.t('appearance.title')} {onclose}>
  <div class="settings">
    <div class="field">
      <span class="label" aria-hidden="true">{i18n.t('appearance.theme')}</span>
      <Select
        label={i18n.t('appearance.theme')}
        options={themes}
        value={settings.theme}
        onchange={(theme: string) => save({ theme })}
      />
      <button type="button" class="link" onclick={oneditthemes}
        >{i18n.t('appearance.editThemes')}</button
      >
    </div>

    <div class="field">
      <span class="label">{i18n.t('appearance.accent')}</span>
      <div class="row">
        <ColorField
          label="accent"
          value={accent}
          onchange={(color) => save({ accent_color: color })}
        />
        <button
          type="button"
          class="chip"
          aria-pressed={settings.accent_color === null}
          onclick={() => save({ accent_color: null })}>{i18n.t('appearance.accentOfTheme')}</button
        >
      </div>
      {#if accentContrast < 3}
        <p class="warning">
          {i18n.t('appearance.accentLow', { ratio: accentContrast.toFixed(1) })}
        </p>
      {/if}
    </div>

    <div class="field">
      <span class="label" aria-hidden="true">{i18n.t('appearance.density')}</span>
      <SegmentedControl
        label={i18n.t('appearance.density')}
        options={[
          { value: 'comfortable', label: i18n.t('appearance.comfortable') },
          { value: 'compact', label: i18n.t('appearance.compact') },
        ]}
        value={settings.density}
        onchange={(density: Settings['density']) => save({ density })}
      />
    </div>

    <div class="field">
      <span class="label" aria-hidden="true">{i18n.t('appearance.fontSize')}</span>
      <Select
        label={i18n.t('appearance.fontSize')}
        options={fontSizes.map((size) => ({ value: size, label: `${size} px` }))}
        value={settings.ui_font_size}
        onchange={(size: number) => save({ ui_font_size: size })}
      />
    </div>

    <div class="field">
      <label class="field">
        <span class="label">{i18n.t('appearance.monoFont')}</span>
        <input
          class="text"
          type="text"
          maxlength="200"
          spellcheck="false"
          placeholder="JetBrains Mono"
          aria-invalid={monoInvalid}
          aria-describedby="mono-font-hint"
          bind:value={monoFont}
          onchange={saveMonoFont}
        />
      </label>
      <span id="mono-font-hint" class="hint" class:warning={monoInvalid}
        >{monoInvalid
          ? i18n.t('appearance.monoFontInvalid')
          : i18n.t('appearance.monoFontHint')}</span
      >
    </div>

    <div class="toggle">
      <Switch
        label={i18n.t('appearance.envFrame')}
        checked={settings.env_frame}
        onchange={(value) => save({ env_frame: value })}
      />
      <span aria-hidden="true">{i18n.t('appearance.envFrame')}</span>
    </div>
  </div>
</Dialog>

<style>
  .settings {
    display: grid;
    gap: var(--space-4);
  }

  .field {
    display: grid;
    gap: var(--space-2);
  }

  .label {
    color: var(--text-muted);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .row,
  .toggle {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    align-items: center;
  }

  .row {
    align-items: start;
  }

  .text {
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font-family: var(--font-mono);
  }

  .text[aria-invalid='true'] {
    border-color: var(--level-error);
  }

  .hint {
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .warning {
    margin: 0;
    color: var(--level-warning);
    font-size: var(--font-size-small);
  }

  .chip {
    padding: 2px var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .chip[aria-pressed='true'] {
    border-color: var(--accent);
    color: var(--text);
  }

  .link {
    justify-self: start;
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
    cursor: pointer;
  }

  .chip:focus-visible,
  .link:focus-visible,
  .text:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
