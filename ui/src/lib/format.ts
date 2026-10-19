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

/** Feed row title: method and path for a web request, CLI for a console run. */
export function requestTitle(request: Pick<Request, 'sapi' | 'method' | 'url'>): string {
  if (request.sapi === 'cli') {
    return 'CLI';
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
