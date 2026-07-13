// The pre-caught-up placeholder: a masthead, stats stub, and three anatomical
// card stubs (rail, title bar, text bars, verdict pair), held until SSE replay.
export function BoardSkeleton() {
  return (
    <div className="skeleton" aria-hidden>
      <div className="skeleton-masthead" />
      <div className="skeleton-stats" />
      {[0, 1, 2].map((i) => (
        <div className="skeleton-card" key={i}>
          <div className="skeleton-rail" />
          <div className="skeleton-card-body">
            <div className="skeleton-title" />
            <div className="skeleton-line" />
            <div className="skeleton-line skeleton-line-short" />
            <div className="skeleton-verdicts">
              <div className="skeleton-verdict" />
              <div className="skeleton-verdict" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
