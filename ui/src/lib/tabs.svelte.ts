// Requests opened one after another stay as tabs: to come back to one or compare two.

export interface Tab {
  id: number;
  title?: string;
}

const tabsMax = 8;
let items = $state<Tab[]>([]);

export const openTabs = {
  get items(): readonly Tab[] {
    return items;
  },
  /** Adds a tab (the oldest other one goes when there are too many). */
  open(id: number): void {
    if (items.some((tab) => tab.id === id)) {
      return;
    }
    const added = [...items, { id }];
    items = added.length > tabsMax ? added.slice(added.length - tabsMax) : added;
  },
  name(id: number, title: string): void {
    items = items.map((tab) => (tab.id === id ? { ...tab, title } : tab));
  },
  /** Removes a tab and gives its neighbour: the next one, or the previous at the end. */
  close(id: number): number | undefined {
    const index = items.findIndex((tab) => tab.id === id);
    if (index < 0) {
      return undefined;
    }
    items = items.filter((tab) => tab.id !== id);
    return (items[index] ?? items[index - 1])?.id;
  },
  clear(): void {
    items = [];
  },
};
