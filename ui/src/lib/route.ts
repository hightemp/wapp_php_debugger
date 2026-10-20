// The selection lives in the URL hash so a project, a request or a message can be shared as a link:
// #/projects/3/requests/42/messages/1001 (every part is optional from the right; the project part too).
// #/servers/2/… selects the feed of a server instead of a project.

export interface Route {
  projectId?: number;
  serverId?: number;
  requestId?: number;
  messageId?: number;
}

const pattern =
  /^#(?:\/(projects|servers)\/(\d+))?(?:\/requests\/(\d+)(?:\/messages\/(\d+))?)?(?:\/.*)?$/;

export function parseRoute(hash: string): Route {
  const [, scope, scopeId, request, message] = pattern.exec(hash) ?? [];
  const route: Route = {};
  if (scope === 'projects') {
    route.projectId = Number(scopeId);
  } else if (scope === 'servers') {
    route.serverId = Number(scopeId);
  }
  if (request) {
    route.requestId = Number(request);
    if (message) {
      route.messageId = Number(message);
    }
  }
  return route;
}

export function formatRoute(route: Route): string {
  let hash = '#';
  if (route.projectId !== undefined) {
    hash = `#/projects/${route.projectId}`;
  } else if (route.serverId !== undefined) {
    hash = `#/servers/${route.serverId}`;
  }
  if (route.requestId !== undefined) {
    hash += `/requests/${route.requestId}`;
    if (route.messageId !== undefined) {
      hash += `/messages/${route.messageId}`;
    }
  }
  return hash === '#' ? '#/' : hash;
}
