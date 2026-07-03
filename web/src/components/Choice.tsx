import type { Choice as ChoiceBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { renderInlineMarkdown } from '../markdown';

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
            <button
              type="button"
              key={option.id}
              role={multi ? 'checkbox' : 'radio'}
              aria-checked={on}
              disabled={closed}
              className={`option${on ? ' selected' : ''}`}
              onClick={() => toggle(option.id)}
            >
              <span className="option-label">{option.label}</span>
              {option.md && (
                <span
                  className="option-md prose"
                  dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(option.md) }}
                />
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}
