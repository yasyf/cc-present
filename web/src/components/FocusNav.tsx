export interface FocusNavProps {
  index: number;
  total: number;
  onPrev: () => void;
  onNext: () => void;
}

// FocusNav is the deck's prev/next control. Its real buttons blur a focused input
// on click, committing the field before the step unmounts. Advancing off the last
// step lands on the review summary (index === total).
export function FocusNav({ index, total, onPrev, onNext }: FocusNavProps) {
  const onSummary = index >= total;
  return (
    <div className="focus-nav">
      <button type="button" className="focus-nav-btn" onClick={onPrev} disabled={index <= 0}>
        ‹ Back
      </button>
      <button type="button" className="focus-nav-btn primary" onClick={onNext} disabled={onSummary}>
        {index >= total - 1 ? 'Review' : 'Next ›'}
      </button>
    </div>
  );
}
