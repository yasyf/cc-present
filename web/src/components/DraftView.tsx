import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import type { ThemedToken } from 'shiki/core';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Draft } from '../schema';
import type { Annotation, Interactions } from '../events';
import { usePresent } from '../present';
import { useDecidable } from '../keyboard';
import { anchorOf, formatRangeAnchor, parseAnchor, resolveAnchor } from '../anchor';
import { Clamped } from './Clamped';
import { Mark } from './Mark';

// A resolved annotation: its live line span when the anchor still resolves (moved
// carries the original line), or detached when the anchored content is gone.
interface Resolved {
  ann: Annotation;
  start: number;
  end: number;
  moved: boolean;
  from: number;
  detached: boolean;
}

function resolveAll(annotations: Annotation[], lines: string[]): Resolved[] {
  return annotations.map((ann) => {
    try {
      const r = resolveAnchor(parseAnchor(ann.anchor), lines);
      return { ann, start: r.start, end: r.end, moved: r.moved, from: r.from, detached: false };
    } catch {
      return { ann, start: 0, end: 0, moved: false, from: 0, detached: true };
    }
  });
}

// DraftView renders a draft as numbered, syntax-highlighted source lines a human
// annotates by selecting a line or range. Each annotation re-anchors against the
// current text on every render: an exact hit marks its lines, a moved anchor tags
// "was L<n>", and a vanished one drops to the Detached notes section.
export function DraftView({ block, interactions }: { block: Draft; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const locked = closed || readOnly;

  const lines = useMemo(() => block.text.split('\n'), [block.text]);
  const annotations = interactions.annotations[block.id] ?? [];
  const resolved = useMemo(() => resolveAll(annotations, lines), [annotations, lines]);

  const [tokens, setTokens] = useState<ThemedToken[][] | null>(null);
  useEffect(() => {
    let alive = true;
    setTokens(null);
    void (async () => {
      const { tokenizeLines, resolveLang } = await import('../highlight');
      const lang = resolveLang(block.lang);
      if (!lang) return;
      const out = await tokenizeLines(block.text, lang);
      if (alive) setTokens(out);
    })();
    return () => {
      alive = false;
    };
  }, [block.text, block.lang]);

  const [selection, setSelection] = useState<{ anchor: number; focus: number } | null>(null);
  const [dragging, setDragging] = useState(false);
  const [draftText, setDraftText] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const composerRef = useRef<HTMLTextAreaElement>(null);

  const span = selection
    ? { start: Math.min(selection.anchor, selection.focus), end: Math.max(selection.anchor, selection.focus) }
    : null;

  useEffect(() => {
    if (!dragging) return;
    const end = () => setDragging(false);
    window.addEventListener('mouseup', end);
    return () => window.removeEventListener('mouseup', end);
  }, [dragging]);

  useEffect(() => {
    if (selection && composerRef.current) composerRef.current.focus();
  }, [selection]);

  function openAt(line: number) {
    setEditingId(null);
    setDraftText('');
    setSelection({ anchor: line, focus: line });
  }

  function gutterDown(line: number, shift: boolean) {
    if (locked) return;
    setEditingId(null);
    if (shift && selection) {
      setSelection({ anchor: selection.anchor, focus: line });
    } else {
      setDraftText('');
      setSelection({ anchor: line, focus: line });
    }
    setDragging(true);
  }

  function gutterEnter(line: number) {
    if (dragging && selection) setSelection({ anchor: selection.anchor, focus: line });
  }

  function editAnnotation(r: Resolved) {
    if (locked) return;
    setEditingId(r.ann.id);
    setDraftText(r.ann.text);
    setSelection({ anchor: r.start, focus: r.end });
  }

  function cancel() {
    setSelection(null);
    setDraftText('');
    setEditingId(null);
  }

  async function send() {
    if (!span) return;
    const text = draftText.trim();
    if (!text) return;
    const anchor = formatRangeAnchor(span.start, span.end, anchorOf(lines[span.start - 1]!));
    const quote = lines.slice(span.start - 1, span.end).join('\n');
    const id = editingId ?? crypto.randomUUID();
    const ok = await post({ type: 'annotation.created', id, blockId: block.id, anchor, text, quote });
    if (ok) cancel();
  }

  function remove(id: string) {
    if (locked) return;
    void post({ type: 'annotation.removed', id, blockId: block.id });
  }

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'draft',
    disabled: locked,
    engage: () => openAt(span?.start ?? 1),
  });

  const marked = new Map<number, Resolved[]>();
  const attached: Resolved[] = [];
  const detached: Resolved[] = [];
  for (const r of resolved) {
    if (r.detached) {
      detached.push(r);
      continue;
    }
    attached.push(r);
    const list = marked.get(r.start) ?? [];
    list.push(r);
    marked.set(r.start, list);
  }
  attached.sort((a, b) => a.start - b.start);

  return (
    <figure className="draft-block" ref={ref} data-kbd-cursor={cursor || undefined}>
      {block.title && <figcaption className="draft-title">{block.title}</figcaption>}
      <Clamped lines={32} className="draft-lines">
        {lines.map((line, i) => {
          const n = i + 1;
          const rowTokens = tokens?.[i];
          const inSpan = span !== null && n >= span.start && n <= span.end;
          const marks = marked.get(n);
          return (
            <div
              key={n}
              className={`draft-row${inSpan ? ' in-span' : ''}${marks ? ' annotated' : ''}`}
              data-line={n}
            >
              <button
                type="button"
                className="draft-gutter"
                disabled={locked}
                aria-label={`Line ${n}`}
                onMouseDown={(e) => gutterDown(n, e.shiftKey)}
                onMouseEnter={() => gutterEnter(n)}
              >
                {n}
              </button>
              <code className="draft-text">
                {rowTokens
                  ? rowTokens.map((t, ti) => (
                      <span key={ti} className="draft-tok" style={t.htmlStyle as CSSProperties}>
                        {t.content}
                      </span>
                    ))
                  : line}
              </code>
              {marks && (
                <span className="draft-marker" aria-hidden>
                  {marks.length}
                </span>
              )}
            </div>
          );
        })}
      </Clamped>

      {selection && !locked && (
        <div className="draft-composer">
          <div className="draft-composer-range">
            {span && (span.start === span.end ? `Line ${span.start}` : `Lines ${span.start}–${span.end}`)}
          </div>
          <textarea
            ref={composerRef}
            className="draft-composer-input"
            rows={2}
            value={draftText}
            placeholder="Add a note on these lines…"
            onChange={(e) => setDraftText(e.target.value)}
            onKeyDown={(e) => {
              if (!e.repeat && (e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                e.preventDefault();
                void send();
              }
            }}
          />
          <div className="draft-composer-actions">
            <button type="button" className="primary" onClick={send}>
              {editingId ? 'Save' : 'Add note'}
            </button>
            <button type="button" onClick={cancel}>
              Cancel
            </button>
          </div>
        </div>
      )}

      {attached.length > 0 && (
        <ul className="draft-notes">
          {attached.map((r) => (
            <li key={r.ann.id} className="draft-note">
              <span className="draft-note-line">
                {r.start === r.end ? `L${r.start}` : `L${r.start}–${r.end}`}
              </span>
              <span className="draft-note-text">{r.ann.text}</span>
              {r.moved && <span className="draft-note-moved">moved · was L{r.from}</span>}
              {!locked && (
                <span className="draft-note-actions">
                  <button type="button" className="link-btn" onClick={() => editAnnotation(r)}>
                    Edit
                  </button>
                  <button type="button" className="link-btn" onClick={() => remove(r.ann.id)}>
                    Remove
                  </button>
                </span>
              )}
            </li>
          ))}
        </ul>
      )}

      {detached.length > 0 && (
        <div className="draft-detached">
          <p className="draft-detached-head">Detached notes</p>
          <ul className="draft-notes">
            {detached.map((r) => (
              <li key={r.ann.id} className="draft-note detached">
                <blockquote className="draft-note-quote">{r.ann.quote}</blockquote>
                <span className="draft-note-text">{r.ann.text}</span>
                {!locked && (
                  <button type="button" className="link-btn" onClick={() => remove(r.ann.id)}>
                    <Mark kind="cross" /> Remove
                  </button>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </figure>
  );
}
