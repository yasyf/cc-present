import { useEffect, useId, useRef } from 'react';
import type { ReactNode } from 'react';
import { Mark } from './Mark';

// Lightbox shows its children enlarged in a native modal <dialog>; the browser owns
// Escape, the backdrop, and focus restoration. `onClose` fires via the native `close`.
export function Lightbox({
  open,
  onClose,
  label,
  caption,
  children,
}: {
  open: boolean;
  onClose: () => void;
  label: string;
  caption?: string;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const captionId = useId();

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    else if (!open && dialog.open) dialog.close();
  }, [open]);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    dialog.addEventListener('close', onClose);
    return () => dialog.removeEventListener('close', onClose);
  }, [onClose]);

  return (
    <dialog
      ref={ref}
      className="lightbox"
      aria-label={caption ? undefined : label}
      aria-labelledby={caption ? captionId : undefined}
      onClick={(e) => {
        if (e.target === ref.current) ref.current.close();
      }}
    >
      <button type="button" className="lightbox-close" aria-label="Close" onClick={() => ref.current?.close()}>
        <Mark kind="cross" />
      </button>
      <figure className="lightbox-figure">
        {children}
        {caption && (
          <figcaption id={captionId} className="lightbox-caption">
            {caption}
          </figcaption>
        )}
      </figure>
    </dialog>
  );
}
