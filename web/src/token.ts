// The page-URL bearer token. Off-loopback the daemon requires the token on every
// request, but an EventSource cannot set an Authorization header, so it rides in the
// page URL as `?token=<hex>` (contract.md "Authentication"). On loopback there is no
// token and every URL stays byte-identical.

let cached: string | null | undefined;

// currentToken reads and memoizes the page URL's `?token=` — the URL is fixed for the
// session, and the lazy read keeps `window` (absent under node tests) off import.
function currentToken(): string | null {
  if (cached === undefined) {
    cached = typeof window === 'undefined' ? null : new URLSearchParams(window.location.search).get('token');
  }
  return cached;
}

// withToken appends the page token to a request path, picking `?` or `&` by whether
// the path already has a query; with no token it returns the path unchanged.
export function withToken(path: string): string {
  const token = currentToken();
  if (!token) return path;
  const sep = path.includes('?') ? '&' : '?';
  return `${path}${sep}token=${encodeURIComponent(token)}`;
}

export function resetTokenForTest(): void {
  cached = undefined;
}
