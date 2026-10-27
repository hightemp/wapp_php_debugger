<script lang="ts">
  // Settings of the desktop app: they are saved and applied at once (no Save button).
  import type { Transport } from '../lib/api/client';
  import type { DesktopSettings, Project } from '../lib/api/types';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { Settings } from '../lib/icons';
  import Dialog from './Dialog.svelte';
  import SegmentedControl from './SegmentedControl.svelte';
  import Switch from './Switch.svelte';

  let { open, transport, onclose }: { open: boolean; transport: Transport; onclose: () => void } =
    $props();

  type MinLevel = DesktopSettings['notifications']['min_level'];
  let settings = $state<DesktopSettings | undefined>(undefined);
  let projects = $state<Project[]>([]);
  let failure = $state('');

  $effect(() => {
    if (open) {
      failure = '';
      void Promise.all([transport.getDesktopSettings(), transport.listProjects()])
        .then(([loaded, list]) => {
          settings = loaded;
          projects = list.items;
        })
        .catch((error: unknown) => (failure = String(error)));
    }
  });

  async function save(next: DesktopSettings) {
    settings = next;
    try {
      settings = await transport.updateDesktopSettings(next);
      failure = '';
    } catch (error) {
      failure = error instanceof Error ? error.message : String(error);
    }
  }

  function notifications(change: Partial<DesktopSettings['notifications']>) {
    if (settings) {
      void save({ ...settings, notifications: { ...settings.notifications, ...change } });
    }
  }

  // No project chosen means all of them
  function toggleProject(id: number, on: boolean) {
    if (!settings) {
      return;
    }
    const chosen = settings.notifications.project_ids.filter((item) => item !== id);
    notifications({ project_ids: on ? [...chosen, id] : chosen });
  }
</script>

<Dialog icon={Settings} {open} title={i18n.t('settings.title')} {onclose}>
  {#if settings}
    {@const current = settings}
    <div class="settings">
      <section class="block">
        <h3>{i18n.t('settings.notifications')}</h3>
        {@render toggle(i18n.t('settings.notify'), current.notifications.enabled, (value) =>
          notifications({ enabled: value }),
        )}
        {#if current.notifications.enabled}
          <SegmentedControl
            label={i18n.t('settings.minLevel')}
            options={[
              { value: 'error', label: i18n.t('settings.errors') },
              { value: 'warning', label: i18n.t('settings.warnings') },
            ]}
            value={current.notifications.min_level}
            onchange={(value: MinLevel) => notifications({ min_level: value })}
          />
          <div class="projects" role="group" aria-label={i18n.t('settings.projects')}>
            {@render toggle(
              i18n.t('projects.all'),
              current.notifications.project_ids.length === 0,
              (value) =>
                notifications({ project_ids: value ? [] : projects.map((project) => project.id) }),
            )}
            {#if current.notifications.project_ids.length > 0}
              {#each projects as project (project.id)}
                {@render toggle(
                  project.name,
                  current.notifications.project_ids.includes(project.id),
                  (value) => toggleProject(project.id, value),
                )}
              {/each}
            {/if}
          </div>
        {/if}
      </section>
      <section class="block">
        <h3>{i18n.t('settings.app')}</h3>
        {@render toggle(i18n.t('settings.autostart'), current.autostart, (value) =>
          save({ ...current, autostart: value }),
        )}
        {@render toggle(i18n.t('settings.closeToTray'), current.close_to_tray, (value) =>
          save({ ...current, close_to_tray: value }),
        )}
        {@render toggle(i18n.t('settings.updateCheck'), current.update_check, (value) =>
          save({ ...current, update_check: value }),
        )}
      </section>
    </div>
  {/if}
  {#if failure}
    <p class="error" role="alert">{failure}</p>
  {/if}
</Dialog>

{#snippet toggle(label: string, checked: boolean, onchange: (value: boolean) => void)}
  <div class="toggle">
    <Switch {label} {checked} {onchange} />
    <span aria-hidden="true">{label}</span>
  </div>
{/snippet}

<style>
  .settings {
    display: grid;
    gap: var(--space-4);
  }

  .block {
    display: grid;
    gap: var(--space-3);
  }

  h3 {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .projects {
    display: grid;
    gap: var(--space-2);
    padding-left: var(--space-4);
    border-left: 1px solid var(--border);
  }

  .toggle {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }

  .error {
    margin: var(--space-3) 0 0;
    color: var(--level-error);
    font-size: var(--font-size-small);
  }
</style>
