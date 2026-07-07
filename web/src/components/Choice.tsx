import type { Choice as ChoiceBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { renderInlineMarkdown } from '../markdown';
import { Clamped } from './Clamped';

export function Choice({ block, interactions }: { block: ChoiceBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const selected = interactions.choices[block.id]?.optionIds ?? [];
  const multi = block.multi ?? false;

  function toggle(optionId: string) {
    let next: string[];
    if (multi) {
      next = selected.includes(optionId)
        ? selected.filter((id) => id !== optionId)
        : [...selected, optionId];
    } else {
      next = selected.includes(optionId) ? [] : [optionId];
    }
    post({ type: 'choice.selected', blockId: block.id, optionIds: next });
  }

  return (
    <div className="choice">
      {block.prompt && <p className="choice-prompt">{block.prompt}</p>}
      <div className="options" role={multi ? 'group' : 'radiogroup'}>
        {block.options.map((option) => {
          const on = selected.includes(option.id);
          return (
            <div
              key={option.id}
              role={multi ? 'checkbox' : 'radio'}
              aria-checked={on}
              aria-disabled={closed}
              tabIndex={closed ? -1 : 0}
              className={`option${on ? ' selected' : ''}`}
              onClick={() => {
                if (!closed) toggle(option.id);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  if (closed) return;
                  toggle(option.id);
                }
              }}
            >
              <span className="option-indicator" aria-hidden />
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
