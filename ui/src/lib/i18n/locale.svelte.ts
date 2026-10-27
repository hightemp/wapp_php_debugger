// Current UI language. Components call i18n.t() in markup, so a language switch re-renders them.
import {
  detectLocale,
  parseLocale,
  translate,
  type Locale,
  type MessageKey,
  type Params,
} from './messages';

export const localeStorageKey = 'wapp-debugger.locale';

function storedLocale(): string | null {
  try {
    return localStorage.getItem(localeStorageKey);
  } catch {
    return null;
  }
}

const initial = detectLocale(
  storedLocale(),
  typeof navigator === 'undefined' ? [] : navigator.languages,
);
let current = $state<Locale>(initial);

function applyDocumentLanguage(locale: Locale) {
  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale;
  }
}

applyDocumentLanguage(initial);

export const i18n = {
  get locale(): Locale {
    return current;
  },

  /** Switches the language, remembers it and updates <html lang>. */
  set(locale: Locale) {
    current = parseLocale(locale) ?? 'en';
    applyDocumentLanguage(current);
    try {
      localStorage.setItem(localeStorageKey, current);
    } catch {
      // storage is unavailable (private mode): the choice lasts until reload
    }
  },

  t(key: MessageKey, params?: Params): string {
    return translate(current, key, params);
  },
};
