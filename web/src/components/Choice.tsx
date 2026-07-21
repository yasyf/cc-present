import { useContext, useEffect, useRef, useState } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Choice as ChoiceBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { choiceToggle } from '../decide';
import { factAxes } from '../focus';
import { useDecidable } from '../keyboard';
import { useActiveBlock } from '../activeBlock';
import { Mark } from './Mark';
import { OptionCard } from './OptionCard';
import { OptionStrip } from './OptionStrip';
import { FeedbackThread } from './FeedbackThread';
import type { FeedbackHandle } from './FeedbackThread';
import { CommentChip } from './CommentChip';
import { FocusStageContext, FocusStepContext } from './focusStep';
import { useThreadHost } from './threadHost';

// At three or more options the choice renders as a scroll-snap card strip; two or
// fewer stay the vertical stack.
const STRIP_MIN = 3;

export function Choice({ block, interactions }: { block: ChoiceBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const focus = useContext(FocusStepContext);
  const stage = useContext(FocusStageContext);
  const rail = useThreadHost() === 'rail';
  const { requestCompose } = useActiveBlock();
  const suppressPrompt = focus?.headlineId === block.id;
  const locked = closed || readOnly;
  const selection = interactions.choices[block.id];
  const selected = selection?.optionIds ?? [];
  const other = selection?.other ?? '';
  const multi = block.multi ?? false;
  // The shared ordered fact labels when options declare a matching sequence; each
  // card renders its facts in this order so equal-height cards line their rows up.
  const axes = factAxes(block.options);
  const strip = block.options.length >= STRIP_MIN;

  const feedbackRef = useRef<FeedbackHandle>(null);
  const otherRef = useRef<HTMLTextAreaElement>(null);
  const optionsRef = useRef<HTMLDivElement>(null);
  const [noteComposing, setNoteComposing] = useState(false);
  const [otherComposing, setOtherComposing] = useState(false);
  const [draftOther, setDraftOther] = useState(other);
  // Keep the write-in field in sync with the committed answer while it is idle; a
  // re-pick on a single-select clears `other` server-side and the field follows.
  useEffect(() => {
    if (!otherComposing) setDraftOther(other);
  }, [other, otherComposing]);

  function toggle(optionId: string) {
    const optionIds = choiceToggle(selected, optionId, multi);
    // A single-select answer is exclusive: an authored pick drops any write-in.
    post({ type: 'choice.selected', blockId: block.id, optionIds, other: multi ? other || undefined : undefined });
  }

  function commitOther(text: string) {
    const trimmed = text.trim();
    if (trimmed === other) return;
    // A single-select write-in is the sole answer, so it clears authored picks;
    // multi keeps them alongside the write-in.
    post({
      type: 'choice.selected',
      blockId: block.id,
      optionIds: multi ? selected : [],
      other: trimmed || undefined,
    });
  }

  // The write-in row rides one index past the last option, so the 1-9 keys can land
  // the cursor in the composer.
  const otherIndex = block.options.length + 1;

  // Bring the nth card into view so the 1-9 keymap stays visibly correct as the
  // cursor lands a card outside the strip's snap window.
  function revealCard(cardIndex: number) {
    const card = optionsRef.current?.children[cardIndex] as HTMLElement | undefined;
    card?.scrollIntoView({ inline: 'nearest', block: 'nearest' });
  }

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'choice',
    disabled: locked,
    engage: rail ? requestCompose : () => feedbackRef.current?.open(),
    choose: (n) => {
      if (n === otherIndex) {
        otherRef.current?.focus();
        revealCard(otherIndex - 1);
        return;
      }
      const option = block.options[n - 1];
      if (option) {
        toggle(option.id);
        revealCard(n - 1);
      }
    },
  });

  // Publish the active option's visual to the stage: hovered, else the single-select
  // pick, else the first option. Board mode has no stage and skips this.
  const [hovered, setHovered] = useState<string | null>(null);
  const activeId = hovered ?? (multi ? undefined : selected[0]) ?? block.options[0]?.id;
  const activeVisual = block.options.find((o) => o.id === activeId)?.visual ?? null;
  useEffect(() => {
    stage?.setVisual(activeVisual);
  }, [stage, activeVisual]);

  const otherActive = other !== '';
  const feedback = interactions.feedback[block.id] ?? [];
  const replies = interactions.replies[block.id] ?? [];

  return (
    <div className="choice" ref={ref} data-kbd-cursor={cursor || undefined} data-composing={noteComposing || undefined}>
      {!suppressPrompt && block.prompt && <p className="choice-prompt">{block.prompt}</p>}
      <OptionStrip
        strip={strip}
        role={multi ? 'group' : 'radiogroup'}
        ariaLabel={suppressPrompt ? block.prompt : undefined}
        cardCount={block.options.length + 1}
        containerRef={optionsRef}
      >
        {block.options.map((option, i) => (
          <OptionCard
            key={option.id}
            option={option}
            index={i}
            selected={selected.includes(option.id)}
            multi={multi}
            locked={locked}
            showIndex={!!cursor && i < 9}
            stage={stage}
            interactions={interactions}
            axes={axes}
            onToggle={() => toggle(option.id)}
            onHoverEnter={() => setHovered(option.id)}
            onHoverLeave={() => setHovered((h) => (h === option.id ? null : h))}
          />
        ))}
        <div
          role={multi ? 'checkbox' : 'radio'}
          aria-checked={otherActive}
          aria-disabled={locked}
          tabIndex={locked ? -1 : 0}
          className={`option option-other${otherActive ? ' selected' : ''}`}
          data-composing={otherComposing || undefined}
          onClick={(e) => {
            // Activating the card chrome (label, indicator, padding) lands the
            // cursor in the write-in; a click on the textarea keeps its native focus.
            if (!locked && e.target !== otherRef.current) otherRef.current?.focus();
          }}
          onKeyDown={(e) => {
            // Only when the card itself holds focus — a keydown bubbling from the
            // textarea reports the textarea as target, so literal Space/Enter type
            // and commit as usual.
            if ((e.key === 'Enter' || e.key === ' ') && e.target === e.currentTarget) {
              e.preventDefault();
              if (!locked) otherRef.current?.focus();
            }
          }}
        >
          {cursor && otherIndex <= 9 && (
            <kbd className="option-index" aria-hidden>
              {otherIndex}
            </kbd>
          )}
          <span className="option-indicator" aria-hidden>
            {otherActive && <Mark kind={multi ? 'check' : 'ring'} />}
          </span>
          <span className="option-body">
            <span className="option-label">Other</span>
            <textarea
              ref={otherRef}
              className="option-other-input"
              rows={1}
              value={draftOther}
              placeholder="Write in your own answer…"
              disabled={locked}
              onFocus={() => setOtherComposing(true)}
              onChange={(e) => setDraftOther(e.target.value)}
              onBlur={(e) => {
                setOtherComposing(false);
                commitOther(e.target.value);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  commitOther(e.currentTarget.value);
                  otherRef.current?.blur();
                }
              }}
            />
          </span>
        </div>
      </OptionStrip>
      {rail ? (
        <CommentChip blockId={block.id} count={feedback.length + replies.length} addLabel="Add note" />
      ) : (
        <FeedbackThread
          ref={feedbackRef}
          blockId={block.id}
          feedback={feedback}
          replies={replies}
          locked={locked}
          addLabel="Add note"
          placeholder="Add a note for the agent…"
          onComposingChange={setNoteComposing}
        />
      )}
    </div>
  );
}
