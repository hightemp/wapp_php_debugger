// What a request did and when: the timeline and the SQL of its messages.
import type { Message } from './api/types';

export type TimelineKind = 'sql' | 'http' | 'timer' | 'event';

export interface TimelineRow {
  id: number;
  kind: TimelineKind;
  label: string;
  level: Message['level'];
  /** Since the start of the request, ms */
  startMs: number;
  durationMs: number;
  startPercent: number;
  widthPercent: number;
}

function kindOf(message: Message): TimelineKind {
  if (message.type === 'timer') {
    return 'timer';
  }
  if (message.tags.includes('sql')) {
    return 'sql';
  }
  if (message.tags.includes('http')) {
    return 'http';
  }
  return 'event';
}

/**
 * Events on the scale of the request: an event with a duration was written when it ended, so it
 * spans [ts - duration, ts]; others are points. A request still running is as long as its last event.
 */
export function timeline(
  request: { started_at: number; duration_ms?: number | null },
  messages: readonly Message[],
): { rows: TimelineRow[]; totalMs: number } {
  const lastEnd = Math.max(0, ...messages.map((message) => message.ts - request.started_at));
  const totalMs = request.duration_ms ?? lastEnd;
  const scale = totalMs > 0 ? 100 / totalMs : 0;
  const rows = messages.map((message): TimelineRow => {
    const end = Math.min(totalMs, Math.max(0, message.ts - request.started_at));
    const start = Math.max(0, end - (message.duration_ms ?? 0));
    const durationMs = end - start;
    return {
      id: message.id,
      kind: kindOf(message),
      label: message.message,
      level: message.level,
      startMs: start,
      durationMs,
      startPercent: start * scale,
      widthPercent: durationMs * scale,
    };
  });
  return { rows, totalMs };
}

export interface SqlQuery {
  id: number;
  sql: string;
  durationMs?: number | null;
  /** How many times this query (the same text) ran in the request */
  repeats: number;
  message: Message;
}

/** The same query whatever its line breaks and indentation. */
function normalize(sql: string): string {
  return sql.replace(/\s+/g, ' ').trim();
}

/** SQL queries of a request with their time, and the ones run again and again (an N+1 sign). */
export function sqlSummary(messages: readonly Message[]) {
  const sql = messages.filter((message) => message.tags.includes('sql'));
  const groups = new Map<string, { count: number; totalMs: number }>();
  for (const message of sql) {
    const key = normalize(message.message);
    const group = groups.get(key) ?? { count: 0, totalMs: 0 };
    group.count += 1;
    group.totalMs += message.duration_ms ?? 0;
    groups.set(key, group);
  }
  const queries: SqlQuery[] = sql.map((message) => ({
    id: message.id,
    sql: message.message,
    durationMs: message.duration_ms,
    repeats: groups.get(normalize(message.message))?.count ?? 1,
    message,
  }));
  const repeated = [...groups.entries()]
    .filter(([, group]) => group.count > 1)
    .map(([text, group]) => ({ sql: text, count: group.count, totalMs: group.totalMs }))
    .sort((a, b) => b.count - a.count || b.totalMs - a.totalMs);
  const slowest = queries.reduce<SqlQuery | undefined>(
    (best, query) => ((query.durationMs ?? -1) > (best?.durationMs ?? -1) ? query : best),
    undefined,
  );
  return {
    count: queries.length,
    totalMs: queries.reduce((sum, query) => sum + (query.durationMs ?? 0), 0),
    queries,
    repeated,
    slowest,
  };
}
