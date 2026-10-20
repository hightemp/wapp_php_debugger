// Short notices in the corner of the window: a copied value, a failing request the feed does not
// show. They go by themselves after a while.

export interface Toast {
  id: number;
  text: string;
  kind: 'info' | 'error';
  link?: { label: string; href: string };
}

const lifetimeMs = 5000;
const visibleMax = 4;
let next = 1;
// Timers are bookkeeping, not state the page shows: no reactive map is needed
// eslint-disable-next-line svelte/prefer-svelte-reactivity
const timers = new Map<number, ReturnType<typeof setTimeout>>();
let items = $state<Toast[]>([]);

export const toasts = {
  get items(): readonly Toast[] {
    return items;
  },
  push(toast: Omit<Toast, 'id' | 'kind'> & { kind?: Toast['kind'] }): void {
    const id = next++;
    const item: Toast = { ...toast, kind: toast.kind ?? 'info', id };
    items = [...items, item].slice(-visibleMax);
    timers.set(
      id,
      setTimeout(() => toasts.close(id), lifetimeMs),
    );
  },
  close(id: number): void {
    clearTimeout(timers.get(id));
    timers.delete(id);
    items = items.filter((toast) => toast.id !== id);
  },
  clear(): void {
    for (const id of [...timers.keys()]) {
      toasts.close(id);
    }
    items = [];
  },
};
