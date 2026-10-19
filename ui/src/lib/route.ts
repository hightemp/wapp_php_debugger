// The selection lives in the URL hash so a project, a request or a message can be shared as a link:
// #/projects/3/requests/42/messages/1001 (every part is optional from the right; the project part too)

export interface Route {
  projectId?: number;
  requestId?: number;
  messageId?: number;
}

const pattern = /^#(?:\/projects\/(\d+))?(?:\/requests\/(\d+)(?:\/messages\/(\d+))?)?(?:\/.*)?$/;

export function parseRoute(hash: string): Route {
  const [, project, request, message] = pattern.exec(hash) ?? [];
  const route: Route = {};
  if (project) {
    route.projectId = Number(project);
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
  let hash = route.projectId === undefined ? '#' : `#/projects/${route.projectId}`;
  if (route.requestId !== undefined) {
    hash += `/requests/${route.requestId}`;
    if (route.messageId !== undefined) {
      hash += `/messages/${route.messageId}`;
    }
  }
  return hash === '#' ? '#/' : hash;
}
