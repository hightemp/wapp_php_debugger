<script lang="ts">
  // The component showcase (#/dev/components): every component in every built-in theme side by side,
  // to check a change of tokens or of a component at a glance. The e2e snapshots take it too.
  import { i18n } from '../lib/i18n/locale.svelte';
  import { themeBases, type ThemeBase } from '../lib/themeTokens';
  import ColorField from './ColorField.svelte';
  import JsonTree from './JsonTree.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Select from './Select.svelte';
  import Switch from './Switch.svelte';
  import Tabs from './Tabs.svelte';
  import ThemePreview from './ThemePreview.svelte';

  let shown = $state<ThemeBase | 'all'>('all');
  const columns = $derived(shown === 'all' ? themeBases : [shown]);

  // Each column keeps its own state: the components are live
  let density = $state<Record<string, 'comfortable' | 'compact'>>({});
  let toggles = $state<Record<string, boolean>>({});
  let servers = $state<Record<string, number>>({});
  let tabs = $state<Record<string, string>>({});
  let colors = $state<Record<string, string>>({});

  const variables = {
    item_id: 42,
    title: 'Blue mug',
    discount: null,
    in_stock: true,
    tags: ['kitchen', 'sale'],
  };
</script>

<div class="showcase">
  <header class="header">
    <h1>{i18n.t('showcase.title')}</h1>
    <p>{i18n.t('showcase.intro')}</p>
    <SegmentedControl
      label={i18n.t('theme.label')}
      options={[
        { value: 'all' as const, label: '∗' },
        ...themeBases.map((value) => ({ value, label: i18n.t(`theme.${value}`) })),
      ]}
      value={shown}
      onchange={(value: ThemeBase | 'all') => (shown = value)}
    />
    <a href="#/">{i18n.t('showcase.back')}</a>
  </header>
  <div class="columns">
    {#each columns as theme (theme)}
      <section
        class="column"
        data-theme={theme}
        aria-label={`${i18n.t('theme.label')} ${i18n.t(`theme.${theme}`)}`}
      >
        <h2>{i18n.t(`theme.${theme}`)}</h2>
        <SegmentedControl
          label={i18n.t('appearance.density')}
          options={[
            { value: 'comfortable' as const, label: i18n.t('appearance.comfortable') },
            { value: 'compact' as const, label: i18n.t('appearance.compact') },
          ]}
          value={density[theme] ?? 'comfortable'}
          onchange={(value) => (density[theme] = value)}
        />
        <div class="row">
          <Switch
            label="On"
            checked={toggles[theme] ?? true}
            onchange={(value) => (toggles[theme] = value)}
          />
          <Switch label="Off" checked={false} onchange={() => {}} />
          <Switch label="Disabled" checked={true} disabled onchange={() => {}} />
        </div>
        <Select
          label="Server"
          options={[
            { value: 1, label: 'local' },
            { value: 2, label: 'stage', hint: 'deploy@stage-web' },
            { value: 3, label: 'prod', hint: 'deploy@prod-web' },
          ]}
          value={servers[theme] ?? 2}
          onchange={(value) => (servers[theme] = value)}
        />
        <Tabs
          label="Request"
          idPrefix={`showcase-${theme}`}
          tabs={[
            { id: 'messages', label: 'Messages', count: 12 },
            { id: 'timeline', label: 'Timeline' },
            { id: 'sql', label: 'SQL', count: 3 },
          ]}
          value={tabs[theme] ?? 'messages'}
          onchange={(value) => (tabs[theme] = value)}
        />
        <ColorField
          label="accent"
          value={colors[theme] ?? '#2f6fed'}
          onchange={(value) => (colors[theme] = value)}
        />
        <div class="json">
          <JsonTree value={variables} />
        </div>
        <ThemePreview {theme} label={i18n.t('themeEditor.preview')} />
      </section>
    {/each}
  </div>
</div>

<style>
  .showcase {
    min-height: 100vh;
    background: var(--bg);
    color: var(--text);
  }

  .header {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4);
    align-items: center;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }

  h1 {
    margin: 0;
    font-size: 15px;
  }

  .header p {
    margin: 0;
    color: var(--text-muted);
  }

  .header a {
    color: var(--accent);
  }

  .columns {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  }

  .column {
    display: grid;
    gap: var(--space-4);
    align-content: start;
    padding: var(--space-4);
    background: var(--surface);
    color: var(--text);
    border-right: 1px solid var(--border);
  }

  h2 {
    margin: 0;
    font-size: var(--font-size);
  }

  .row {
    display: flex;
    gap: var(--space-3);
  }

  .json {
    padding: var(--space-2);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }
</style>
