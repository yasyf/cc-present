import { useGroupReadOnly } from '@cc-interact/react';
import type { Choice as ChoiceBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { choiceToggle } from '../decide';
import { useDecidable } from '../keyboard';
import { renderInlineMarkdown } from '../markdown';
import { Mark } from './Mark';
import { Clamped } from './Clamped';

export function Choice({ block, interactions }: { block: ChoiceBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const locked = closed || readOnly;
  const selected = interactions.choices[block.id]?.optionIds ?? [];
  const multi = block.multi ?? false;

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
      {block.prompt && <p className="choice-prompt">{block.prompt}</p>}
      <div className="options" role={multi ? 'group' : 'radiogroup'}>
        {block.options.map((option, i) => {
          const on = selected.includes(option.id);
          return (
            <div
              key={option.id}
              role={multi ? 'checkbox' : 'radio'}
              aria-checked={on}
              aria-disabled={locked}
              tabIndex={locked ? -1 : 0}
              className={`option${on ? ' selected' : ''}`}
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
                <span className="option-label">{option.label}</span>
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
            </div>
          );
        })}
      </div>
    </div>
  );
}
