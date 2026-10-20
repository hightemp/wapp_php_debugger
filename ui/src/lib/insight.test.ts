import { describe, expect, it } from 'vitest';
import { sampleMessage } from '../test/transport';
import { sqlSummary, timeline } from './insight';

const start = 1792400001000;

describe('timeline', () => {
  it('places timed events as spans that end at their ts, others as points', () => {
    const messages = [
      sampleMessage(2, { ts: start + 12, tags: ['sql'], message: 'SELECT 1', duration_ms: 4 }),
      sampleMessage(3, { ts: start + 50, message: 'Loading' }),
      sampleMessage(4, { ts: start + 150, type: 'timer', message: 'render', duration_ms: 100 }),
      sampleMessage(5, {
        ts: start + 160,
        tags: ['http'],
        message: 'GET https://pay.test',
        duration_ms: 20,
        level: 'warning',
      }),
    ];
    const { rows, totalMs } = timeline({ started_at: start, duration_ms: 200 }, messages);
    expect(totalMs).toBe(200);
    expect(rows.map((row) => [row.kind, row.startMs, row.durationMs])).toEqual([
      ['sql', 8, 4],
      ['event', 50, 0],
      ['timer', 50, 100],
      ['http', 140, 20],
    ]);
    expect(rows[2]).toMatchObject({ startPercent: 25, widthPercent: 50, id: 4 });
  });

  it('measures a request without an end by its last event, and keeps spans inside', () => {
    const messages = [
      sampleMessage(2, { ts: start - 5, message: 'clock skew' }),
      sampleMessage(3, { ts: start + 30, type: 'timer', message: 'early', duration_ms: 80 }),
    ];
    const { rows, totalMs } = timeline({ started_at: start, duration_ms: null }, messages);
    expect(totalMs).toBe(30);
    expect(rows[0]).toMatchObject({ startMs: 0, startPercent: 0 });
    // started before the request did: clipped to its start
    expect(rows[1]).toMatchObject({ startMs: 0, durationMs: 30, widthPercent: 100 });
  });
});

describe('sqlSummary', () => {
  it('counts queries, their time and the repeated ones (N+1)', () => {
    const messages = [
      sampleMessage(2, {
        tags: ['sql'],
        message: 'SELECT * FROM orders WHERE user_id = ?',
        duration_ms: 4.25,
      }),
      sampleMessage(3, {
        tags: ['sql'],
        message: 'SELECT * FROM items\n  WHERE order_id = ?',
        duration_ms: 1.5,
      }),
      sampleMessage(4, {
        tags: ['sql'],
        message: 'SELECT * FROM items WHERE order_id = ?',
        duration_ms: 1.25,
      }),
      sampleMessage(5, { tags: ['http'], message: 'GET x', duration_ms: 100 }),
      sampleMessage(6, { tags: ['sql'], message: 'UPDATE orders SET seen = 1' }),
    ];
    const summary = sqlSummary(messages);
    expect(summary.count).toBe(4);
    expect(summary.totalMs).toBe(7);
    expect(summary.queries.map((query) => [query.id, query.repeats])).toEqual([
      [2, 1],
      [3, 2],
      [4, 2],
      [6, 1],
    ]);
    expect(summary.repeated).toEqual([
      { sql: 'SELECT * FROM items WHERE order_id = ?', count: 2, totalMs: 2.75 },
    ]);
    expect(summary.slowest?.id).toBe(2);
  });

  it('is empty without SQL', () => {
    expect(sqlSummary([sampleMessage(1)])).toMatchObject({ count: 0, totalMs: 0, repeated: [] });
  });
});
