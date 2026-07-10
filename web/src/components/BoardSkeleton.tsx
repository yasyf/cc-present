// The pre-caught-up placeholder: a shimmer header and three card stand-ins,
// shown until the SSE replay flushes so live content never flickers over it.
export function BoardSkeleton() {
  return (
    <div className="skeleton" aria-hidden>
      <div className="skeleton-header" />
      <div className="skeleton-card" />
      <div className="skeleton-card" />
      <div className="skeleton-card" />
    </div>
  );
}
