import type { MouseEvent } from 'react';
import type { ChoiceOption } from '../schema';
import type { Interactions } from '../events';
import { renderInlineMarkdown } from '../markdown';
import { Mark } from './Mark';
import { Clamped } from './Clamped';
import { DetailDisclosure } from './Detail';

// OptionCard renders one choice option as a selectable card: keyboard index badge,
// selection indicator, label + Recommended stamp, hint/md body, a per-card facts
// tray in factAxes() order, and a foot Detail drill-down. It keeps the .option
// grammar the keyboard, shell, and swipe-exemption rules couple to.
export function OptionCard({
  option,
  index,
  selected,
  multi,
  locked,
  showIndex,
  interactions,
  axes,
  onToggle,
  onHoverEnter,
  onHoverLeave,
}: {
  option: ChoiceOption;
  index: number;
  selected: boolean;
  multi: boolean;
  locked: boolean;
  showIndex: boolean;
  interactions: Interactions;
  axes: string[] | null;
  onToggle: () => void;
  onHoverEnter: (pointer?: MouseEvent) => void;
  onHoverLeave: (pointer?: MouseEvent) => void;
}) {
  const facts = option.facts && option.facts.length > 0 ? option.facts : null;
  const factLabels = axes ?? facts?.map((f) => f.label ?? '') ?? [];
  return (
    <div
      role={multi ? 'checkbox' : 'radio'}
      aria-checked={selected}
      aria-disabled={locked}
      tabIndex={locked ? -1 : 0}
      className={`option${selected ? ' selected' : ''}${option.recommended ? ' recommended' : ''}`}
      onMouseMove={onHoverEnter}
      onMouseLeave={onHoverLeave}
      onFocus={() => onHoverEnter()}
      onBlur={() => onHoverLeave()}
      onClick={() => {
        if (!locked) onToggle();
      }}
      onKeyDown={(e) => {
        if ((e.key === 'Enter' || e.key === ' ') && !e.metaKey && !e.ctrlKey) {
          e.preventDefault();
          if (!locked) onToggle();
        }
      }}
    >
      {showIndex && (
        <kbd className="option-index" aria-hidden>
          {index + 1}
        </kbd>
      )}
      <span className="option-indicator" aria-hidden>
        {selected && <Mark kind={multi ? 'check' : 'ring'} />}
      </span>
      <span className="option-body">
        <span className="option-label">
          {option.label}
          {option.recommended && <span className="option-reco">Recommended</span>}
        </span>
        {option.hint && (
          <div className="option-hint" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(option.hint) }} />
        )}
        {option.md && <Clamped html={renderInlineMarkdown(option.md)} lines={8} className="option-md prose" />}
      </span>
      {facts && (
        <span className="option-facts">
          {factLabels.map((label, fi) => {
            const fact = facts[fi];
            if (!fact) return null;
            return (
              <span key={fi} className={`fact fact-${fact.tone ?? 'default'}`}>
                {label && <span className="fact-label">{label}</span>}
                <span className="fact-value">{fact.value}</span>
              </span>
            );
          })}
        </span>
      )}
      {option.detail && <DetailDisclosure detail={option.detail} interactions={interactions} />}
    </div>
  );
}
