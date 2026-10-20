import { describe, expect, it } from 'vitest';
import { clockSkew, formatDuration, formatTime, requestTitle } from './format';

describe('formatDuration', () => {
  it('uses ms below a second and s above', () => {
    expect(formatDuration(null, 'en')).toBe('—');
    expect(formatDuration(0.4, 'en')).toBe('<1 ms');
    expect(formatDuration(40.2, 'en')).toBe('40 ms');
    expect(formatDuration(1890.5, 'en')).toBe('1.89 s');
  });

  it('follows the locale for units and decimals', () => {
    expect(formatDuration(40.2, 'ru')).toBe('40 мс');
    expect(formatDuration(1890.5, 'ru')).toBe('1,89 с');
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

describe('clockSkew', () => {
  it('names only a clock off by more than two seconds', () => {
    expect(clockSkew(undefined)).toBeUndefined();
    expect(clockSkew(1999)).toBeUndefined();
    expect(clockSkew(-2000)).toBeUndefined();
    expect(clockSkew(12400)).toBe('+12');
    expect(clockSkew(-2600)).toBe('−3');
  });
});
