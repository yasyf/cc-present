import { useEffect, useRef, useState } from 'react';
import { useKeyboardApi } from '../keyboard';
import { ThemeToggle } from './ThemeToggle';

export function HeaderMenu({ commentCount, onOpenComments }: { commentCount?: number; onOpenComments?: () => void }) {
  const { openHelp } = useKeyboardApi();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = ref.current!;
    if (open && !dialog.open) dialog.showModal();
    else if (!open && dialog.open) dialog.close();
  }, [open]);

  const run = (action: () => void) => () => {
    setOpen(false);
    action();
  };

  return (
    <>
      <button
        type="button"
        className="header-menu-toggle"
        aria-label="More"
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(true)}
      >
        ⋯
      </button>
      <dialog
        ref={ref}
        className="header-menu"
        aria-label="More"
        onClose={() => setOpen(false)}
        onClick={(e) => {
          if (e.target === e.currentTarget) setOpen(false);
        }}
      >
        {onOpenComments && (
          <button type="button" className="header-menu-item" onClick={run(onOpenComments)}>
            Notes
            {commentCount ? <span className="header-menu-count">{commentCount}</span> : null}
          </button>
        )}
        <button type="button" className="header-menu-item help-toggle" aria-keyshortcuts="?" onClick={run(openHelp)}>
          Shortcuts <kbd aria-hidden>?</kbd>
        </button>
        <div className="header-menu-item header-menu-theme">
          Theme
          <ThemeToggle />
        </div>
      </dialog>
    </>
  );
}
