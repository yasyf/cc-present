import { useEffect, useRef } from 'react';
import type { ReactNode } from 'react';

// CommentsSheet hosts the rail below the margin breakpoint as a native <dialog>:
// showModal gives it a focus trap and free Esc, and keyboard.tsx already suppresses
// the global shortcut ring whenever a dialog[open] is in the tree. It is never used
// at desktop — the sticky margin rail is, so j/k keep working beside it.
export function CommentsSheet({ open, onClose, children }: { open: boolean; onClose: () => void; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dlg = ref.current;
    if (!dlg) return;
    if (open && !dlg.open) dlg.showModal();
    else if (!open && dlg.open) dlg.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className="comments-sheet"
      onClose={onClose}
      onCancel={onClose}
      onClick={(e) => {
        if (e.target === ref.current) onClose();
      }}
    >
      {open && children}
    </dialog>
  );
}
