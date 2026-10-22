import { useContext, useEffect, useRef, useState } from 'react';
import type { FocusEvent, MouseEvent } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Choice as ChoiceBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { draftStore } from '../drafts';
import { choiceToggle } from '../decide';
import { choiceShape, factAxes } from '../focus';
import { useDecidable } from '../keyboard';
import { useMediaQuery } from '../useMediaQuery';
import { Mark } from './Mark';
import { OptionCard } from './OptionCard';
import { ChoiceGrid } from './ChoiceGrid';
import { BlockThread } from './BlockThread';
import { BlockBody } from './BlockRenderer';
import type { FeedbackHandle } from './FeedbackThread';
import { FocusStepContext } from './focusStep';

export function Choice({ block, interactions }: { block: ChoiceBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const focus = useContext(FocusStepContext);
  const narrow = useMediaQuery('(max-width: 720px)');
  const suppressPrompt = focus?.headlineId === block.id;
  const locked = closed || readOnly;
  const selection = interactions.choices[block.id];
  const selected = selection?.optionIds ?? [];
  const committed = selection?.other ?? '';
  const multi = block.multi ?? false;
  const axes = factAxes(block.options);
  const grid = axes !== null && !narrow && choiceShape(block.options) === 'grid';

  const feedbackRef = useRef<FeedbackHandle>(null);
  const otherRef = useRef<HTMLTextAreaElement>(null);
  const [noteComposing, setNoteComposing] = useState(false);
  const [otherComposing, setOtherComposing] = useState(false);
  const [draft, setDraft] = useState(
    () => committed || (readOnly ? undefined : draftStore.get<string>(block.id, 'other')) || '',
  );
  useEffect(() => {
    if (readOnly) return;
    if (draft) draftStore.set(block.id, 'other', draft);
    else draftStore.clear(block.id, 'other');
  }, [readOnly, block.id, draft]);
  const [dirty, setDirty] = useState(false);
  const lastAction = useRef(0);
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
    lastAction.current += 1;
    const optionIds = choiceToggle(picks, optionId, multi);
    // A single-select answer is exclusive: an authored pick drops any write-in.
    if (!multi) setDirty(false);
    post({ type: 'choice.selected', blockId: block.id, optionIds, other: multi && otherOn ? text : undefined });
  }

  function postOther(other: string | undefined) {
    // A single-select write-in is the sole answer, so it clears authored picks;
    // multi keeps them alongside the write-in.
    lastAction.current += 1;
    return post({ type: 'choice.selected', blockId: block.id, optionIds: multi ? selected : [], other });
  }

  async function commitOther() {
    if (!dirty) return;
    if (text === committed) {
      setDirty(false);
      return;
    }
    const sent = postOther(text || undefined);
    const action = lastAction.current;
    if ((await sent) || lastAction.current !== action) return;
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

  function pick(i: number) {
    toggle(block.options[i]!.id);
  }

  const [pointed, setPointed] = useState<string | null>(null);
  const pointerAt = useRef('');

  function pointerMoved({ clientX, clientY }: MouseEvent) {
    const at = `${clientX},${clientY}`;
    const moved = at !== pointerAt.current;
    pointerAt.current = at;
    return moved;
  }

  const pointing = {
    onMouseMove: (e: MouseEvent) => {
      if (pointerMoved(e)) setPointed(optionAt(e.target));
    },
    onMouseLeave: (e: MouseEvent) => {
      if (pointerMoved(e)) setPointed(null);
    },
    onFocus: (e: FocusEvent) => setPointed(optionAt(e.target)),
    onBlur: () => setPointed(null),
  };

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'choice',
    disabled: locked,
    engage: () => feedbackRef.current?.open(),
    choose: (n) => {
      if (n === otherIndex) {
        pickOther();
        return;
      }
      if (block.options[n - 1]) {
        setPointed(null);
        pick(n - 1);
      }
    },
  });

  const shownId = pointed ?? picks.at(-1);
  const shownVisual = block.options.find((option) => option.id === shownId)?.visual;
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

  const other = (
    <div
      role={multi ? 'checkbox' : 'radio'}
      aria-checked={otherOn}
      aria-disabled={locked}
      tabIndex={locked ? -1 : 0}
      className={`option option-other${otherOn ? ' selected' : ''}`}
      data-composing={otherComposing || undefined}
      onClick={(e) => {
        if (locked) return;
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
  );

  return (
    <div
      className="choice"
      ref={ref}
      data-kbd-cursor={cursor || undefined}
      data-composing={noteComposing || undefined}
      data-closed={closed || undefined}
    >
      {!suppressPrompt && block.prompt && <p className="choice-prompt">{block.prompt}</p>}
      {grid ? (
        <div className="choice-compare" role={multi ? 'group' : 'radiogroup'} aria-label={block.prompt} {...pointing}>
          <ChoiceGrid
            options={block.options}
            axes={axes}
            picks={picks}
            multi={multi}
            locked={locked}
            showIndex={!!cursor}
            interactions={interactions}
            onToggle={pick}
          />
          {shownVisual && (
            <div className="option-visual" data-option-id={shownId}>
              <BlockBody block={shownVisual} interactions={interactions} />
            </div>
          )}
          {other}
        </div>
      ) : (
        <div className="options" role={multi ? 'group' : 'radiogroup'} aria-label={block.prompt} {...pointing}>
          {block.options.map((option, i) => (
            <div key={option.id} className="option-row" data-option-id={option.id}>
              <OptionCard
                option={option}
                index={i}
                selected={picks.includes(option.id)}
                multi={multi}
                locked={locked}
                showIndex={!!cursor && i < 9}
                interactions={interactions}
                axes={axes}
                onToggle={() => pick(i)}
              />
              {option.visual && option.id === shownId && (
                <div className="option-visual">
                  <BlockBody block={option.visual} interactions={interactions} />
                </div>
              )}
            </div>
          ))}
          {other}
        </div>
      )}
      {thread}
    </div>
  );
}

function optionAt(target: EventTarget): string | null {
  return (target as Element).closest<HTMLElement>('[data-option-id]')?.dataset.optionId ?? null;
}
