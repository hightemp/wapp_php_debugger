import { describe, expect, it } from 'vitest';
import { formatRoute, parseRoute } from './route';

describe('route', () => {
  it('parses the selection from the hash', () => {
    expect(parseRoute('')).toEqual({});
    expect(parseRoute('#/')).toEqual({});
    expect(parseRoute('#/requests/42')).toEqual({ requestId: 42 });
    expect(parseRoute('#/requests/42/messages/1001')).toEqual({ requestId: 42, messageId: 1001 });
  });

  it('ignores anything it does not understand', () => {
    expect(parseRoute('#/requests/abc')).toEqual({});
    expect(parseRoute('#/requests/-1')).toEqual({});
    expect(parseRoute('#/settings')).toEqual({});
    expect(parseRoute('#/requests/42/messages/x')).toEqual({ requestId: 42 });
  });

  it('formats the selection back', () => {
    expect(formatRoute({})).toBe('#/');
    expect(formatRoute({ requestId: 42 })).toBe('#/requests/42');
    expect(formatRoute({ requestId: 42, messageId: 1001 })).toBe('#/requests/42/messages/1001');
    expect(formatRoute({ messageId: 1001 })).toBe('#/');
  });
});
