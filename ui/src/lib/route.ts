// The selection lives in the URL hash so a request or a message can be shared as a link:
// #/requests/42/messages/1001

export interface Route {
  requestId?: number;
  messageId?: number;
}

const pattern = /^#\/requests\/(\d+)(?:\/messages\/(\d+))?\/?$/;

export function parseRoute(hash: string): Route {
  const [, request, message] =
    pattern.exec(hash) ?? /^#\/requests\/(\d+)(?:\/.*)?$/.exec(hash) ?? [];
  if (!request) {
    return {};
  }
  const route: Route = { requestId: Number(request) };
  if (message) {
    route.messageId = Number(message);
  }
  return route;
}

export function formatRoute(route: Route): string {
  if (route.requestId === undefined) {
    return '#/';
  }
  const request = `#/requests/${route.requestId}`;
  return route.messageId === undefined ? request : `${request}/messages/${route.messageId}`;
}
