// UI dictionaries. English is the base language and the source of keys; Russian is the second language.
// Plural entries are keyed by Intl.PluralRules categories of each locale.

export const locales = ['en', 'ru'] as const;
export type Locale = (typeof locales)[number];

type Plural = Partial<Record<Intl.LDMLPluralRule, string>> & { other: string };
type Entry = string | Plural;

const en = {
  'app.title': 'wapp-debugger',
  'theme.label': 'Theme',
  'theme.light': 'Light',
  'theme.dark': 'Dark',
  'theme.system': 'System',
  'theme.high-contrast': 'High contrast',
  'language.label': 'Language',
  'language.en': 'English',
  'language.ru': 'Русский',
  'feed.title': 'Requests',
  'feed.loading': 'Loading…',
  'feed.error': 'Could not load requests: {message}',
  'feed.retry': 'Retry',
  'feed.emptyTitle': 'No requests yet',
  'feed.emptyHint':
    'Add the debugger to a project and open a page: the request shows up here by itself.',
  'feed.loadMore': 'Load more',
  'request.notFinished': 'not finished',
  'request.cli': 'CLI',
  'request.exitCode': 'exit {code}',
  'request.errors': { one: '{count} error', other: '{count} errors' },
  'request.warnings': { one: '{count} warning', other: '{count} warnings' },
  'request.messages': { one: '{count} message', other: '{count} messages' },
  'duration.belowMs': '<1 ms',
  'duration.ms': '{value} ms',
  'duration.s': '{value} s',
} satisfies Record<string, Entry>;

export type MessageKey = keyof typeof en;

const ru: Record<MessageKey, Entry> = {
  'app.title': 'wapp-debugger',
  'theme.label': 'Тема',
  'theme.light': 'Светлая',
  'theme.dark': 'Тёмная',
  'theme.system': 'Системная',
  'theme.high-contrast': 'Контрастная',
  'language.label': 'Язык',
  'language.en': 'English',
  'language.ru': 'Русский',
  'feed.title': 'Запросы',
  'feed.loading': 'Загрузка…',
  'feed.error': 'Не удалось загрузить запросы: {message}',
  'feed.retry': 'Повторить',
  'feed.emptyTitle': 'Запросов пока нет',
  'feed.emptyHint': 'Подключите отладчик в проект и откройте страницу: запрос появится здесь сам.',
  'feed.loadMore': 'Загрузить ещё',
  'request.notFinished': 'не завершён',
  'request.cli': 'CLI',
  'request.exitCode': 'код {code}',
  'request.errors': {
    one: '{count} ошибка',
    few: '{count} ошибки',
    many: '{count} ошибок',
    other: '{count} ошибки',
  },
  'request.warnings': {
    one: '{count} предупреждение',
    few: '{count} предупреждения',
    many: '{count} предупреждений',
    other: '{count} предупреждения',
  },
  'request.messages': {
    one: '{count} сообщение',
    few: '{count} сообщения',
    many: '{count} сообщений',
    other: '{count} сообщения',
  },
  'duration.belowMs': '<1 мс',
  'duration.ms': '{value} мс',
  'duration.s': '{value} с',
};

export const messages: Record<Locale, Record<MessageKey, Entry>> = { en, ru };

export type Params = Record<string, string | number>;

export function translate(locale: Locale, key: MessageKey, params: Params = {}): string {
  const entry = messages[locale][key];
  let text: string;
  if (typeof entry === 'string') {
    text = entry;
  } else {
    const category = new Intl.PluralRules(locale).select(Number(params.count ?? 0));
    text = entry[category] ?? entry.other;
  }
  return text.replace(/\{(\w+)\}/g, (placeholder, name: string) =>
    name in params ? String(params[name]) : placeholder,
  );
}

export function parseLocale(value: string | null): Locale | null {
  return locales.find((locale) => locale === value) ?? null;
}

/**
 * Stored choice wins; otherwise Russian when the browser's preferred language is Russian;
 * otherwise English, the base language.
 */
export function detectLocale(stored: string | null, browserLanguages: readonly string[]): Locale {
  const saved = parseLocale(stored);
  if (saved) {
    return saved;
  }
  return browserLanguages[0]?.toLowerCase().startsWith('ru') ? 'ru' : 'en';
}
