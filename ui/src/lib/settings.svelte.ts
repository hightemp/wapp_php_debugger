// UI settings of this window: applied to the page at once, cached for the first paint and saved on
// the viewer. Saves of other windows come as settings_updated.
import type { Transport } from './api/client';
import type { Settings } from './api/types';
import {
  appearanceOf,
  applyAppearance,
  loadCachedSettings,
  saveCachedSettings,
  systemMedia,
  type Appearance,
  type Media,
} from './appearance';
import { i18n } from './i18n/locale.svelte';
import { toasts } from './toasts.svelte';

interface Options {
  storage: Pick<Storage, 'getItem' | 'setItem'>;
  root: HTMLElement;
  media: () => Media;
  /** A language chosen in another window or saved earlier. */
  onLocale?: (locale: NonNullable<Settings['locale']>) => void;
}

export function createAppearanceStore(options: Options) {
  let current = $state<Settings>(loadCachedSettings(options.storage));
  // What the viewer has: a refused save goes back to it
  let confirmed = current;
  let pending = 0;

  function show(settings: Settings) {
    current = settings;
    applyAppearance(options.root, appearanceOf(settings, options.media()));
  }

  function receive(settings: Settings) {
    confirmed = settings;
    saveCachedSettings(options.storage, settings);
    show(settings);
    if (settings.locale) {
      options.onLocale?.(settings.locale);
    }
  }

  async function load(transport: Transport) {
    receive(await transport.getSettings());
  }

  return {
    get settings(): Settings {
      return current;
    },

    /** What the settings look like now, with the theme the system one resolves to. */
    get appearance(): Appearance {
      return appearanceOf(current, options.media());
    },

    load,

    /** Shows the change now and saves the whole settings; quick changes add up. */
    async update(transport: Transport, change: Partial<Settings>) {
      show({ ...current, ...change });
      pending++;
      try {
        const saved = await transport.updateSettings(current);
        if (--pending === 0) {
          receive(saved);
        }
      } catch (error) {
        if (--pending === 0) {
          show(confirmed);
        }
        throw error;
      }
    },

    /** Loads the settings and follows other windows and the OS; returns the stop function. */
    connect(transport: Transport): () => void {
      show(current);
      void load(transport).catch(() => {
        // the viewer is not there yet: the cached settings stay
      });
      const unsubscribe = transport.subscribe((event) => {
        if (event.event === 'settings_updated' && event.settings && pending === 0) {
          receive(event.settings);
        }
      });
      const queries = ['(prefers-color-scheme: dark)', '(prefers-contrast: more)'].map((query) =>
        globalThis.matchMedia?.(query),
      );
      const follow = () => show(current);
      for (const query of queries) {
        query?.addEventListener('change', follow);
      }
      return () => {
        unsubscribe();
        for (const query of queries) {
          query?.removeEventListener('change', follow);
        }
      };
    },
  };
}

export type AppearanceStore = ReturnType<typeof createAppearanceStore>;

function browserStorage(): Pick<Storage, 'getItem' | 'setItem'> {
  try {
    return localStorage;
  } catch {
    return { getItem: () => null, setItem: () => {} };
  }
}

export const appearance = createAppearanceStore({
  storage: browserStorage(),
  root: document.documentElement,
  media: systemMedia,
  onLocale: (locale) => {
    if (locale !== i18n.locale) {
      i18n.set(locale);
    }
  },
});

/** Saves a change of the settings; a refusal is told in a toast and the change goes back. */
export function saveSettings(
  transport: Transport,
  change: Partial<Settings>,
  store: AppearanceStore = appearance,
): Promise<void> {
  return store.update(transport, change).catch((error: unknown) => {
    toasts.push({
      kind: 'error',
      text: i18n.t('settings.saveFailed', {
        reason: error instanceof Error ? error.message : String(error),
      }),
    });
  });
}
