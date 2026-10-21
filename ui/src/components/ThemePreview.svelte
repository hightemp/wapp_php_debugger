<script lang="ts">
  // A small copy of the viewer in a theme: the feed, messages, variables and code. The theme editor
  // shows its draft here; the theme applies to this element only (theme.css works on any data-theme).
  import type { ThemeBase } from '../lib/themeTokens';

  let {
    theme,
    tokens = {},
    label,
  }: {
    theme: ThemeBase;
    /** Colors over the theme, by token name without the dashes. */
    tokens?: Record<string, string>;
    label: string;
  } = $props();

  // Names and values are checked (theme files, the editor): only #rrggbb gets here
  const style = $derived(
    Object.entries(tokens)
      .map(([name, value]) => `--${name}: ${value}`)
      .join('; '),
  );
</script>

<section class="preview" data-theme={theme} {style} aria-label={label}>
  <div class="bar">
    <strong>wapp-debugger</strong>
    <span class="env" data-environment="prod">prod · web-1</span>
    <span class="env" data-environment="stage">stage</span>
    <span class="env" data-environment="dev">dev</span>
  </div>
  <ul class="feed">
    <li class="row current">
      <span class="outcome">500</span>
      <span class="url">POST /api/cart/add</span>
      <span class="badge error">2 errors</span>
    </li>
    <li class="row">
      <span class="outcome ok">200</span>
      <span class="url">GET /orders?page=2</span>
      <span class="badge warning">1 warning</span>
    </li>
    <li class="row">
      <span class="outcome ok">200</span>
      <span class="url muted">cli/import.php</span>
    </li>
  </ul>
  <ul class="messages">
    <li data-level="error"><b>error</b> Undefined index: sku</li>
    <li data-level="warning"><b>warning</b> Slow query: 812 ms</li>
    <li data-level="info"><b>info</b> Loading the item</li>
    <li data-level="message"><b>message</b> Cart total</li>
    <li data-level="debug"><b>debug</b> SELECT * FROM items</li>
  </ul>
  <pre class="code"><span class="key">"item_id"</span>: <span class="number">42</span>,
<span class="key">"title"</span>: <span class="string">"Blue mug"</span>,
<span class="key">"discount"</span>: <span class="null">null</span>,
<span class="key">"in_stock"</span>: <span class="keyword">true</span></pre>
  <div class="actions">
    <button type="button" class="primary" tabindex="-1">Save</button>
    <span class="link">Open in editor</span>
  </div>
</section>

<style>
  .preview {
    display: grid;
    gap: var(--space-3);
    padding: var(--space-3);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg);
    color: var(--text);
    font-size: var(--font-size-small);
  }

  .bar,
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    align-items: center;
  }

  .env {
    padding: 0 var(--space-2);
    border: 1px solid currentcolor;
    border-radius: 999px;
  }

  [data-environment='dev'] {
    color: var(--env-dev);
  }

  [data-environment='stage'] {
    color: var(--env-stage);
  }

  [data-environment='prod'] {
    color: var(--env-prod);
  }

  .feed,
  .messages {
    margin: 0;
    padding: 0;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--surface);
    list-style: none;
  }

  .row {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border-bottom: 1px solid var(--border);
  }

  .row:last-child {
    border-bottom: 0;
  }

  .row.current {
    background: var(--surface-hover);
    box-shadow: inset 3px 0 0 var(--accent);
  }

  .outcome {
    color: var(--level-error);
    font-family: var(--font-mono);
  }

  .outcome.ok {
    color: var(--level-info);
  }

  .url {
    flex: 1;
  }

  .muted {
    color: var(--text-muted);
  }

  .badge {
    padding: 0 var(--space-2);
    border-radius: var(--radius);
  }

  .badge.error {
    background: var(--level-error-soft);
    color: var(--level-error);
  }

  .badge.warning {
    background: var(--level-warning-soft);
    color: var(--level-warning);
  }

  .messages li {
    padding: var(--space-1) var(--space-3);
  }

  .messages b {
    display: inline-block;
    width: 8ch;
    font-family: var(--font-mono);
    font-weight: 400;
  }

  [data-level='error'] b {
    color: var(--level-error);
  }

  [data-level='warning'] b {
    color: var(--level-warning);
  }

  [data-level='info'] b {
    color: var(--level-info);
  }

  [data-level='message'] b {
    color: var(--level-message);
  }

  [data-level='debug'] b {
    color: var(--level-debug);
  }

  .code {
    margin: 0;
    padding: var(--space-3);
    border-radius: var(--radius);
    background: var(--surface-sunken);
    font-family: var(--font-mono);
  }

  .key {
    color: var(--text);
  }

  .string {
    color: var(--syntax-string);
  }

  .number {
    color: var(--syntax-number);
  }

  .keyword {
    color: var(--syntax-keyword);
  }

  .null {
    color: var(--syntax-null);
  }

  .primary {
    padding: var(--space-1) var(--space-4);
    border: 1px solid var(--accent);
    border-radius: var(--radius);
    background: var(--surface);
    color: var(--text);
    font: inherit;
  }

  .link {
    color: var(--accent);
    text-decoration: underline;
  }
</style>
