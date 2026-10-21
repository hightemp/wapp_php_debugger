<script lang="ts">
  // Custom themes: tokens over a built-in base, a live preview, the contrast of the colors, and
  // theme files to share. Saving puts the theme into the settings on the viewer and turns it on.
  import { untrack } from 'svelte';
  import type { Transport } from '../lib/api/client';
  import type { CustomTheme } from '../lib/api/types';
  import { customThemeOf, exportCustomTheme, parseCustomTheme, themeId } from '../lib/appearance';
  import { contrastProblems } from '../lib/contrast';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { appearance, type AppearanceStore } from '../lib/settings.svelte';
  import { toasts } from '../lib/toasts.svelte';
  import { builtInTokens, colorTokens, themeBases, type ThemeBase } from '../lib/themeTokens';
  import ColorField from './ColorField.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import Dialog from './Dialog.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Select from './Select.svelte';
  import ThemePreview from './ThemePreview.svelte';

  let {
    open,
    transport,
    store = appearance,
    onclose,
  }: {
    open: boolean;
    transport: Transport;
    store?: AppearanceStore;
    onclose: () => void;
  } = $props();

  const maxThemes = 20;
  // An empty id is a theme that is not saved yet
  let draft = $state<CustomTheme>({ id: '', name: '', base: 'light', tokens: {} });
  let deleting = $state(false);
  let saving = $state(false);

  function fresh(): CustomTheme {
    return { id: '', name: i18n.t('themeEditor.new'), base: store.appearance.theme, tokens: {} };
  }

  function edit(id: string) {
    const theme = store.settings.custom_themes.find((item) => item.id === id);
    draft = theme ? structuredClone($state.snapshot(theme)) : fresh();
  }

  // Opening starts from the custom theme in use, or a new one over the theme in use
  $effect(() => {
    if (open) {
      untrack(() => edit(customThemeOf(store.settings)?.id ?? ''));
    }
  });

  const themes = $derived(store.settings.custom_themes);
  const choices = $derived([
    { value: '', label: i18n.t('themeEditor.new') },
    ...themes.map((theme) => ({ value: theme.id, label: theme.name })),
  ]);
  const base = $derived(builtInTokens[draft.base]);
  const colors = $derived({ ...base, ...draft.tokens });
  const problems = $derived(contrastProblems(colors));

  const groups = $derived(
    (['interface', 'text', 'level', 'syntax', 'env'] as const).map((group) => ({
      group,
      tokens: colorTokens.filter((token) => groupOf(token) === group),
    })),
  );

  function groupOf(token: string) {
    const prefix = token.split('-')[0];
    if (prefix === 'level' || prefix === 'syntax' || prefix === 'env') {
      return prefix;
    }
    return prefix === 'text' ? 'text' : 'interface';
  }

  // A color equal to the base is no override
  function setToken(token: string, value: string) {
    const rest = Object.fromEntries(
      Object.entries(draft.tokens).filter(([name]) => name !== token),
    );
    draft.tokens = value === base[token] ? rest : { ...rest, [token]: value };
  }

  async function save() {
    const isNew = draft.id === '';
    if (isNew && themes.length >= maxThemes) {
      toasts.push({ kind: 'error', text: i18n.t('themeEditor.full') });
      return;
    }
    const name = draft.name.trim() || i18n.t('themeEditor.new');
    const id = isNew
      ? themeId(
          name,
          themes.map((theme) => theme.id),
        )
      : draft.id;
    const theme: CustomTheme = { ...$state.snapshot(draft), id, name };
    saving = true;
    try {
      await store.update(transport, {
        custom_themes: isNew
          ? [...themes, theme]
          : themes.map((item) => (item.id === id ? theme : item)),
        theme: `custom:${id}`,
      });
      draft = theme;
      toasts.push({ text: i18n.t('themeEditor.saved', { name }) });
    } catch (error) {
      toasts.push({
        kind: 'error',
        text: i18n.t('settings.saveFailed', {
          reason: error instanceof Error ? error.message : String(error),
        }),
      });
    } finally {
      saving = false;
    }
  }

  async function remove() {
    deleting = false;
    const id = draft.id;
    const settings = store.settings;
    try {
      await store.update(transport, {
        custom_themes: settings.custom_themes.filter((theme) => theme.id !== id),
        theme: settings.theme === `custom:${id}` ? 'system' : settings.theme,
      });
      draft = fresh();
    } catch (error) {
      toasts.push({
        kind: 'error',
        text: i18n.t('settings.saveFailed', {
          reason: error instanceof Error ? error.message : String(error),
        }),
      });
    }
  }

  function download() {
    const theme = $state.snapshot(draft);
    const id = theme.id || themeId(theme.name, []);
    const url = URL.createObjectURL(
      new Blob([exportCustomTheme({ ...theme, id })], { type: 'application/json' }),
    );
    const link = document.createElement('a');
    link.href = url;
    link.download = `${id}.theme.json`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }

  // An imported theme is a new draft: nothing is saved until Save
  async function upload(event: Event & { currentTarget: HTMLInputElement }) {
    const input = event.currentTarget;
    const file = input.files?.[0];
    input.value = '';
    if (!file) {
      return;
    }
    const parsed = parseCustomTheme(await file.text());
    if ('error' in parsed) {
      toasts.push({
        kind: 'error',
        text: i18n.t('themeEditor.importFailed', { reason: parsed.error }),
      });
      return;
    }
    draft = { ...parsed.theme, id: '' };
    toasts.push({ text: i18n.t('themeEditor.imported', { name: parsed.theme.name }) });
  }
</script>

<Dialog {open} wide title={i18n.t('themeEditor.title')} {onclose}>
  <div class="editor">
    <div class="form">
      <div class="head">
        <Select
          label={i18n.t('themeEditor.theme')}
          options={choices}
          value={draft.id}
          onchange={(id: string) => edit(id)}
        />
        <label class="name">
          <span>{i18n.t('themeEditor.name')}</span>
          <input type="text" maxlength="60" bind:value={draft.name} />
        </label>
        <SegmentedControl
          label={i18n.t('themeEditor.base')}
          options={themeBases.map((value) => ({ value, label: i18n.t(`theme.${value}`) }))}
          value={draft.base}
          onchange={(value: ThemeBase) => (draft.base = value)}
        />
      </div>
      {#each groups as { group, tokens } (group)}
        <fieldset class="group">
          <legend>{i18n.t(`themeEditor.group.${group}`)}</legend>
          {#each tokens as token (token)}
            <div class="token">
              <code>{token}</code>
              <ColorField
                label={token}
                value={colors[token] ?? '#000000'}
                onchange={(value) => setToken(token, value)}
              />
              {#if token in draft.tokens}
                <span class="changed">{i18n.t('themeEditor.changed')}</span>
                <button
                  type="button"
                  class="reset"
                  aria-label={i18n.t('themeEditor.resetToken', { token })}
                  onclick={() => setToken(token, base[token] ?? '')}>↺</button
                >
              {/if}
            </div>
          {/each}
        </fieldset>
      {/each}
    </div>
    <div class="side">
      <ThemePreview
        theme={draft.base}
        tokens={draft.tokens}
        label={i18n.t('themeEditor.preview')}
      />
      <div class="contrast" role="status">
        {#if problems.length === 0}
          <p class="ok">{i18n.t('themeEditor.contrastOk')}</p>
        {:else}
          <ul>
            {#each problems as problem (`${problem.foreground}/${problem.background}`)}
              <li>
                {i18n.t('themeEditor.contrastLow', {
                  foreground: problem.foreground,
                  background: problem.background,
                  ratio: problem.ratio.toFixed(1),
                  minimum: problem.minimum,
                })}
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
  </div>

  {#snippet footer()}
    <label class="button file-button">
      {i18n.t('themeEditor.import')}
      <input
        class="file"
        type="file"
        accept=".json,application/json"
        aria-label={i18n.t('themeEditor.import')}
        onchange={upload}
      />
    </label>
    <button type="button" class="button" onclick={download}>{i18n.t('themeEditor.export')}</button>
    <span class="spacer"></span>
    {#if draft.id}
      <button type="button" class="button danger" onclick={() => (deleting = true)}
        >{i18n.t('themeEditor.delete')}</button
      >
    {/if}
    <button type="button" class="button" onclick={() => (draft.tokens = {})}
      >{i18n.t('themeEditor.reset')}</button
    >
    <button type="button" class="button primary" disabled={saving} onclick={() => void save()}
      >{i18n.t('themeEditor.save')}</button
    >
  {/snippet}
</Dialog>

<ConfirmDialog
  open={deleting}
  title={i18n.t('themeEditor.deleteConfirm', { name: draft.name })}
  message={draft.name}
  confirmLabel={i18n.t('themeEditor.delete')}
  danger
  onconfirm={() => void remove()}
  oncancel={() => (deleting = false)}
/>

<style>
  .editor {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: var(--space-4);
    align-items: start;
  }

  .form {
    display: grid;
    gap: var(--space-3);
  }

  .head {
    display: grid;
    gap: var(--space-3);
  }

  .name {
    display: grid;
    gap: var(--space-1);
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .name input {
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    color: var(--text);
    font: inherit;
  }

  .group {
    display: grid;
    gap: var(--space-2);
    margin: 0;
    padding: var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  legend {
    padding: 0 var(--space-2);
    color: var(--text-muted);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .token {
    display: grid;
    grid-template-columns: 16ch auto auto auto;
    gap: var(--space-2);
    align-items: center;
    justify-content: start;
  }

  code {
    font-family: var(--font-mono);
    font-size: var(--font-size-small);
  }

  .changed {
    color: var(--accent);
    font-size: var(--font-size-small);
  }

  .reset {
    padding: 0 var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text-muted);
    cursor: pointer;
  }

  .side {
    position: sticky;
    top: 0;
    display: grid;
    gap: var(--space-3);
  }

  .contrast {
    font-size: var(--font-size-small);
  }

  .contrast ul {
    margin: 0;
    padding-left: var(--space-4);
    color: var(--level-warning);
  }

  .ok {
    margin: 0;
    color: var(--level-info);
  }

  .spacer {
    flex: 1;
  }

  .button {
    padding: var(--space-2) var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    font: inherit;
    cursor: pointer;
  }

  .button:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .primary {
    border-color: var(--accent);
  }

  .danger {
    color: var(--level-error);
  }

  /* The file input stays reachable from the keyboard inside its label, out of sight */
  .file-button {
    position: relative;
  }

  .file {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
  }

  .file-button:focus-within,
  .button:focus-visible,
  .reset:focus-visible,
  .name input:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  @media (max-width: 800px) {
    .editor {
      grid-template-columns: 1fr;
    }

    .side {
      position: static;
    }
  }
</style>
