import { Fragment } from 'react';
import type { CSSProperties } from 'react';
import type { ChoiceOption } from '../schema';
import type { Interactions } from '../events';
import { renderInlineMarkdown } from '../markdown';
import { Clamped } from './Clamped';
import { DetailDisclosure } from './Detail';
import { Mark } from './Mark';
import { RecommendedMark } from './OptionCard';

export function ChoiceGrid({
  options,
  axes,
  picks,
  multi,
  locked,
  showIndex,
  interactions,
  onToggle,
}: {
  options: ChoiceOption[];
  axes: string[];
  picks: string[];
  multi: boolean;
  locked: boolean;
  showIndex: boolean;
  interactions: Interactions;
  onToggle: (index: number) => void;
}) {
  const on = (option: ChoiceOption) => (picks.includes(option.id) ? ' selected' : '');
  const hasDetail = options.some((option) => option.detail);
  return (
    <div className="choice-grid" style={{ '--grid-n': options.length } as CSSProperties}>
      <span className="grid-corner" />
      {options.map((option, i) => {
        const selected = picks.includes(option.id);
        return (
          <div
            key={option.id}
            role={multi ? 'checkbox' : 'radio'}
            aria-checked={selected}
            aria-disabled={locked}
            tabIndex={locked ? -1 : 0}
            className={`option grid-head grid-pick${on(option)}`}
            data-option-id={option.id}
            onClick={() => {
              if (!locked) onToggle(i);
            }}
            onKeyDown={(e) => {
              if ((e.key === 'Enter' || e.key === ' ') && !e.metaKey && !e.ctrlKey) {
                e.preventDefault();
                if (!locked) onToggle(i);
              }
            }}
          >
            <span className="grid-pick-row">
              {showIndex && i < 9 && (
                <kbd className="option-index" aria-hidden>
                  {i + 1}
                </kbd>
              )}
              <span className="option-indicator" aria-hidden>
                {selected && <Mark kind={multi ? 'check' : 'ring'} />}
              </span>
              <span className="option-label">{option.label}</span>
              {option.recommended && <RecommendedMark />}
            </span>
            {option.hint && (
              <div className="option-hint" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(option.hint) }} />
            )}
            {option.md && <Clamped html={renderInlineMarkdown(option.md)} lines={4} className="option-md prose" />}
          </div>
        );
      })}
      {axes.map((axis, ai) => (
        <Fragment key={axis}>
          <span className="grid-axis">{axis}</span>
          {options.map((option) => {
            const fact = option.facts![ai]!;
            return (
              <span
                key={option.id}
                className={`grid-cell fact fact-${fact.tone ?? 'default'}${on(option)}`}
                data-option-id={option.id}
              >
                <span className="fact-value">{fact.value}</span>
              </span>
            );
          })}
        </Fragment>
      ))}
      {hasDetail && (
        <>
          <span className="grid-axis" />
          {options.map((option) => (
            <div key={option.id} className={`grid-cell${on(option)}`} data-option-id={option.id}>
              {option.detail && <DetailDisclosure detail={option.detail} interactions={interactions} />}
            </div>
          ))}
        </>
      )}
    </div>
  );
}
