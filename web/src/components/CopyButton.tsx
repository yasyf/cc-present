import { useEffect, useRef, useState } from 'react';
import { Mark } from './Mark';

type CopyState = 'idle' | 'copied' | 'failed';

// CopyButton copies text and holds a drawn check for ~1.2s; a denied write draws
// a cross, and an origin without the Clipboard API renders nothing.
export function CopyButton({ text }: { text: string }) {
  const [state, setState] = useState<CopyState>('idle');
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  if (!navigator.clipboard) return null;

  const hold = (next: CopyState) => {
    setState(next);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setState('idle'), 1200);
  };

  const onCopy = () => {
    navigator.clipboard.writeText(text).then(
      () => hold('copied'),
      () => hold('failed'),
    );
  };

  const glyph =
    state === 'copied' ? (
      <Mark kind="check" className="copy-check" />
    ) : state === 'failed' ? (
      <Mark kind="cross" className="copy-cross" />
    ) : (
      <svg className="copy-glyph" viewBox="0 0 24 24" fill="none" aria-hidden focusable="false">
        <rect x="8.5" y="8.5" width="11" height="11" rx="2" />
        <path d="M15.5 5.5H6.5A1.5 1.5 0 0 0 5 7v9" />
      </svg>
    );

  return (
    <button type="button" className="copy-button" onClick={onCopy} aria-label="Copy code">
      {glyph}
    </button>
  );
}
