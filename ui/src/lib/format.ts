import type { Request } from './api/types';
import { translate, type Locale } from './i18n/messages';

export function formatDuration(ms: number | null | undefined, locale: Locale): string {
  if (ms === null || ms === undefined) {
    return '—';
  }
  if (ms < 1) {
    return translate(locale, 'duration.belowMs');
  }
  if (ms < 1000) {
    return translate(locale, 'duration.ms', { value: Math.round(ms) });
  }
  const seconds = new Intl.NumberFormat(locale, { maximumFractionDigits: 2 }).format(ms / 1000);
  return translate(locale, 'duration.s', { value: seconds });
}

export function formatTime(ts: number): string {
  const date = new Date(ts);
  const pad = (value: number, size = 2) => String(value).padStart(size, '0');
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(), 3)}`;
}

/** Feed row title: method and path for a web request, the script and its arguments for a console run. */
export function requestTitle(
  request: Pick<Request, 'sapi'> & Partial<Pick<Request, 'method' | 'url' | 'argv'>>,
): string {
  if (request.sapi === 'cli') {
    const [script, ...args] = request.argv ?? [];
    // Logs of library v1 carry no command line
    return script ? [script.split(/[\\/]/).pop() ?? script, ...args].join(' ') : 'CLI';
  }
  const url = request.url ?? '';
  let path = url;
  try {
    const parsed = new URL(url);
    path = parsed.pathname + parsed.search;
  } catch {
    // v1 writes the URL as is; an unparsable one is shown whole
  }
  return request.method ? `${request.method} ${decodeReadable(path)}` : decodeReadable(path);
}

export type OutcomeKind = 'ok' | 'warn' | 'bad';

/** How a request ended, for its color: errors, 5xx, a failed exit or no end are bad; warnings and 4xx warn. */
export function outcomeKind(
  request: Pick<Request, 'finished' | 'error_count' | 'warning_count' | 'status_code'> & {
    exit_code?: number | null;
  },
): OutcomeKind {
  const status = request.status_code ?? 0;
  const failedExit =
    request.exit_code !== null && request.exit_code !== undefined && request.exit_code !== 0;
  if (!request.finished || request.error_count > 0 || status >= 500 || failedExit) {
    return 'bad';
  }
  if (request.warning_count > 0 || status >= 400) {
    return 'warn';
  }
  return 'ok';
}

/**
 * Decodes %XX for reading. Each run of escape sequences is decoded separately:
 * library v1 truncates URLs to 300 bytes and may cut a character in half, leaving only that tail encoded.
 */
function decodeReadable(text: string): string {
  return text.replace(/(?:%[0-9a-fA-F]{2})+/g, (chunk) => {
    for (let end = chunk.length; end > 0; end -= 3) {
      try {
        return decodeURIComponent(chunk.slice(0, end)) + chunk.slice(end);
      } catch {
        // shorten the run until it is valid UTF-8
      }
    }
    return chunk;
  });
}

/** Clocks further apart than this make the order of requests across servers misleading. */
export const clockSkewLimitMs = 2000;

/** "+12" or "−3" (whole seconds) when a server clock is off by more than the limit. */
export function clockSkew(offsetMs: number | null | undefined): string | undefined {
  if (offsetMs === null || offsetMs === undefined || Math.abs(offsetMs) <= clockSkewLimitMs) {
    return undefined;
  }
  const seconds = Math.round(Math.abs(offsetMs) / 1000);
  return `${offsetMs > 0 ? '+' : '−'}${seconds}`;
}
