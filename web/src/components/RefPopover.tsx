import { useCallback, useEffect, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import { createPortal } from 'react-dom';
import { loadSnippet, placePopover } from '../refs';
import type { Snippet } from '../refs';
import { renderMarkdown } from '../markdown';
import { Code } from './Code';

const OPEN_DELAY = 150;
const CLOSE_DELAY = 200;
const WIDTH = 560;
const MAX_HEIGHT = 420;

interface Open {
  anchor: HTMLElement;
  sha: string;
  pinned: boolean;
}

function refAt(target: EventTarget | null): HTMLElement | null {
  return target instanceof Element ? target.closest<HTMLElement>('a.ref[data-ref]') : null;
}

function coarsePointer(): boolean {
  return window.matchMedia('(pointer: coarse)').matches;
}

// RefPopover is the board-wide hover card for reference chips. A touch tap, or
// a click on a chip with no URL, pins it open instead of navigating.
export function RefPopover() {
  const [open, setOpen] = useState<Open | null>(null);
  const [snippet, setSnippet] = useState<{ sha: string; value: Snippet | Error } | null>(null);
  const openTimer = useRef<number>(undefined);
  const closeTimer = useRef<number>(undefined);
  const popRef = useRef<HTMLDivElement>(null);
  const openRef = useRef(open);
  openRef.current = open;

  const cancel = useCallback(() => {
    window.clearTimeout(openTimer.current);
    window.clearTimeout(closeTimer.current);
  }, []);
  const show = useCallback(
    (anchor: HTMLElement, pinned: boolean, delay: number) => {
      cancel();
      openTimer.current = window.setTimeout(() => setOpen({ anchor, sha: anchor.dataset.ref!, pinned }), delay);
    },
    [cancel],
  );
  const hide = useCallback(
    (force: boolean) => {
      window.clearTimeout(openTimer.current);
      if (!force && openRef.current?.pinned) return;
      window.clearTimeout(closeTimer.current);
      closeTimer.current = window.setTimeout(() => setOpen(null), force ? 0 : CLOSE_DELAY);
    },
    [],
  );

  useEffect(() => {
    const over = (e: PointerEvent) => {
      if (e.pointerType === 'touch') return;
      const a = refAt(e.target);
      if (a && openRef.current?.anchor !== a) show(a, false, OPEN_DELAY);
      else if (a || popRef.current?.contains(e.target as Node)) window.clearTimeout(closeTimer.current);
    };
    const out = (e: PointerEvent) => {
      if (e.pointerType === 'touch') return;
      const to = e.relatedTarget as Node | null;
      if (refAt(e.target) || popRef.current?.contains(e.target as Node)) {
        if (to && (popRef.current?.contains(to) || refAt(to) === openRef.current?.anchor)) return;
        hide(false);
      }
    };
    const click = (e: MouseEvent) => {
      const a = refAt(e.target);
      if (a) {
        const current = openRef.current;
        const pinnedHere = current?.pinned === true && current.anchor === a;
        if (a.hasAttribute('href') && (!coarsePointer() || pinnedHere)) return;
        e.preventDefault();
        if (pinnedHere) hide(true);
        else show(a, true, 0);
        return;
      }
      if (openRef.current && !popRef.current?.contains(e.target as Node)) hide(true);
    };
    const focusIn = (e: FocusEvent) => {
      const a = refAt(e.target);
      if (a) show(a, false, 0);
    };
    const focusOut = (e: FocusEvent) => {
      if (refAt(e.target) && !popRef.current?.contains(e.relatedTarget as Node)) hide(false);
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && openRef.current) {
        hide(true);
        return;
      }
      const a = refAt(e.target);
      if (a && !a.hasAttribute('href') && (e.key === 'Enter' || e.key === ' ')) {
        e.preventDefault();
        show(a, true, 0);
      }
    };
    const scroll = (e: Event) => {
      if (openRef.current && !popRef.current?.contains(e.target as Node)) hide(true);
    };
    document.addEventListener('pointerover', over);
    document.addEventListener('pointerout', out);
    document.addEventListener('click', click);
    document.addEventListener('focusin', focusIn);
    document.addEventListener('focusout', focusOut);
    document.addEventListener('keydown', key);
    window.addEventListener('scroll', scroll, true);
    return () => {
      cancel();
      document.removeEventListener('pointerover', over);
      document.removeEventListener('pointerout', out);
      document.removeEventListener('click', click);
      document.removeEventListener('focusin', focusIn);
      document.removeEventListener('focusout', focusOut);
      document.removeEventListener('keydown', key);
      window.removeEventListener('scroll', scroll, true);
    };
  }, [show, hide, cancel]);

  useEffect(() => {
    if (!open) return;
    let alive = true;
    loadSnippet(open.sha).then(
      (value) => alive && setSnippet({ sha: open.sha, value }),
      (err: Error) => alive && setSnippet({ sha: open.sha, value: err }),
    );
    return () => {
      alive = false;
    };
  }, [open]);

  if (!open || !open.anchor.isConnected) return null;
  const width = Math.min(WIDTH, window.innerWidth - 16);
  const place = placePopover(
    open.anchor.getBoundingClientRect(),
    { width: window.innerWidth, height: window.innerHeight },
    { width, maxHeight: MAX_HEIGHT },
  );
  const style = { top: place.top, left: place.left, width, '--ref-max-h': `${MAX_HEIGHT}px` } as CSSProperties;
  const current = snippet?.sha === open.sha ? snippet.value : null;
  return createPortal(
    <div
      ref={popRef}
      className={`ref-popover${place.above ? ' above' : ''}`}
      style={style}
      role="dialog"
      aria-label={current && !(current instanceof Error) ? current.title ?? current.label : 'Reference'}
    >
      {current === null ? (
        <p className="ref-loading">Loading…</p>
      ) : current instanceof Error ? (
        <p className="ref-error">{current.message}</p>
      ) : (
        <SnippetCard snippet={current} />
      )}
    </div>,
    document.body,
  );
}

function SnippetCard({ snippet }: { snippet: Snippet }) {
  const sha = snippet.sha?.slice(0, 11);
  return (
    <>
      <header className="ref-head">
        <span className="ref-kind">{snippet.kind === 'code' ? 'Code' : snippet.kind === 'record' ? 'Record' : 'Commit'}</span>
        <span className="ref-title">{snippet.kind === 'code' ? snippet.path : snippet.title}</span>
        {sha && snippet.kind === 'code' && <span className="ref-sha">{sha}</span>}
        {snippet.url && (
          <a className="ref-open" href={snippet.url} target="_blank" rel="noopener noreferrer">
            Open on GitHub ↗
          </a>
        )}
      </header>
      <div className="ref-body">
        {snippet.kind === 'code' ? (
          snippet.chunks?.map((c, i) => (
            <Code
              key={c.start}
              block={{
                id: `ref-${i}`,
                type: 'code',
                lang: snippet.lang ?? 'text',
                code: c.code,
                start: c.start,
                highlight: snippet.highlight,
              }}
            />
          ))
        ) : (
          <>
            {(snippet.author || snippet.date || sha) && (
              <p className="ref-meta">
                {[snippet.kind === 'commit' ? sha : snippet.label, snippet.author, snippet.date].filter(Boolean).join(' · ')}
              </p>
            )}
            {snippet.tags && snippet.tags.length > 0 && (
              <p className="ref-tags">
                {snippet.tags.map((t) => (
                  <span key={t} className="ref-tag">
                    {t}
                  </span>
                ))}
              </p>
            )}
            {snippet.body && (
              <div className="prose ref-text" dangerouslySetInnerHTML={{ __html: renderMarkdown(snippet.body) }} />
            )}
          </>
        )}
      </div>
    </>
  );
}
