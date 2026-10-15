import { forwardRef, useImperativeHandle, useRef } from 'react';
import { useActiveBlock } from '../activeBlock';
import { useDraft } from '../drafts';
import { noteCopy } from '../threadFeed';
import type { Feedback, Reply } from '../events';
import { CommentChip } from './CommentChip';
import { FeedbackThread } from './FeedbackThread';
import type { FeedbackHandle } from './FeedbackThread';
import { useThreadHost } from './threadHost';

export interface BlockThreadProps {
  blockId: string;
  type: string;
  feedback: Feedback[];
  replies: Reply[];
  locked: boolean;
  step?: boolean;
  onComposingChange?: (composing: boolean) => void;
}

export const BlockThread = forwardRef<FeedbackHandle, BlockThreadProps>(function BlockThread(
  { blockId, type, feedback, replies, locked, step, onComposingChange },
  ref,
) {
  const rail = useThreadHost() === 'rail';
  const { pin, requestCompose } = useActiveBlock();
  const threadRef = useRef<FeedbackHandle>(null);
  const draft = useDraft<string>(blockId, 'note');
  const copy = noteCopy(type, step);
  const count = feedback.length + replies.length;

  useImperativeHandle(
    ref,
    () => ({
      open: () => {
        if (!rail) {
          threadRef.current?.open();
          return;
        }
        pin(blockId);
        requestCompose();
      },
    }),
    [rail, pin, requestCompose, blockId],
  );

  if (!rail) {
    return (
      <FeedbackThread
        ref={threadRef}
        blockId={blockId}
        feedback={feedback}
        replies={replies}
        locked={locked}
        addLabel={copy.add}
        placeholder={copy.placeholder}
        onComposingChange={onComposingChange}
      />
    );
  }
  if (locked && count === 0) return null;
  return (
    <CommentChip
      blockId={blockId}
      count={count}
      locked={locked}
      addLabel={draft === undefined ? copy.add : 'Continue note'}
    />
  );
});
