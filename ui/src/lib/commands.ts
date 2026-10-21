// Commands from the keyboard and the palette to the parts of the UI that carry them out.
export type Command =
  | { type: 'filter'; text: string }
  | { type: 'focus-filter' }
  | { type: 'focus-projects' }
  | { type: 'open-servers' }
  | { type: 'add-project' }
  /** A built-in theme or custom:<id>. */
  | { type: 'theme'; theme: string };

const listeners = new Set<(command: Command) => void>();

export function emitCommand(command: Command): void {
  for (const listener of [...listeners]) {
    listener(command);
  }
}

/** Listens for commands; the returned function stops it (fits an $effect). */
export function onCommand(listener: (command: Command) => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
