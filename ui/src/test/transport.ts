// Test helpers: an in-memory Transport and sample DTOs.
import { vi } from 'vitest';
import type { Transport } from '../lib/api/client';
import type { MessageDetail, Request, RequestDetail, StreamEvent } from '../lib/api/types';

export function sampleRequest(id: number, overrides: Partial<Request> = {}): Request {
  return {
    id,
    project_id: 1,
    server_id: 1,
    request_key: `k${id}`,
    format_version: 2,
    sapi: 'web',
    method: 'GET',
    url: `https://shop.test/item/${id}`,
    is_ajax: false,
    status_code: 200,
    started_at: Date.UTC(2026, 9, 19, 12, 0, id),
    duration_ms: 40,
    finished: true,
    message_count: 5,
    error_count: 0,
    warning_count: 0,
    received_at: 0,
    ...overrides,
  };
}

export function sampleRequestDetail(
  id: number,
  overrides: Partial<RequestDetail> = {},
): RequestDetail {
  return { ...sampleRequest(id), lang: 'php', php_version: '8.3.12', children: [], ...overrides };
}

export function sampleMessage(id: number, overrides: Partial<MessageDetail> = {}): MessageDetail {
  return {
    id,
    request_id: 1,
    seq: id,
    ts: Date.UTC(2026, 9, 19, 12, 0, 0, id),
    type: 'message',
    level: 'info',
    message: `message ${id}`,
    tags: [],
    has_context: false,
    size_bytes: 10,
    ...overrides,
  };
}

/** A Transport whose methods are vi.fn() with empty answers; override what a test needs. */
export function fakeTransport(overrides: Partial<Transport> = {}) {
  let emit: (event: StreamEvent) => void = () => {};
  const unsubscribe = vi.fn();
  const transport = {
    getVersion: vi.fn(async () => ({ version: 'v0.0.0-test', commit: 'abc', date: '' })),
    listRequests: vi.fn(async () => ({ items: [], next_cursor: null })),
    getRequest: vi.fn(async (id: number) => sampleRequestDetail(id)),
    listMessages: vi.fn(async () => ({ items: [], next_cursor: null })),
    getMessage: vi.fn(async (id: number) => sampleMessage(id)),
    subscribe: vi.fn((onEvent: (event: StreamEvent) => void) => {
      emit = onEvent;
      return unsubscribe;
    }),
    ...overrides,
  } satisfies Transport;
  return { transport, unsubscribe, emit: (event: StreamEvent) => emit(event) };
}
