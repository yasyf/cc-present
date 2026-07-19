// resolveColor forces a token to a concrete rgb through a probe span (attached by
// the caller for the read): a custom property's own computed value keeps its
// unresolved light-dark()/color-mix() string, which mermaid and the chart palette reject.
export function resolveColor(probe: HTMLElement, token: string, fallback: string): string {
  probe.style.color = `var(${token}, ${fallback})`;
  return getComputedStyle(probe).color || fallback;
}
