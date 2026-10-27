<script lang="ts">
  // The first steps, while there are no projects: what to do and where the help is.
  import { emitCommand } from '../lib/commands';
  import { i18n } from '../lib/i18n/locale.svelte';
  import { Bug, FolderSearch, Lightbulb, Plus, Rocket } from '../lib/icons';
</script>

<section class="start" aria-labelledby="start-title">
  <div class="logo" aria-hidden="true"><Bug size={24} /></div>
  <h2 id="start-title" class="title">{i18n.t('start.title')}</h2>
  <p class="text">{i18n.t('start.text')}</p>

  <ol class="steps">
    <li class="step">
      <span class="number">1</span>
      <div class="body">
        <h3 class="step-title">{i18n.t('start.library')}</h3>
        <p class="step-text">{i18n.t('start.libraryText')}</p>
        <button
          type="button"
          class="btn"
          onclick={() => emitCommand({ type: 'help', topic: 'library' })}
          ><Rocket size={14} />{i18n.t('start.library')}</button
        >
      </div>
    </li>
    <li class="step">
      <span class="number">2</span>
      <div class="body">
        <h3 class="step-title">{i18n.t('start.project')}</h3>
        <p class="step-text">{i18n.t('start.projectText')}</p>
        <div class="buttons">
          <button
            type="button"
            class="btn btn-primary"
            onclick={() => emitCommand({ type: 'add-project' })}
            ><Plus size={14} />{i18n.t('projects.add')}</button
          >
          <button type="button" class="btn" onclick={() => emitCommand({ type: 'open-servers' })}
            ><FolderSearch size={14} />{i18n.t('start.find')}</button
          >
        </div>
      </div>
    </li>
    <li class="step">
      <span class="number">3</span>
      <div class="body">
        <h3 class="step-title">{i18n.t('start.open')}</h3>
        <p class="step-text">{i18n.t('start.openText')}</p>
      </div>
    </li>
  </ol>

  <button
    type="button"
    class="btn btn-ghost"
    onclick={() => emitCommand({ type: 'help', topic: 'overview' })}
    ><Lightbulb size={14} />{i18n.t('start.how')}</button
  >
</section>

<style>
  .start {
    display: grid;
    justify-items: center;
    gap: var(--space-3);
    max-width: 460px;
    margin: var(--space-6) auto;
    padding: 0 var(--space-5);
    text-align: center;
  }

  .logo {
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    border-radius: var(--radius-large);
    background: var(--accent);
    color: var(--accent-contrast);
  }

  .title {
    margin: 0;
    font-size: 18px;
  }

  .text {
    margin: 0;
    color: var(--text-muted);
  }

  .steps {
    display: grid;
    gap: var(--space-3);
    width: 100%;
    margin: var(--space-3) 0;
    padding: 0;
    list-style: none;
    text-align: left;
  }

  .step {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius-large);
    background: var(--surface);
  }

  .number {
    display: grid;
    flex: none;
    place-items: center;
    width: 24px;
    height: 24px;
    border-radius: 50%;
    background: var(--accent-soft);
    color: var(--text);
    font-size: var(--font-size-small);
    font-weight: 600;
  }

  .body {
    display: grid;
    justify-items: start;
    gap: var(--space-2);
    min-width: 0;
  }

  .step-title {
    margin: 0;
    font-size: var(--font-size);
  }

  .step-text {
    margin: 0;
    color: var(--text-muted);
    font-size: var(--font-size-small);
  }

  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
</style>
