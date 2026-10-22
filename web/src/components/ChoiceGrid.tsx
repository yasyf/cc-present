import { Fragment, useCallback, useState } from 'react';
import type { CSSProperties } from 'react';
import type { ChoiceOption, OptionVisual } from '../schema';
import type { Interactions } from '../events';
import { renderInlineMarkdown } from '../markdown';
import { BlockBody } from './BlockRenderer';
import { Clamped } from './Clamped';
import { DetailDisclosure } from './Detail';
import { Lightbox } from './Lightbox';
import { Mark } from './Mark';
import { RecommendedMark } from './OptionCard';

export function visualTitle(visual: OptionVisual): string {
  switch (visual.type) {
    case 'code':
      return visual.title ?? visual.lang;
    case 'diff':
      return visual.title ?? 'Diff';
    case 'diagram':
      return visual.title ?? 'Diagram';
    case 'image':
      return visual.caption ?? visual.alt;
    case 'chart':
      return visual.title ?? 'Chart';
    case 'term':
      return visual.title ?? 'Terminal';
    case 'filetree':
      return visual.title ?? 'Files';
    case 'record':
      return visual.title ?? 'Record';
  }
}

function GridVisual({ visual, interactions }: { visual: OptionVisual; interactions: Interactions }) {
  const [open, setOpen] = useState(false);
  const close = useCallback(() => setOpen(false), []);
  const title = visualTitle(visual);
  return (
    <>
      <div className="grid-thumb-frame" inert>
        <BlockBody block={visual} interactions={interactions} />
      </div>
      <button
        type="button"
        className="grid-thumb-open"
        aria-haspopup="dialog"
        aria-label={`Expand ${title}`}
        onClick={() => setOpen(true)}
      />
      <Lightbox open={open} onClose={close} label={title} caption={title}>
        {open ? <BlockBody block={visual} interactions={interactions} /> : null}
      </Lightbox>
    </>
  );
}

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
  const hasVisual = options.some((option) => option.visual);
  const hasDetail = options.some((option) => option.detail);
  return (
    <div className="choice-grid" style={{ '--grid-n': options.length } as CSSProperties}>
      <span className="grid-corner" />
      {options.map((option) => (
        <div key={option.id} className={`grid-head${on(option)}`}>
          <span className="option-label">{option.label}</span>
          {option.recommended && <RecommendedMark />}
          {option.hint && (
            <div className="option-hint" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(option.hint) }} />
          )}
          {option.md && <Clamped html={renderInlineMarkdown(option.md)} lines={4} className="option-md prose" />}
        </div>
      ))}
      {axes.map((axis, ai) => (
        <Fragment key={axis}>
          <span className="grid-axis">{axis}</span>
          {options.map((option) => {
            const fact = option.facts![ai]!;
            return (
              <span key={option.id} className={`grid-cell fact fact-${fact.tone ?? 'default'}${on(option)}`}>
                <span className="fact-value">{fact.value}</span>
              </span>
            );
          })}
        </Fragment>
      ))}
      {hasVisual && (
        <>
          <span className="grid-axis">Visual</span>
          {options.map((option) => (
            <div key={option.id} className={`grid-cell grid-thumb${on(option)}`}>
              {option.visual && <GridVisual visual={option.visual} interactions={interactions} />}
            </div>
          ))}
        </>
      )}
      {hasDetail && (
        <>
          <span className="grid-axis" />
          {options.map((option) => (
            <div key={option.id} className={`grid-cell${on(option)}`}>
              {option.detail && <DetailDisclosure detail={option.detail} interactions={interactions} />}
            </div>
          ))}
        </>
      )}
      <span className="grid-axis grid-pick-axis" />
      {options.map((option, i) => {
        const selected = picks.includes(option.id);
        return (
          <div
            key={option.id}
            role={multi ? 'checkbox' : 'radio'}
            aria-checked={selected}
            aria-disabled={locked}
            tabIndex={locked ? -1 : 0}
            className={`option grid-pick${selected ? ' selected' : ''}`}
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
            {showIndex && i < 9 && (
              <kbd className="option-index" aria-hidden>
                {i + 1}
              </kbd>
            )}
            <span className="option-indicator" aria-hidden>
              {selected && <Mark kind={multi ? 'check' : 'ring'} />}
            </span>
            <span className="option-body">
              <span className="option-label">{option.label}</span>
            </span>
          </div>
        );
      })}
    </div>
  );
}
