import { useContext, useEffect, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Choice as ChoiceBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { choiceToggle } from '../decide';
import { factAxes } from '../focus';
import { useDecidable } from '../keyboard';
import { renderInlineMarkdown } from '../markdown';
import { Mark } from './Mark';
import { Clamped } from './Clamped';
import { DetailDisclosure } from './Detail';
import { FeedbackThread } from './FeedbackThread';
import type { FeedbackHandle } from './FeedbackThread';
import { FocusStageContext, FocusStepContext } from './focusStep';

export function Choice({ block, interactions }: { block: ChoiceBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const focus = useContext(FocusStepContext);
  const stage = useContext(FocusStageContext);
  const suppressPrompt = focus?.headlineId === block.id;
  const locked = closed || readOnly;
  const selection = interactions.choices[block.id];
  const selected = selection?.optionIds ?? [];
  const other = selection?.other ?? '';
  const multi = block.multi ?? false;
  // When every fact-carrying option shares a label sequence, values align into a
  // comparable subgrid; any mismatch returns null and the per-option cluster renders.
  const axes = factAxes(block.options);

  const feedbackRef = useRef<FeedbackHandle>(null);
  const otherRef = useRef<HTMLTextAreaElement>(null);
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

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'choice',
    disabled: locked,
    engage: () => feedbackRef.current?.open(),
    choose: (n) => {
      if (n === otherIndex) {
        otherRef.current?.focus();
        return;
      }
      const option = block.options[n - 1];
      if (option) toggle(option.id);
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
      <div
        className="options"
        role={multi ? 'group' : 'radiogroup'}
        aria-label={suppressPrompt ? block.prompt : undefined}
        data-facts-aligned={axes ? '' : undefined}
        style={axes ? ({ '--fact-count': axes.length } as CSSProperties) : undefined}
      >
        {axes && (
          <div className="fact-axes" aria-hidden>
            <span className="fact-axis-lead" />
            {axes.map((label, ai) => (
              <span key={ai} className="fact-axis">
                {label}
              </span>
            ))}
          </div>
        )}
        {block.options.map((option, i) => {
          const on = selected.includes(option.id);
          return (
            <div
              key={option.id}
              role={multi ? 'checkbox' : 'radio'}
              aria-checked={on}
              aria-disabled={locked}
              tabIndex={locked ? -1 : 0}
              className={`option${on ? ' selected' : ''}${option.recommended ? ' recommended' : ''}`}
              onMouseEnter={() => setHovered(option.id)}
              onMouseLeave={() => setHovered((h) => (h === option.id ? null : h))}
              onClick={() => {
                if (!locked) toggle(option.id);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  if (locked) return;
                  toggle(option.id);
                }
              }}
            >
              {cursor && i < 9 && (
                <kbd className="option-index" aria-hidden>
                  {i + 1}
                </kbd>
              )}
              <span className="option-indicator" aria-hidden>
                {on && <Mark kind={multi ? 'check' : 'ring'} />}
              </span>
              <span className="option-body">
                {option.recommended ? (
                  <span className="option-label">
                    {option.label}
                    <span className="option-reco">Recommended</span>
                  </span>
                ) : (
                  <span className="option-label">{option.label}</span>
                )}
                {option.hint && (
                  <div
                    className="option-hint"
                    dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(option.hint) }}
                  />
                )}
                {option.md && (
                  <Clamped
                    html={renderInlineMarkdown(option.md)}
                    lines={3}
                    className="option-md prose"
                  />
                )}
              </span>
              {axes
                ? axes.map((label, fi) => {
                    const fact = option.facts?.[fi];
                    return (
                      <span key={fi} className={`fact-cell fact-${fact?.tone ?? 'default'}`}>
                        <span className="fact-cell-value">{fact?.value ?? ''}</span>
                        <span className="fact-cell-label">{label}</span>
                      </span>
                    );
                  })
                : option.facts &&
                  option.facts.length > 0 && (
                    <span className="option-facts">
                      {option.facts.map((fact, fi) => (
                        <span key={fi} className={`fact fact-${fact.tone ?? 'default'}`}>
                          <span className="fact-value">{fact.value}</span>
                          {fact.label && <span className="fact-label">{fact.label}</span>}
                        </span>
                      ))}
                    </span>
                  )}
              {option.detail && <DetailDisclosure detail={option.detail} />}
            </div>
          );
        })}
      </div>
      <div
        className={`option option-other${otherActive ? ' selected' : ''}`}
        data-composing={otherComposing || undefined}
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
    </div>
  );
}
