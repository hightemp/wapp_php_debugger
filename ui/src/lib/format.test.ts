import { describe, expect, it } from 'vitest';
import { formatDuration, formatTime, plural, requestTitle } from './format';

describe('formatDuration', () => {
  it('uses ms below a second and s above', () => {
    expect(formatDuration(null)).toBe('—');
    expect(formatDuration(0.4)).toBe('<1 мс');
    expect(formatDuration(40.2)).toBe('40 мс');
    expect(formatDuration(1890.5)).toBe('1.89 с');
  });
});

describe('formatTime', () => {
  it('shows local time with milliseconds', () => {
    const ts = new Date(2026, 9, 19, 14, 3, 7, 45).getTime();
    expect(formatTime(ts)).toBe('14:03:07.045');
  });
});

describe('requestTitle', () => {
  it('shows method and path for web requests', () => {
    expect(
      requestTitle({ sapi: 'web', method: 'GET', url: 'https://shop.test/catalog?id=1' }),
    ).toBe('GET /catalog?id=1');
  });

  it('decodes percent-encoded text for reading', () => {
    expect(
      requestTitle({ sapi: 'web', method: 'GET', url: 'https://shop.test/search?q=%D0%BE%D1%87' }),
    ).toBe('GET /search?q=оч');
  });

  it('decodes what it can when the library cut an escape in half (sURL ≤ 300 bytes)', () => {
    expect(requestTitle({ sapi: 'web', url: 'https://shop.test/s?q=%D0%BE%D1%87%D0' })).toBe(
      '/s?q=оч%D0',
    );
  });

  it('keeps malformed escapes as is', () => {
    expect(requestTitle({ sapi: 'web', url: 'https://shop.test/a%E0%A4%A' })).toBe('/a%E0%A4%A');
  });

  it('falls back to the whole url when it cannot be parsed', () => {
    expect(requestTitle({ sapi: 'web', url: 'not a url' })).toBe('not a url');
  });

  it('shows CLI for console runs', () => {
    expect(requestTitle({ sapi: 'cli', url: null })).toBe('CLI');
  });
});

describe('plural', () => {
  it('uses Russian plural forms', () => {
    const forms: [string, string, string] = ['ошибка', 'ошибки', 'ошибок'];
    expect([1, 2, 5, 11, 21, 22, 25, 111].map((n) => plural(n, forms))).toEqual([
      '1 ошибка',
      '2 ошибки',
      '5 ошибок',
      '11 ошибок',
      '21 ошибка',
      '22 ошибки',
      '25 ошибок',
      '111 ошибок',
    ]);
  });
});
