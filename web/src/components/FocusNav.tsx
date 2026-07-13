export interface FocusNavProps {
  index: number;
  total: number;
  // True while the deck's auto-advance is armed: the Next control fills over 450ms
  // (a static "next in a moment" note stands in under reduced motion).
  advancing: boolean;
  onPrev: () => void;
  onNext: () => void;
}

// FocusNav is the deck's prev/next control. Its real buttons blur a focused input
// on click, committing the field before the step unmounts. Advancing off the last
// step lands on the review summary (index === total).
export function FocusNav({ index, total, advancing, onPrev, onNext }: FocusNavProps) {
  const onSummary = index >= total;
  return (
    <div className="focus-nav">
      <button type="button" className="focus-nav-btn" onClick={onPrev} disabled={index <= 0}>
        ‹ Back
      </button>
      <button
        type="button"
        className={`focus-nav-btn primary${advancing ? ' advancing' : ''}`}
        onClick={onNext}
        disabled={onSummary}
      >
        {index >= total - 1 ? 'Review' : 'Next ›'}
        {advancing && (
          <>
            <span className="focus-advance-fill" aria-hidden />
            <span className="focus-advance-text" aria-hidden>
              next in a moment
            </span>
          </>
        )}
      </button>
    </div>
  );
}
