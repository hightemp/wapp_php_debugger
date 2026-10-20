import { describe, expect, it } from 'vitest';
import { parseFeedQuery, suggestions } from './feedQuery';

describe('parseFeedQuery', () => {
  it('turns conditions into feed parameters and the rest into the text search', () => {
    expect(
      parseFeedQuery('level:error tag:sql method:post status:5xx duration>500 checkout failed'),
    ).toEqual({
      params: {
        level: 'error',
        tag: 'sql',
        method: 'POST',
        status: '5xx',
        min_duration_ms: 500,
        q: 'checkout failed',
      },
      invalid: [],
    });
  });

  it('reads durations with units and yes/no flags', () => {
    expect(parseFeedQuery('duration>1.5s ajax:yes pinned:yes sapi:cli').params).toEqual({
      min_duration_ms: 1500,
      is_ajax: true,
      pinned: true,
      sapi: 'cli',
    });
    expect(parseFeedQuery('duration>=250ms ajax:no').params).toEqual({
      min_duration_ms: 250,
      is_ajax: false,
    });
  });

  it('searches a URL part with url:, wildcards dropped', () => {
    expect(parseFeedQuery('url:/api/*').params).toEqual({ q: '/api/' });
    expect(parseFeedQuery('url:/api/* timeout').params).toEqual({ q: '/api/ timeout' });
  });

  it('keeps what it cannot read as invalid, not as search words', () => {
    const parsed = parseFeedQuery('level:fatal status:6xx duration>fast colour:red ok');
    expect(parsed.params).toEqual({ q: 'ok' });
    expect(parsed.invalid).toEqual(['level:fatal', 'status:6xx', 'duration>fast', 'colour:red']);
  });

  it('is empty for an empty query', () => {
    expect(parseFeedQuery('   ')).toEqual({ params: {}, invalid: [] });
  });
});

describe('suggestions', () => {
  it('offers keys for a started word and values after the colon', () => {
    expect(suggestions('checkout le').map((s) => s.insert)).toEqual(['level:']);
    expect(suggestions('level:').map((s) => s.insert)).toEqual(['level:error', 'level:warning']);
    expect(suggestions('status:5').map((s) => s.insert)).toEqual(['status:5xx']);
    expect(suggestions('tag:', ['sql', 'http']).map((s) => s.insert)).toEqual([
      'tag:sql',
      'tag:http',
    ]);
    expect(suggestions('checkout ')).toEqual([]);
  });
});
