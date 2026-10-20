import { describe, expect, it } from 'vitest';
import { asCurl, withXdebug } from './replay';

describe('asCurl', () => {
  it('repeats a GET with the headers the request came with', () => {
    const { command, masked } = asCurl(
      { method: 'GET', url: 'https://shop.test/catalog?id=42' },
      {
        server: {
          HTTP_HOST: 'shop.test',
          HTTP_ACCEPT: 'text/html',
          HTTP_X_REQUESTED_WITH: 'XMLHttpRequest',
          HTTP_CONNECTION: 'keep-alive',
          HTTP_ACCEPT_ENCODING: 'gzip',
          HTTP_COOKIE: 'sid=abc',
          REMOTE_ADDR: '10.0.0.1',
        },
      },
    );
    expect(command).toBe(
      [
        "curl 'https://shop.test/catalog?id=42'",
        "  -H 'Accept: text/html'",
        "  -H 'X-Requested-With: XMLHttpRequest'",
        "  -H 'Cookie: sid=abc'",
      ].join(' \\\n'),
    );
    expect(masked).toBe(false);
  });

  it('sends the form back and quotes what the shell would read', () => {
    const { command } = asCurl(
      { method: 'POST', url: "https://shop.test/o'rder" },
      { post: { name: "O'Brien", items: { 0: 'a', 1: 'b' } }, cookie: { sid: 'abc', lang: 'ru' } },
    );
    expect(command).toBe(
      [
        "curl 'https://shop.test/o'\\''rder'",
        "  -H 'Cookie: sid=abc; lang=ru'",
        "  --data-urlencode 'name=O'\\''Brien'",
        "  --data-urlencode 'items[0]=a'",
        "  --data-urlencode 'items[1]=b'",
      ].join(' \\\n'),
    );
  });

  it('names a method curl would not guess and tells about masked values', () => {
    const { command, masked } = asCurl(
      { method: 'DELETE', url: 'https://shop.test/item/1' },
      { server: { HTTP_AUTHORIZATION: '***', CONTENT_TYPE: 'application/json' } },
    );
    expect(command).toBe(
      [
        "curl -X DELETE 'https://shop.test/item/1'",
        "  -H 'Authorization: ***'",
        "  -H 'Content-Type: application/json'",
      ].join(' \\\n'),
    );
    expect(masked).toBe(true);
  });
});

describe('withXdebug', () => {
  it('adds the Xdebug trigger to the query', () => {
    expect(withXdebug('https://shop.test/a')).toBe('https://shop.test/a?XDEBUG_SESSION=1');
    expect(withXdebug('https://shop.test/a?b=1#top')).toBe(
      'https://shop.test/a?b=1&XDEBUG_SESSION=1#top',
    );
    expect(withXdebug('https://shop.test/a?XDEBUG_SESSION=PHPSTORM')).toBe(
      'https://shop.test/a?XDEBUG_SESSION=PHPSTORM',
    );
  });
});
