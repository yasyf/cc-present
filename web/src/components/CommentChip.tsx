import { useActiveBlock } from '../activeBlock';
import { Button } from './Button';

// CommentChip is the inline stand-in for a block's feedback thread once it moves
// to the margin rail: a count of notes plus replies, or the add affordance when
// empty. A click pins the block and opens the panel so its thread and composer
// surface in the rail (or the comments sheet below the rail breakpoint).
export function CommentChip({ blockId, count, addLabel }: { blockId: string; count: number; addLabel: string }) {
  const { pin } = useActiveBlock();
  const label = count > 0 ? `${count} ${count === 1 ? 'comment' : 'comments'}` : addLabel;
  return (
    <div className="comment-chip-row">
      <Button
        variant="ghost"
        size="sm"
        className="comment-chip"
        data-count={count > 0 || undefined}
        aria-label={count > 0 ? `${label} — open in margin` : addLabel}
        onClick={() => pin(blockId)}
      >
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" aria-hidden focusable="false">
          <path
            d="M4 5h16v11H8l-4 4V5z"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinejoin="round"
          />
        </svg>
        {label}
      </Button>
    </div>
  );
}
