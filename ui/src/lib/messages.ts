// Every message of a request, page after page: a long CLI run has thousands.
import type { ListMessagesParams, Transport } from './api/client';
import type { Message } from './api/types';

const pageSize = 500;
const messagesMax = 100_000;

/**
 * Loads the messages page by page and reports the growing list; stops when current() says a newer
 * load (another request, a filter, a live update) took over.
 */
export async function loadAllMessages(
  transport: Transport,
  requestId: number,
  params: Omit<ListMessagesParams, 'limit' | 'cursor'>,
  onPage: (messages: Message[]) => void,
  current: () => boolean,
): Promise<void> {
  let page = await transport.listMessages(requestId, { ...params, limit: pageSize });
  if (!current()) {
    return;
  }
  let loaded = page.items;
  onPage(loaded);
  while (page.next_cursor && loaded.length < messagesMax) {
    page = await transport.listMessages(requestId, {
      ...params,
      limit: pageSize,
      cursor: page.next_cursor,
    });
    if (!current()) {
      return;
    }
    loaded = [...loaded, ...page.items];
    onPage(loaded);
  }
}
