// Repeating a request: as a curl command built from what the library logged ($_SERVER headers,
// $_COOKIE, $_POST), or with the Xdebug trigger. The raw body is not logged, so a JSON body is lost.

export interface LoggedGlobals {
  server?: Record<string, unknown>;
  post?: Record<string, unknown>;
  cookie?: Record<string, unknown>;
}

/** The value Masker writes in place of secrets */
const maskedValue = '***';

// Headers curl sets by itself or that would break the repeat
const skipped = new Set(['HOST', 'CONNECTION', 'ACCEPT_ENCODING', 'CONTENT_LENGTH']);

export function shellQuote(text: string): string {
  return `'${text.replaceAll("'", "'\\''")}'`;
}

function headerName(key: string): string {
  return key
    .toLowerCase()
    .split('_')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join('-');
}

/** name=value pairs of a form the PHP way: items[0]=a */
function formPairs(values: Record<string, unknown>, prefix = ''): [string, string][] {
  return Object.entries(values).flatMap(([key, value]): [string, string][] => {
    const name = prefix === '' ? key : `${prefix}[${key}]`;
    if (value !== null && typeof value === 'object') {
      return formPairs(value as Record<string, unknown>, name);
    }
    return [[name, value === null || value === undefined ? '' : String(value)]];
  });
}

export function asCurl(
  request: { method?: string | null; url: string },
  globals: LoggedGlobals,
): { command: string; masked: boolean } {
  const headers: [string, string][] = [];
  let cookie: string | undefined;
  for (const [key, value] of Object.entries(globals.server ?? {})) {
    if (typeof value !== 'string') {
      continue;
    }
    if (key === 'HTTP_COOKIE') {
      cookie = value;
    } else if (key.startsWith('HTTP_') && !skipped.has(key.slice(5))) {
      headers.push([headerName(key.slice(5)), value]);
    } else if (key === 'CONTENT_TYPE') {
      headers.push(['Content-Type', value]);
    }
  }
  if (cookie === undefined && globals.cookie && Object.keys(globals.cookie).length > 0) {
    cookie = Object.entries(globals.cookie)
      .map(([name, value]) => `${name}=${String(value)}`)
      .join('; ');
  }
  if (cookie !== undefined) {
    headers.push(['Cookie', cookie]);
  }
  const form = formPairs(globals.post ?? {});
  // curl sends --data-urlencode as a form itself
  const sent = form.length > 0 ? headers.filter(([name]) => name !== 'Content-Type') : headers;

  const method = (request.method ?? 'GET').toUpperCase();
  const implied = form.length > 0 ? 'POST' : 'GET';
  const lines = [`curl ${method === implied ? '' : `-X ${method} `}${shellQuote(request.url)}`];
  for (const [name, value] of sent) {
    lines.push(`  -H ${shellQuote(`${name}: ${value}`)}`);
  }
  for (const [name, value] of form) {
    lines.push(`  --data-urlencode ${shellQuote(`${name}=${value}`)}`);
  }
  const values = [...sent.map(([, value]) => value), ...form.map(([, value]) => value)];
  return {
    command: lines.join(' \\\n'),
    masked: values.some((value) => value.includes(maskedValue)),
  };
}

/** The URL with the trigger Xdebug 3 starts a debug session on; a trigger already there stays. */
export function withXdebug(url: string): string {
  const [beforeHash, hash] = url.split(/#(.*)/s, 2);
  const base = beforeHash ?? url;
  if (/[?&]XDEBUG_SESSION(_START)?=/.test(base)) {
    return url;
  }
  const joined = `${base}${base.includes('?') ? '&' : '?'}XDEBUG_SESSION=1`;
  return hash === undefined ? joined : `${joined}#${hash}`;
}
