import { useEffect, useRef, useState } from 'react';
import type { KeyboardEvent, MouseEvent } from 'react';
import { CollapsedGroup } from '@cc-interact/react';
import type { Detail } from '../schema';
import { useExpandAll } from '../expand';
import { renderMarkdown } from '../markdown';
import { Mark } from './Mark';

// DetailDisclosure renders Tier-2 drill-down behind a full-width "Details"
// affordance: an inline CollapsedGroup joined to expand-all, or a modal overlay.
export function DetailDisclosure({ detail }: { detail: Detail }) {
  return detail.mode === 'modal' ? <DetailModal detail={detail} /> : <DetailInline detail={detail} />;
}

// The disclosure lives inside a selectable option; a click anywhere in it, and
// the two keys the option acts on, must not trigger selection.
function stopClick(e: MouseEvent) {
  e.stopPropagation();
}

function stopSelectKeys(e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ' ') e.stopPropagation();
}

function DetailInline({ detail }: { detail: Detail }) {
  const { epoch, expanded } = useExpandAll();
  return (
    <div className="option-detail" onClick={stopClick} onKeyDown={stopSelectKeys}>
      <CollapsedGroup key={epoch} defaultExpanded={expanded} header="Details">
        <DetailBody detail={detail} />
      </CollapsedGroup>
    </div>
  );
}

function DetailModal({ detail }: { detail: Detail }) {
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
        <DetailBody detail={detail} />
      </dialog>
    </div>
  );
}

function DetailBody({ detail }: { detail: Detail }) {
  return (
    <div className="detail-body">
      {detail.pros && detail.pros.length > 0 && (
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
      {detail.cons && detail.cons.length > 0 && (
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
      {detail.md && (
        <div
          className="detail-md prose"
          dangerouslySetInnerHTML={{ __html: renderMarkdown(detail.md) }}
        />
      )}
    </div>
  );
}
