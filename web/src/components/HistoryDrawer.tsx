import { useEffect, useRef, useState } from 'react';
import { useRoundRecords, useSubject } from '../api';
import type { Interactions, RoundEntry } from '../events';
import { RoundGroup } from './RoundGroup';

export function HistoryDrawer({
  open,
  onClose,
  history,
  interactions,
}: {
  open: boolean;
  onClose: () => void;
  history: RoundEntry[];
  interactions: Interactions;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const [shown, setShown] = useState(false);
  const { subject } = useSubject();
  const { records, failed } = useRoundRecords(subject, history, shown);

  useEffect(() => {
    const dialog = ref.current!;
    if (open && !dialog.open) {
      dialog.showModal();
      setShown(true);
    } else if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className="history-drawer"
      aria-label="Earlier rounds"
      onClose={(e) => {
        if (e.target !== e.currentTarget) return;
        setShown(false);
        onClose();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="history-drawer-head">
        <h2 className="history-drawer-title">Earlier rounds</h2>
        <button type="button" className="link-btn" onClick={onClose}>
          Close
        </button>
      </div>
      {shown && (
        <div className="history-drawer-rounds">
          {records ? (
            records.map((record) => (
              <div className="round-group" key={`round-${record.number}`}>
                <RoundGroup record={record} interactions={interactions} />
              </div>
            ))
          ) : failed ? (
            <p className="history-drawer-status">Earlier rounds didn't load. Close and reopen to try again.</p>
          ) : (
            <div className="skeleton" aria-hidden>
              <div className="skeleton-card" />
              <div className="skeleton-card" />
            </div>
          )}
        </div>
      )}
    </dialog>
  );
}
