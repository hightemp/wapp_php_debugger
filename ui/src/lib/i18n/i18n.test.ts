import { describe, expect, it } from 'vitest';
import { detectLocale, locales, messages, translate, type Locale } from './messages';

function placeholders(text: string): string[] {
  return [...text.matchAll(/\{(\w+)\}/g)].map((match) => match[1] ?? '').sort();
}

function variants(entry: unknown): string[] {
  return typeof entry === 'string' ? [entry] : Object.values(entry as Record<string, string>);
}

describe('dictionaries', () => {
  it('have the same keys in every locale', () => {
    const english = Object.keys(messages.en).sort();
    for (const locale of locales) {
      expect(Object.keys(messages[locale]).sort(), locale).toEqual(english);
    }
  });

  it('use the same placeholders as English', () => {
    for (const locale of locales) {
      for (const [key, entry] of Object.entries(messages[locale])) {
        const english = messages.en[key as keyof typeof messages.en];
        for (const text of variants(entry)) {
          expect(placeholders(text), `${locale}.${key}`).toEqual(
            placeholders(variants(english)[0] ?? ''),
          );
        }
      }
    }
  });

  it('cover every plural category of the locale', () => {
    for (const locale of locales) {
      const categories = new Intl.PluralRules(locale).resolvedOptions().pluralCategories;
      for (const [key, entry] of Object.entries(messages[locale])) {
        if (typeof entry !== 'string') {
          for (const category of categories) {
            expect(entry, `${locale}.${key}.${category}`).toHaveProperty(category);
          }
        }
      }
    }
  });
});

describe('translate', () => {
  it('interpolates parameters', () => {
    expect(translate('en', 'request.exitCode', { code: 255 })).toBe('exit 255');
    expect(translate('ru', 'request.exitCode', { code: 255 })).toBe('код 255');
  });

  it('picks English plural forms', () => {
    expect(translate('en', 'request.errors', { count: 1 })).toBe('1 error');
    expect(translate('en', 'request.errors', { count: 2 })).toBe('2 errors');
  });

  it('picks Russian plural forms', () => {
    const forms = [1, 2, 5, 11, 21, 22, 25].map((count) =>
      translate('ru', 'request.errors', { count }),
    );
    expect(forms).toEqual([
      '1 ошибка',
      '2 ошибки',
      '5 ошибок',
      '11 ошибок',
      '21 ошибка',
      '22 ошибки',
      '25 ошибок',
    ]);
  });
});

describe('detectLocale', () => {
  const cases: [string | null, readonly string[], Locale][] = [
    ['ru', ['en-US'], 'ru'],
    ['en', ['ru-RU'], 'en'],
    [null, ['ru-RU', 'en'], 'ru'],
    [null, ['de-DE', 'ru'], 'en'],
    [null, [], 'en'],
    ['fr', ['fr-FR'], 'en'],
  ];

  it.each(cases)('stored %s, browser %j → %s', (stored, languages, expected) => {
    expect(detectLocale(stored, languages)).toBe(expected);
  });
});
