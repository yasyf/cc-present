import { useContext, useEffect, useRef, useState } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Choice as ChoiceBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { choiceToggle } from '../decide';
import { factAxes } from '../focus';
import { useDecidable } from '../keyboard';
import { Mark } from './Mark';
import { OptionCard } from './OptionCard';
import { OptionStrip, scrollBehavior } from './OptionStrip';
import { BlockThread } from './BlockThread';
import type { FeedbackHandle } from './FeedbackThread';
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
  const suppressPrompt = focus?.headlineId === block.id;
  const locked = closed || readOnly;
  const selection = interactions.choices[block.id];
  const selected = selection?.optionIds ?? [];
  const committed = selection?.other ?? '';
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
  const [draft, setDraft] = useState(committed);
  const [dirty, setDirty] = useState(false);
  const [synced, setSynced] = useState(committed);
  if (synced !== committed) {
    setSynced(committed);
    if (committed === draft.trim()) setDirty(false);
    else if (committed !== '' && !otherComposing && !dirty) setDraft(committed);
  }
  const text = draft.trim();
  const otherOn = dirty ? text !== '' : committed !== '';
  const picks = !multi && dirty && text !== '' ? [] : selected;

  function toggle(optionId: string) {
    const optionIds = choiceToggle(picks, optionId, multi);
    // A single-select answer is exclusive: an authored pick drops any write-in.
    if (!multi) setDirty(false);
    post({ type: 'choice.selected', blockId: block.id, optionIds, other: multi && otherOn ? text : undefined });
  }

  function postOther(other: string | undefined) {
    // A single-select write-in is the sole answer, so it clears authored picks;
    // multi keeps them alongside the write-in.
    return post({ type: 'choice.selected', blockId: block.id, optionIds: multi ? selected : [], other });
  }

  async function commitOther() {
    if (!dirty) return;
    if (text === committed) {
      setDirty(false);
      return;
    }
    if (await postOther(text || undefined)) return;
    setDraft((current) => (current === committed ? draft : current));
    setDirty(true);
  }

  function pickOther() {
    if (text !== '' && !otherOn) {
      setDirty(true);
      postOther(text);
    }
    otherRef.current?.focus();
  }

  function uncheckOther() {
    setDirty(false);
    postOther(undefined);
    otherRef.current?.focus();
  }

  // The write-in row rides one index past the last option, so the 1-9 keys can land
  // the cursor in the composer.
  const otherIndex = block.options.length + 1;

  // Bring the nth card into view when a click or the 1-9 keymap picks a card
  // resting outside the strip's snap window.
  function revealCard(cardIndex: number) {
    const card = optionsRef.current?.children[cardIndex] as HTMLElement | undefined;
    card?.scrollIntoView({ inline: 'nearest', block: 'nearest', behavior: scrollBehavior() });
  }

  function pick(i: number) {
    toggle(block.options[i]!.id);
    revealCard(i);
  }

  const [pointed, setPointed] = useState<string | null>(null);
  const [lastPointed, setLastPointed] = useState<string | null>(null);

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'choice',
    disabled: locked,
    engage: () => feedbackRef.current?.open(),
    choose: (n) => {
      if (n === otherIndex) {
        pickOther();
        revealCard(otherIndex - 1);
        return;
      }
      const option = block.options[n - 1];
      if (option) {
        setPointed(null);
        setLastPointed(option.id);
        pick(n - 1);
      }
    },
  });

  // Publish the active option's visual to the stage: the pointed card, else the
  // latest pick, else the last pointed card, else the first option. Board mode has
  // no stage and skips this.
  const activeId = pointed ?? picks.at(-1) ?? lastPointed ?? block.options[0]?.id;
  const activeVisual = block.options.find((o) => o.id === activeId)?.visual ?? null;
  useEffect(() => {
    stage?.setVisual(activeVisual);
  }, [stage, activeVisual]);

  const picked = new Set(block.options.flatMap((option, i) => (picks.includes(option.id) ? [i] : [])));
  if (otherOn) picked.add(block.options.length);
  const feedback = interactions.feedback[block.id] ?? [];
  const replies = interactions.replies[block.id] ?? [];
  const thread = (
    <BlockThread
      ref={feedbackRef}
      blockId={block.id}
      type={block.type}
      feedback={feedback}
      replies={replies}
      locked={locked}
      onComposingChange={setNoteComposing}
    />
  );

  return (
    <div className="choice" ref={ref} data-kbd-cursor={cursor || undefined} data-composing={noteComposing || undefined}>
      {!suppressPrompt && block.prompt && <p className="choice-prompt">{block.prompt}</p>}
      <OptionStrip
        strip={strip}
        role={multi ? 'group' : 'radiogroup'}
        ariaLabel={suppressPrompt ? block.prompt : undefined}
        cardCount={block.options.length + 1}
        picked={picked}
        containerRef={optionsRef}
        navActions={strip && rail ? thread : undefined}
      >
        {block.options.map((option, i) => (
          <OptionCard
            key={option.id}
            option={option}
            index={i}
            selected={picks.includes(option.id)}
            multi={multi}
            locked={locked}
            showIndex={!!cursor && i < 9}
            stage={stage}
            interactions={interactions}
            axes={axes}
            onToggle={() => pick(i)}
            onHoverEnter={() => {
              setPointed(option.id);
              setLastPointed(option.id);
            }}
            onHoverLeave={() => setPointed((p) => (p === option.id ? null : p))}
          />
        ))}
        <div
          role={multi ? 'checkbox' : 'radio'}
          aria-checked={otherOn}
          aria-disabled={locked}
          tabIndex={locked ? -1 : 0}
          className={`option option-other${otherOn ? ' selected' : ''}`}
          data-composing={otherComposing || undefined}
          onClick={(e) => {
            if (locked) return;
            revealCard(otherIndex - 1);
            // Activating the card chrome (label, indicator, padding) lands the
            // cursor in the write-in; a click on the textarea keeps its native focus.
            if (e.target === otherRef.current) return;
            if (multi && otherOn) uncheckOther();
            else pickOther();
          }}
          onKeyDown={(e) => {
            // Only when the card itself holds focus — a keydown bubbling from the
            // textarea reports the textarea as target, so literal Space/Enter type
            // and commit as usual.
            if ((e.key === 'Enter' || e.key === ' ') && !e.metaKey && !e.ctrlKey && e.target === e.currentTarget) {
              e.preventDefault();
              if (!locked) pickOther();
            }
          }}
        >
          {cursor && otherIndex <= 9 && (
            <kbd className="option-index" aria-hidden>
              {otherIndex}
            </kbd>
          )}
          <span className="option-indicator" aria-hidden>
            {otherOn && <Mark kind={multi ? 'check' : 'ring'} />}
          </span>
          <span className="option-body">
            <span className="option-label">Other</span>
            <textarea
              ref={otherRef}
              className="option-other-input"
              rows={1}
              value={draft}
              placeholder="Write in your own answer…"
              disabled={locked}
              onFocus={() => setOtherComposing(true)}
              onChange={(e) => {
                setDraft(e.target.value);
                setDirty(true);
              }}
              onBlur={() => {
                setOtherComposing(false);
                void commitOther();
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey && !e.metaKey && !e.ctrlKey) {
                  e.preventDefault();
                  e.currentTarget.blur();
                }
              }}
            />
          </span>
        </div>
      </OptionStrip>
      {!(strip && rail) && thread}
    </div>
  );
}
