import { useEffect, useId, useRef } from 'react';
import { Mark } from './Mark';

// Lightbox shows an image in a native modal <dialog>; the browser owns Escape,
// the backdrop, and focus restoration. `onClose` fires via the native `close`.
export function Lightbox({
  open,
  onClose,
  src,
  alt,
  caption,
}: {
  open: boolean;
  onClose: () => void;
  src: string;
  alt: string;
  caption?: string;
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
      aria-label={caption ? undefined : alt}
      aria-labelledby={caption ? captionId : undefined}
      onClick={(e) => {
        if (e.target === ref.current) ref.current.close();
      }}
    >
      <button type="button" className="lightbox-close" aria-label="Close" onClick={() => ref.current?.close()}>
        <Mark kind="cross" />
      </button>
      <figure className="lightbox-figure">
        <img className="lightbox-img" src={src} alt={alt} />
        {caption && (
          <figcaption id={captionId} className="lightbox-caption">
            {caption}
          </figcaption>
        )}
      </figure>
    </dialog>
  );
}
