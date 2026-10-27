import { describe, expect, it } from 'vitest';
import { help, helpSpans, helpTopics } from './help';
import { locales } from './i18n/messages';

describe('help', () => {
  it('has every topic in every locale, in the same order and shape', () => {
    const shape = (locale: (typeof locales)[number]) =>
      help[locale].map((section) => [
        section.id,
        section.blocks.map((block) =>
          'items' in block ? `${block.kind}:${block.items.length}` : block.kind,
        ),
      ]);
    for (const locale of locales) {
      expect(help[locale].map((section) => section.id)).toEqual([...helpTopics]);
      expect(shape(locale), locale).toEqual(shape('en'));
    }
  });

  it('keeps code samples the same in every locale', () => {
    const code = (locale: (typeof locales)[number]) =>
      help[locale].flatMap((section) =>
        section.blocks.filter((block) => block.kind === 'code').map((block) => block.text),
      );
    expect(code('ru')).toEqual(code('en'));
  });

  it('splits code and strong parts out of a text', () => {
    expect(helpSpans('Set `enabled` in **Settings** now')).toEqual([
      { kind: 'text', text: 'Set ' },
      { kind: 'code', text: 'enabled' },
      { kind: 'text', text: ' in ' },
      { kind: 'strong', text: 'Settings' },
      { kind: 'text', text: ' now' },
    ]);
    expect(helpSpans('plain')).toEqual([{ kind: 'text', text: 'plain' }]);
  });
});
