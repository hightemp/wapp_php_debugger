// The address the library pushes to: the receiver's port on the host the viewer is opened at
// when the receiver listens on every interface.
export function pushUrl(ingestAddr: string | null | undefined, pageHost: string): string {
  if (!ingestAddr) {
    return 'http://VIEWER_HOST:8821/ingest/v2';
  }
  const colon = ingestAddr.lastIndexOf(':');
  const host = ingestAddr.slice(0, colon).replace(/^\[|\]$/g, '');
  const port = ingestAddr.slice(colon + 1);
  const everywhere = host === '' || host === '0.0.0.0' || host === '::';
  const shown = everywhere ? pageHost || 'localhost' : host.includes(':') ? `[${host}]` : host;
  return `http://${shown}:${port}/ingest/v2`;
}

/** Keys for dws_debugger.json that send the logs of the project to the viewer. */
export function libraryConfig(url: string, token: string): string {
  return `"push_url": "${url}",\n"push_token": "${token}"`;
}
