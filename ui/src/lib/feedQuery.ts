// The filter line of the feed: `level:error tag:sql method:POST status:5xx duration>500 ajax:yes
// sapi:cli pinned:yes url:/api/*`; the other words go to the text search.
import type { ListRequestsParams } from './api/client';

export type FeedParams = Omit<ListRequestsParams, 'cursor' | 'limit' | 'project_id' | 'server_id'>;

const yesNo: Record<string, boolean> = {
  yes: true,
  true: true,
  '1': true,
  no: false,
  false: false,
  '0': false,
};

/** Milliseconds from 500, 500ms, 1.5s. */
function duration(text: string): number | undefined {
  const match = /^(\d+(?:\.\d+)?)(ms|s)?$/i.exec(text);
  if (!match?.[1]) {
    return undefined;
  }
  const value = Number(match[1]);
  return match[2]?.toLowerCase() === 's' ? value * 1000 : value;
}

type Reader = (value: string, params: FeedParams) => boolean;

const keys: Record<string, Reader> = {
  level: (value, params) => {
    const level = value.toLowerCase();
    if (level !== 'error' && level !== 'warning') {
      return false;
    }
    params.level = level;
    return true;
  },
  tag: (value, params) => {
    params.tag = value;
    return value !== '';
  },
  method: (value, params) => {
    params.method = value.toUpperCase();
    return /^[A-Za-z]+$/.test(value);
  },
  status: (value, params) => {
    params.status = value.toLowerCase();
    return /^[1-5](\d\d|xx)$/i.test(value);
  },
  ajax: (value, params) => {
    params.is_ajax = yesNo[value.toLowerCase()];
    return params.is_ajax !== undefined;
  },
  pinned: (value, params) => {
    params.pinned = yesNo[value.toLowerCase()];
    return params.pinned !== undefined;
  },
  sapi: (value, params) => {
    const sapi = value.toLowerCase();
    if (sapi !== 'web' && sapi !== 'cli') {
      return false;
    }
    params.sapi = sapi;
    return true;
  },
};

/** Reads the filter line; conditions it cannot read are listed as invalid rather than searched for. */
export function parseFeedQuery(text: string): { params: FeedParams; invalid: string[] } {
  const params: FeedParams = {};
  const invalid: string[] = [];
  const words: string[] = [];
  for (const word of text
    .trim()
    .split(/\s+/)
    .filter((item) => item !== '')) {
    const longer = /^duration(>=?)(.+)$/i.exec(word);
    if (longer) {
      const ms = duration(longer[2] ?? '');
      if (ms === undefined) {
        invalid.push(word);
      } else {
        params.min_duration_ms = ms;
      }
      continue;
    }
    const pair = /^([a-z]+):(.*)$/i.exec(word);
    if (!pair) {
      words.push(word);
      continue;
    }
    const key = (pair[1] ?? '').toLowerCase();
    const value = pair[2] ?? '';
    if (key === 'url') {
      words.push(value.replace(/\*/g, ''));
      continue;
    }
    const reader = keys[key];
    const attempt: FeedParams = {};
    if (reader && reader(value, attempt)) {
      Object.assign(params, attempt);
    } else {
      invalid.push(word);
    }
  }
  const q = words.filter((word) => word !== '').join(' ');
  if (q !== '') {
    params.q = q;
  }
  return { params, invalid };
}

export interface Suggestion {
  /** The word that replaces the one being typed */
  insert: string;
  hint?: string;
}

const values: Record<string, string[]> = {
  level: ['error', 'warning'],
  status: ['2xx', '3xx', '4xx', '5xx'],
  method: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'],
  ajax: ['yes', 'no'],
  pinned: ['yes'],
  sapi: ['web', 'cli'],
};

const allKeys = [
  'level:',
  'tag:',
  'method:',
  'status:',
  'duration>',
  'ajax:',
  'sapi:',
  'pinned:',
  'url:',
];

/** Completions for the last word of the line; tags come from the caller (the tags seen so far). */
export function suggestions(text: string, tags: readonly string[] = []): Suggestion[] {
  const last = /(\S*)$/.exec(text)?.[1] ?? '';
  if (last === '') {
    return [];
  }
  const pair = /^([a-z]+):(.*)$/i.exec(last);
  if (pair) {
    const key = (pair[1] ?? '').toLowerCase();
    const typed = (pair[2] ?? '').toLowerCase();
    const options = key === 'tag' ? tags : (values[key] ?? []);
    return options
      .filter((option) => option.toLowerCase().startsWith(typed) && option.toLowerCase() !== typed)
      .map((option) => ({ insert: `${key}:${option}` }));
  }
  return allKeys
    .filter((key) => key.startsWith(last.toLowerCase()) && key !== last.toLowerCase())
    .map((key) => ({ insert: key }));
}
