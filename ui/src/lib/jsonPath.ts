// Paths in a JSON value of the context: what search finds and what "copy path" gives.

/** A key of a path for sets: segments joined by a character that keys do not hold. */
export function pathKey(path: readonly string[]): string {
  return path.join('\u0000');
}

/** The path the PHP way: ['user']['id'], array indexes as numbers: ['rows'][0]. */
export function phpPath(path: readonly string[], indexes: readonly boolean[]): string {
  return path
    .map((segment, i) =>
      indexes[i] ? `[${segment}]` : `['${segment.replace(/\\/g, '\\\\').replace(/'/g, "\\'")}']`,
    )
    .join('');
}

function scalarText(value: unknown): string {
  return value === null ? 'null' : String(value);
}

/**
 * Nodes whose key or value holds the query (case-insensitive), with every node on the way to them,
 * so the tree can show only these and open them.
 */
export function matchingPaths(value: unknown, query: string): Set<string> {
  const needle = query.toLowerCase();
  const found = new Set<string>();
  const walk = (node: unknown, path: string[]): boolean => {
    let hit = false;
    if (node !== null && typeof node === 'object') {
      for (const [key, child] of Object.entries(node as Record<string, unknown>)) {
        const childPath = [...path, key];
        const own = key.toLowerCase().includes(needle);
        const below = walk(child, childPath);
        if (own || below) {
          found.add(pathKey(childPath));
          hit = true;
        }
      }
      return hit;
    }
    return path.length > 0 && scalarText(node).toLowerCase().includes(needle);
  };
  walk(value, []);
  return found;
}

/** What "copy value" puts on the clipboard: a string as it is, anything else as JSON. */
export function copyText(value: unknown): string {
  return typeof value === 'string' ? value : JSON.stringify(value, null, 2);
}
