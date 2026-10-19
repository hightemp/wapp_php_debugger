import type { Request, StreamEvent } from './api/types';

export interface FeedUpdate {
  items: Request[];
  /** Events were missed (resync): the feed must be reloaded. */
  reload: boolean;
}

/** Applies a stream event to the request feed (newest first). */
export function applyStreamEvent(items: Request[], event: StreamEvent): FeedUpdate {
  switch (event.event) {
    case 'request_created': {
      const created = event.request;
      if (!created || items.some((item) => item.id === created.id)) {
        return { items, reload: false };
      }
      return { items: [created, ...items], reload: false };
    }
    case 'request_updated': {
      const updated = event.request;
      if (!updated) {
        return { items, reload: false };
      }
      return {
        items: items.map((item) => (item.id === updated.id ? updated : item)),
        reload: false,
      };
    }
    case 'resync':
      return { items, reload: true };
    default:
      return { items, reload: false };
  }
}
