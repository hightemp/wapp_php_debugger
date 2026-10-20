// What differs between two values of the context ($_SESSION of two requests, say), leaf by leaf.
import { phpPath } from './jsonPath';

export interface DiffRow {
  /** The path the PHP way: ['user']['cart'][2] */
  path: string;
  kind: 'added' | 'removed' | 'changed';
  /** JSON of the value before and after */
  before?: string;
  after?: string;
}

interface Leaf {
  keys: string[];
  indexes: boolean[];
  json: string;
}

/** Leaves of a value: scalars and empty arrays or objects, by their path. */
function leaves(value: unknown): Map<string, Leaf> {
  const found = new Map<string, Leaf>();
  const walk = (node: unknown, keys: string[], indexes: boolean[]) => {
    const isArray = Array.isArray(node);
    const entries =
      node !== null && typeof node === 'object'
        ? Object.entries(node as Record<string, unknown>)
        : [];
    if (entries.length === 0) {
      if (keys.length > 0) {
        found.set(phpPath(keys, indexes), { keys, indexes, json: JSON.stringify(node) ?? 'null' });
      }
      return;
    }
    for (const [key, child] of entries) {
      walk(child, [...keys, key], [...indexes, isArray]);
    }
  };
  if (value !== undefined) {
    walk(value, [], []);
  }
  return found;
}

/** Paths in their natural order: keys by name, array indexes by number. */
function byPath(a: Leaf, b: Leaf): number {
  for (let i = 0; i < Math.min(a.keys.length, b.keys.length); i++) {
    const [x, y] = [a.keys[i] ?? '', b.keys[i] ?? ''];
    if (x !== y) {
      return a.indexes[i] && b.indexes[i] ? Number(x) - Number(y) : x < y ? -1 : 1;
    }
  }
  return a.keys.length - b.keys.length;
}

export function diffValues(before: unknown, after: unknown): DiffRow[] {
  const old = leaves(before);
  const now = leaves(after);
  const all = new Map([...now, ...old]);
  return [...all.entries()]
    .sort(([, a], [, b]) => byPath(a, b))
    .flatMap(([path]): DiffRow[] => {
      const was = old.get(path);
      const is = now.get(path);
      if (!was && is) {
        return [{ path, kind: 'added', after: is.json }];
      }
      if (was && !is) {
        return [{ path, kind: 'removed', before: was.json }];
      }
      if (was && is && was.json !== is.json) {
        return [{ path, kind: 'changed', before: was.json, after: is.json }];
      }
      return [];
    });
}
