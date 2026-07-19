import { useEffect, useRef, useState } from 'react';
import type { KeyboardEvent, MouseEvent } from 'react';
import { CollapsedGroup } from '@cc-interact/react';
import type { Detail, OptionVisual } from '../schema';
import type { Interactions } from '../events';
import { useExpandAll } from '../expand';
import { renderMarkdown } from '../markdown';
import { BlockBody } from './BlockRenderer';
import { Mark } from './Mark';

// DetailDisclosure renders Tier-2 drill-down behind a full-width "Details"
// affordance: an inline CollapsedGroup joined to expand-all, or a modal overlay. In
// board mode it also carries the option's visual, which focus mode mounts on the
// stage instead; the disclosure exists whenever a detail or a visual is present.
export function DetailDisclosure({
  detail,
  visual,
  interactions,
}: {
  detail?: Detail;
  visual?: OptionVisual;
  interactions: Interactions;
}) {
  return detail?.mode === 'modal' ? (
    <DetailModal detail={detail} visual={visual} interactions={interactions} />
  ) : (
    <DetailInline detail={detail} visual={visual} interactions={interactions} />
  );
}

// The disclosure lives inside a selectable option; a click anywhere in it, and
// the two keys the option acts on, must not trigger selection.
function stopClick(e: MouseEvent) {
  e.stopPropagation();
}

function stopSelectKeys(e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ' ') e.stopPropagation();
}

function DetailInline({
  detail,
  visual,
  interactions,
}: {
  detail?: Detail;
  visual?: OptionVisual;
  interactions: Interactions;
}) {
  const { epoch, expanded } = useExpandAll();
  return (
    <div className="option-detail" onClick={stopClick} onKeyDown={stopSelectKeys}>
      <CollapsedGroup key={epoch} defaultExpanded={expanded} header="Details">
        <DetailBody detail={detail} visual={visual} interactions={interactions} />
      </CollapsedGroup>
    </div>
  );
}

function DetailModal({ detail, visual, interactions }: { detail: Detail; visual?: OptionVisual; interactions: Interactions }) {
  const ref = useRef<HTMLDialogElement>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    else if (!open && dialog.open) dialog.close();
  }, [open]);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    const onClose = () => setOpen(false);
    dialog.addEventListener('close', onClose);
    return () => dialog.removeEventListener('close', onClose);
  }, []);

  return (
    <div className="option-detail" onClick={stopClick} onKeyDown={stopSelectKeys}>
      <button
        type="button"
        className="detail-trigger"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        Details
      </button>
      <dialog
        ref={ref}
        className="detail-modal"
        aria-label="Details"
        onClick={(e) => {
          if (e.target === ref.current) ref.current.close();
        }}
      >
        <button
          type="button"
          className="detail-modal-close"
          aria-label="Close"
          onClick={() => ref.current?.close()}
        >
          <Mark kind="cross" />
        </button>
        <DetailBody detail={detail} visual={visual} interactions={interactions} />
      </dialog>
    </div>
  );
}

function DetailBody({
  detail,
  visual,
  interactions,
}: {
  detail?: Detail;
  visual?: OptionVisual;
  interactions: Interactions;
}) {
  return (
    <div className="detail-body">
      {visual && (
        <div className="detail-visual">
          <BlockBody block={visual} interactions={interactions} />
        </div>
      )}
      {detail?.pros && detail.pros.length > 0 && (
        <ul className="detail-list detail-pros">
          {detail.pros.map((pro, i) => (
            <li key={i}>
              <span className="detail-glyph" aria-hidden>
                ✓
              </span>
              <span className="detail-text">{pro}</span>
            </li>
          ))}
        </ul>
      )}
      {detail?.cons && detail.cons.length > 0 && (
        <ul className="detail-list detail-cons">
          {detail.cons.map((con, i) => (
            <li key={i}>
              <span className="detail-glyph" aria-hidden>
                ✗
              </span>
              <span className="detail-text">{con}</span>
            </li>
          ))}
        </ul>
      )}
      {detail?.md && (
        <div
          className="detail-md prose"
          dangerouslySetInnerHTML={{ __html: renderMarkdown(detail.md) }}
        />
      )}
    </div>
  );
}
