import { describe, expect, it } from 'vitest';
import type { Request } from './api/types';
import { applyStreamEvent } from './requests';

function request(id: number, overrides: Partial<Request> = {}): Request {
  return {
    id,
    project_id: 1,
    server_id: 1,
    request_key: `k${id}`,
    format_version: 1,
    sapi: 'web',
    is_ajax: false,
    started_at: id * 1000,
    finished: true,
    message_count: 1,
    error_count: 0,
    warning_count: 0,
    received_at: id * 1000,
    pinned: false,
    ...overrides,
  };
}

describe('applyStreamEvent', () => {
  it('puts a created request on top', () => {
    const result = applyStreamEvent([request(1)], {
      event: 'request_created',
      request: request(2),
    });
    expect(result).toEqual({ items: [request(2), request(1)], reload: false });
  });

  it('does not duplicate a request that is already listed', () => {
    const result = applyStreamEvent([request(1)], {
      event: 'request_created',
      request: request(1),
    });
    expect(result.items).toHaveLength(1);
  });

  it('replaces an updated request in place', () => {
    const updated = request(1, { error_count: 3 });
    const result = applyStreamEvent([request(2), request(1)], {
      event: 'request_updated',
      request: updated,
    });
    expect(result.items).toEqual([request(2), updated]);
  });

  it('asks for a reload on resync', () => {
    expect(applyStreamEvent([request(1)], { event: 'resync' })).toEqual({
      items: [request(1)],
      reload: true,
    });
  });

  it('ignores events it does not handle yet', () => {
    const items = [request(1)];
    expect(
      applyStreamEvent(items, { event: 'messages_added', request_id: 1, count: 2 }).items,
    ).toBe(items);
  });
});
