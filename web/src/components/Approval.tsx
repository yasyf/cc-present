import { useContext, useRef, useState } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Approval as ApprovalBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { verdictToggle } from '../decide';
import { useDecidable } from '../keyboard';
import { useActiveBlock } from '../activeBlock';
import { Button } from './Button';
import { Mark } from './Mark';
import { DetailDisclosure } from './Detail';
import { FeedbackThread } from './FeedbackThread';
import type { FeedbackHandle } from './FeedbackThread';
import { CommentChip } from './CommentChip';
import { FocusStepContext } from './focusStep';
import { useThreadHost } from './threadHost';

export function Approval({ block, interactions }: { block: ApprovalBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const focus = useContext(FocusStepContext);
  const rail = useThreadHost() === 'rail';
  const { requestCompose } = useActiveBlock();
  const suppressPrompt = focus?.headlineId === block.id;
  const feedbackRef = useRef<FeedbackHandle>(null);
  const [composing, setComposing] = useState(false);
  const locked = closed || readOnly;

  const verdict = interactions.decisions[block.id]?.verdict;
  const allowFeedback = block.allowFeedback ?? true;
  const feedback = interactions.feedback[block.id] ?? [];
  const replies = interactions.replies[block.id] ?? [];

  function choose(next: 'approved' | 'rejected') {
    post({ type: 'decision.created', blockId: block.id, verdict: verdictToggle(verdict, next) });
  }

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'approval',
    disabled: locked,
    verdict: choose,
    clear: () => {
      if (verdict) post({ type: 'decision.created', blockId: block.id, verdict: 'cleared' });
    },
    engage: !allowFeedback ? undefined : rail ? requestCompose : () => feedbackRef.current?.open(),
  });

  const commentCount = feedback.length + replies.length;

  return (
    <div className="approval" ref={ref} data-kbd-cursor={cursor || undefined} data-composing={composing || undefined}>
      {!suppressPrompt && block.prompt && <p className="approval-prompt">{block.prompt}</p>}
      {block.detail && <DetailDisclosure detail={block.detail} interactions={interactions} />}
      <div className="decision-bar" role="radiogroup" aria-label={suppressPrompt && block.prompt ? block.prompt : 'verdict'}>
        <Button
          variant="ghost"
          size="lg"
          role="radio"
          aria-checked={verdict === 'approved'}
          disabled={locked}
          className={`verdict verdict-approve${verdict === 'approved' ? ' active' : ''}`}
          onClick={() => choose('approved')}
        >
          <span className="verdict-glyph" aria-hidden>
            {verdict === 'approved' ? <Mark kind="check" /> : '✓'}
          </span>
          Approve
        </Button>
        <Button
          variant="ghost"
          size="lg"
          role="radio"
          aria-checked={verdict === 'rejected'}
          disabled={locked}
          className={`verdict verdict-reject${verdict === 'rejected' ? ' active' : ''}`}
          onClick={() => choose('rejected')}
        >
          <span className="verdict-glyph" aria-hidden>
            {verdict === 'rejected' ? <Mark kind="cross" /> : '✕'}
          </span>
          Reject
        </Button>
        {rail && (allowFeedback || commentCount > 0) && (
          <CommentChip blockId={block.id} count={commentCount} addLabel="Add feedback" locked={!allowFeedback} />
        )}
      </div>

      {!rail && (
        <FeedbackThread
          ref={feedbackRef}
          blockId={block.id}
          feedback={feedback}
          replies={replies}
          locked={locked || !allowFeedback}
          addLabel="Add feedback"
          placeholder="Add feedback for the agent…"
          onComposingChange={setComposing}
        />
      )}
    </div>
  );
}
