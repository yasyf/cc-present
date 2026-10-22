import { useEffect, useRef, useState } from 'react';
import type { Interactions, RoundRecord } from '../events';
import { RoundGroup } from './RoundGroup';

export function HistoryDrawer({
  open,
  onClose,
  history,
  interactions,
}: {
  open: boolean;
  onClose: () => void;
  history: RoundRecord[];
  interactions: Interactions;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const [shown, setShown] = useState(false);

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
      onClose={() => {
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
          {history.map((record) => (
            <div className="round-group" key={`round-${record.number}`}>
              <RoundGroup record={record} interactions={interactions} />
            </div>
          ))}
        </div>
      )}
    </dialog>
  );
}
