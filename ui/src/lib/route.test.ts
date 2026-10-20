import { describe, expect, it } from 'vitest';
import { formatRoute, parseRoute } from './route';

describe('route', () => {
  it('parses the selection from the hash', () => {
    expect(parseRoute('')).toEqual({});
    expect(parseRoute('#/')).toEqual({});
    expect(parseRoute('#/requests/42')).toEqual({ requestId: 42 });
    expect(parseRoute('#/requests/42/messages/1001')).toEqual({ requestId: 42, messageId: 1001 });
  });

  it('keeps the project filter in front of the selection', () => {
    expect(parseRoute('#/projects/3')).toEqual({ projectId: 3 });
    expect(parseRoute('#/projects/3/requests/42/messages/1001')).toEqual({
      projectId: 3,
      requestId: 42,
      messageId: 1001,
    });
    expect(formatRoute({ projectId: 3 })).toBe('#/projects/3');
    expect(formatRoute({ projectId: 3, requestId: 42 })).toBe('#/projects/3/requests/42');
  });

  it('selects a server like a project; a project wins', () => {
    expect(parseRoute('#/servers/2')).toEqual({ serverId: 2 });
    expect(parseRoute('#/servers/2/requests/42')).toEqual({ serverId: 2, requestId: 42 });
    expect(formatRoute({ serverId: 2, requestId: 42 })).toBe('#/servers/2/requests/42');
    expect(formatRoute({ serverId: 2, projectId: 3 })).toBe('#/projects/3');
  });

  it('compares a request with another', () => {
    expect(parseRoute('#/projects/3/requests/42/compare/41')).toEqual({
      projectId: 3,
      requestId: 42,
      compareId: 41,
    });
    expect(formatRoute({ requestId: 42, compareId: 41 })).toBe('#/requests/42/compare/41');
    // A message and a comparison do not go together: the comparison wins
    expect(formatRoute({ requestId: 42, messageId: 7, compareId: 41 })).toBe(
      '#/requests/42/compare/41',
    );
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
