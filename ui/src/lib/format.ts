import type { Request } from './api/types';

export function formatDuration(ms: number | null | undefined): string {
  if (ms === null || ms === undefined) {
    return '—';
  }
  if (ms < 1) {
    return '<1 мс';
  }
  if (ms < 1000) {
    return `${Math.round(ms)} мс`;
  }
  return `${Number((ms / 1000).toFixed(2))} с`;
}

export function formatTime(ts: number): string {
  const date = new Date(ts);
  const pad = (value: number, size = 2) => String(value).padStart(size, '0');
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(), 3)}`;
}

/** Заголовок строки ленты: метод и путь для веб-запроса, CLI для консоли. */
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
    // v1 пишет URL как есть; непарсящийся показываем целиком
  }
  return request.method ? `${request.method} ${decodeReadable(path)}` : decodeReadable(path);
}

/**
 * Раскодирует %XX для чтения. Каждая цепочка escape-последовательностей раскодируется отдельно:
 * библиотека v1 обрезает URL до 300 байт и может разрезать символ — тогда остаётся только хвост.
 */
function decodeReadable(text: string): string {
  return text.replace(/(?:%[0-9a-fA-F]{2})+/g, (chunk) => {
    for (let end = chunk.length; end > 0; end -= 3) {
      try {
        return decodeURIComponent(chunk.slice(0, end)) + chunk.slice(end);
      } catch {
        // укорачиваем цепочку, пока она не станет корректным UTF-8
      }
    }
    return chunk;
  });
}

/** «1 ошибка», «2 ошибки», «5 ошибок». */
export function plural(count: number, forms: [one: string, few: string, many: string]): string {
  const tens = count % 100;
  const units = count % 10;
  const form =
    tens >= 11 && tens <= 14
      ? forms[2]
      : units === 1
        ? forms[0]
        : units >= 2 && units <= 4
          ? forms[1]
          : forms[2];
  return `${count} ${form}`;
}
