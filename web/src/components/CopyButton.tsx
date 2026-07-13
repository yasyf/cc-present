import { useEffect, useRef, useState } from 'react';
import { Mark } from './Mark';

// CopyButton writes text to the clipboard and swaps its glyph to a drawn check
// for ~1.2s — the human's copy is a pencil mark like every other commitment.
export function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  const onCopy = () => {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 1200);
    });
  };

  return (
    <button type="button" className="copy-button" onClick={onCopy} aria-label="Copy code">
      {copied ? (
        <Mark kind="check" className="copy-check" />
      ) : (
        <svg className="copy-glyph" viewBox="0 0 24 24" fill="none" aria-hidden focusable="false">
          <rect x="8.5" y="8.5" width="11" height="11" rx="2" />
          <path d="M15.5 5.5H6.5A1.5 1.5 0 0 0 5 7v9" />
        </svg>
      )}
    </button>
  );
}
