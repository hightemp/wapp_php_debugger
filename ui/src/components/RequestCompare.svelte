<script lang="ts">
  // Two requests side by side: their fields, then what changed in each superglobal ($_SESSION
  // before and after, say). The earlier request is "before" whichever way it was asked.
  import type { Transport } from '../lib/api/client';
  import type { RequestDetail } from '../lib/api/types';
  import { diffValues, type DiffRow } from '../lib/diff';
  import { formatDuration, formatTime, requestTitle } from '../lib/format';
  import { i18n } from '../lib/i18n/locale.svelte';
  import type { MessageKey } from '../lib/i18n/messages';

  let {
    transport,
    requestId,
    otherId,
    onclose,
  }: { transport: Transport; requestId: number; otherId: number; onclose: () => void } = $props();

  interface Side {
    request: RequestDetail;
    globals: Record<string, unknown>;
  }

  let sides = $state<[Side, Side] | null>(null);
  let errorMessage = $state<string | null>(null);

  // Superglobals as the library logged them: messages tagged globals (superglobals in v1)
  async function load(id: number): Promise<Side> {
    const request = await transport.getRequest(id);
    const pages = await Promise.all(
      ['globals', 'superglobals'].map((tag) => transport.listMessages(id, { tag, limit: 50 })),
    );
    const globals: Record<string, unknown> = {};
    for (const message of pages.flatMap((page) => page.items)) {
      const detail = message.has_context ? await transport.getMessage(message.id) : null;
      globals[message.message] = detail?.context ?? {};
    }
    return { request, globals };
  }

  $effect(() => {
    const ids = [requestId, otherId];
    let current = true;
    sides = null;
    errorMessage = null;
    Promise.all(ids.map(load))
      .then(([a, b]) => {
        if (current && a && b) {
          sides = a.request.started_at <= b.request.started_at ? [a, b] : [b, a];
        }
      })
      .catch((error: unknown) => {
        if (current) {
          errorMessage = error instanceof Error ? error.message : String(error);
        }
      });
    return () => {
      current = false;
    };
  });

  const fields: [MessageKey, (request: RequestDetail) => string][] = [
    ['compare.method', (request) => request.method ?? request.sapi],
    ['compare.url', (request) => request.url ?? (request.argv ?? []).join(' ')],
    ['compare.status', (request) => String(request.status_code ?? request.exit_code ?? '')],
    ['compare.duration', (request) => formatDuration(request.duration_ms, i18n.locale)],
    ['compare.messages', (request) => String(request.message_count)],
    ['compare.errors', (request) => String(request.error_count)],
    ['compare.warnings', (request) => String(request.warning_count)],
    ['compare.php', (request) => request.php_version ?? ''],
  ];

  const changes = $derived.by((): [string, DiffRow[]][] => {
    if (!sides) {
      return [];
    }
    const [before, after] = sides;
    const names = [...new Set([...Object.keys(before.globals), ...Object.keys(after.globals)])];
    return names.map((name) => [name, diffValues(before.globals[name], after.globals[name])]);
  });
</script>

<section class="compare" aria-label={i18n.t('compare.title')}>
  <header>
    <h2>{i18n.t('compare.title')}</h2>
    <button type="button" class="close" onclick={onclose}>{i18n.t('compare.close')}</button>
  </header>

  {#if errorMessage}
    <p class="state error" role="alert">{errorMessage}</p>
  {:else if !sides}
    <p class="state">{i18n.t('detail.loading')}</p>
  {:else}
    {@const [before, after] = sides}
    <table aria-label={i18n.t('compare.requests')}>
      <thead>
        <tr>
          <th></th>
          <th scope="col"
            >{requestTitle(before.request)}<br /><small
              >{formatTime(before.request.started_at)}</small
            ></th
          >
          <th scope="col"
            >{requestTitle(after.request)}<br /><small>{formatTime(after.request.started_at)}</small
            ></th
          >
        </tr>
      </thead>
      <tbody>
        {#each fields as [label, value] (label)}
          {@const a = value(before.request)}
          {@const b = value(after.request)}
          <tr data-changed={a !== b}>
            <th scope="row">{i18n.t(label)}</th>
            <td>{a}</td>
            <td>{b}</td>
          </tr>
        {/each}
      </tbody>
    </table>

    {#if changes.length === 0}
      <p class="state">{i18n.t('compare.noGlobals')}</p>
    {/if}
    {#each changes as [name, rows] (name)}
      {#if rows.length === 0}
        <p class="same">{i18n.t('compare.same', { name })}</p>
      {:else}
        <table aria-label={name}>
          <caption>{name}</caption>
          <thead>
            <tr>
              <th scope="col">{i18n.t('compare.path')}</th>
              <th scope="col"></th>
              <th scope="col">{i18n.t('compare.before')}</th>
              <th scope="col">{i18n.t('compare.after')}</th>
            </tr>
          </thead>
          <tbody>
            {#each rows as row (row.path)}
              <tr data-kind={row.kind}>
                <th scope="row" class="path">{row.path}</th>
                <td class="kind">{i18n.t(`compare.${row.kind}`)}</td>
                <td class="value">{row.before ?? ''}</td>
                <td class="value">{row.after ?? ''}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    {/each}
  {/if}
</section>

<style>
  .compare {
    display: grid;
    gap: var(--space-3);
    align-content: start;
    padding: var(--space-3);
  }

  header {
    display: flex;
    gap: var(--space-3);
    align-items: center;
    justify-content: space-between;
  }

  h2 {
    margin: 0;
    font-size: var(--font-size-large, 1.1rem);
  }

  .close {
    padding: 0 var(--space-3);
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface);
    color: var(--text-muted);
    font: inherit;
    font-size: var(--font-size-small);
    cursor: pointer;
  }

  .close:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  /* Fixed columns: the tables line up under each other */
  table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
    font-size: var(--font-size-small);
  }

  table :is(th, td):first-child {
    width: 28%;
  }

  table:not(:first-of-type) :is(th, td):nth-child(2) {
    width: 10%;
  }

  caption {
    padding-bottom: var(--space-1);
    font-family: var(--font-mono);
    font-weight: 600;
    text-align: left;
  }

  th,
  td {
    padding: var(--space-1) var(--space-2);
    border-bottom: 1px solid var(--border);
    text-align: left;
    vertical-align: top;
    overflow-wrap: anywhere;
  }

  thead th {
    color: var(--text-muted);
    font-weight: 500;
  }

  tr[data-changed='true'] td {
    background: var(--surface-hover);
    font-weight: 600;
  }

  .path,
  .value {
    font-family: var(--font-mono);
    font-weight: 400;
  }

  tr[data-kind='added'] .kind {
    color: var(--level-info);
  }

  tr[data-kind='removed'] .kind {
    color: var(--level-error);
  }

  tr[data-kind='changed'] .kind {
    color: var(--level-warning);
  }

  .same,
  .state {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .error {
    color: var(--level-error);
  }
</style>
