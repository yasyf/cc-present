import { useContext } from 'react';
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
import { FocusStepContext } from './focusStep';

export function Choice({ block, interactions }: { block: ChoiceBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const focus = useContext(FocusStepContext);
  const suppressPrompt = focus?.headlineId === block.id;
  const locked = closed || readOnly;
  const selected = interactions.choices[block.id]?.optionIds ?? [];
  const multi = block.multi ?? false;
  // When every fact-carrying option shares a label sequence, values align into a
  // comparable subgrid; any mismatch returns null and the per-option cluster renders.
  const axes = factAxes(block.options);

  function toggle(optionId: string) {
    post({ type: 'choice.selected', blockId: block.id, optionIds: choiceToggle(selected, optionId, multi) });
  }

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'choice',
    disabled: locked,
    choose: (n) => {
      const option = block.options[n - 1];
      if (option) toggle(option.id);
    },
  });

  return (
    <div className="choice" ref={ref} data-kbd-cursor={cursor || undefined}>
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
    </div>
  );
}
