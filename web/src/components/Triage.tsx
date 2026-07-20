import { useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Item, Triage as TriageBlock } from '../schema';
import type { Interactions, Verdict } from '../events';
import { usePresent } from '../present';
import { nextUndecided, verdictToggle } from '../decide';
import { factAxes } from '../focus';
import { useDecidable } from '../keyboard';
import { renderInlineMarkdown } from '../markdown';
import { Mark } from './Mark';
import { Clamped } from './Clamped';
import { DetailDisclosure } from './Detail';

// Triage renders a multi-item accept/reject block: each item carries its own
// verdict pair, bulk Accept all / Reject all post one full merge carrying existing
// notes forward, and an optional per-item note composer opens once the item has a
// verdict. One keyboard ring entry drives an internal item cursor.
export function Triage({ block, interactions }: { block: TriageBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const locked = closed || readOnly;

  const items = block.items;
  const allowNotes = block.allowNotes ?? true;
  const verdicts = interactions.triage[block.id] ?? {};
  const axes = factAxes(items);
  const decidedCount = items.filter((it) => verdicts[it.id] !== undefined).length;

  const [itemCursor, setItemCursor] = useState(0);
  const [noteFor, setNoteFor] = useState<string | null>(null);
  const [noteDraft, setNoteDraft] = useState('');
  const noteRef = useRef<HTMLTextAreaElement>(null);

  function postItem(itemId: string, verdict: Verdict, note?: string) {
    const entry: { verdict: Verdict; note?: string } = { verdict };
    if (note) entry.note = note;
    void post({ type: 'triage.decided', blockId: block.id, verdicts: { [itemId]: entry } });
  }

  function decide(itemId: string, target: 'approved' | 'rejected') {
    const next = verdictToggle(verdicts[itemId]?.verdict, target);
    // A verdict flip carries the existing note forward; a clear drops it.
    const note = next === 'cleared' ? undefined : verdicts[itemId]?.note || undefined;
    postItem(itemId, next, note);
  }

  function clearItem(itemId: string) {
    if (verdicts[itemId]) postItem(itemId, 'cleared');
  }

  function bulk(target: 'approved' | 'rejected') {
    const merge: Record<string, { verdict: Verdict; note?: string }> = {};
    for (const it of items) {
      const note = verdicts[it.id]?.note;
      merge[it.id] = note ? { verdict: target, note } : { verdict: target };
    }
    void post({ type: 'triage.decided', blockId: block.id, verdicts: merge });
  }

  function openNote(itemId: string) {
    setNoteFor(itemId);
    setNoteDraft(verdicts[itemId]?.note ?? '');
    requestAnimationFrame(() => noteRef.current?.focus());
  }

  function sendNote(itemId: string) {
    const verdict = verdicts[itemId]?.verdict;
    if (!verdict || verdict === 'cleared') return;
    postItem(itemId, verdict, noteDraft.trim() || undefined);
    setNoteFor(null);
    setNoteDraft('');
  }

  function advanceFrom(itemId: string) {
    const ids = items.map((it) => it.id);
    const undecided = new Set(items.filter((it) => it.id !== itemId && verdicts[it.id] === undefined).map((it) => it.id));
    const next = nextUndecided(ids, undecided, itemId);
    if (next) setItemCursor(ids.indexOf(next));
  }

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'triage',
    disabled: locked,
    verdict: (target) => {
      const it = items[itemCursor];
      if (!it) return;
      decide(it.id, target);
      advanceFrom(it.id);
    },
    clear: () => {
      const it = items[itemCursor];
      if (it) clearItem(it.id);
    },
    choose: (n) => {
      if (n >= 1 && n <= items.length) setItemCursor(n - 1);
    },
    engage: () => {
      const it = items[itemCursor];
      if (!it || !allowNotes) return;
      const verdict = verdicts[it.id]?.verdict;
      if (verdict && verdict !== 'cleared') openNote(it.id);
    },
  });

  return (
    <div className="triage" ref={ref} data-kbd-cursor={cursor || undefined}>
      <div className="triage-head">
        {block.prompt && <p className="triage-prompt">{block.prompt}</p>}
        <div className="triage-head-row">
          <span className="triage-progress">
            {decidedCount} of {items.length} decided
          </span>
          {!locked && (
            <div className="triage-bulk">
              <button type="button" className="triage-bulk-btn" onClick={() => bulk('approved')}>
                Accept all
              </button>
              <button type="button" className="triage-bulk-btn" onClick={() => bulk('rejected')}>
                Reject all
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="triage-items">
        {items.map((item, i) => (
          <TriageRow
            key={item.id}
            item={item}
            index={i}
            axes={axes}
            verdict={verdicts[item.id]?.verdict}
            note={verdicts[item.id]?.note}
            allowNotes={allowNotes}
            locked={locked}
            showIndex={cursor && i < 9}
            cursored={cursor && i === itemCursor}
            interactions={interactions}
            onDecide={(target) => decide(item.id, target)}
            noteOpen={noteFor === item.id}
            noteDraft={noteDraft}
            noteRef={noteRef}
            onNoteOpen={() => openNote(item.id)}
            onNoteChange={setNoteDraft}
            onNoteSend={() => sendNote(item.id)}
            onNoteCancel={() => setNoteFor(null)}
          />
        ))}
      </div>
    </div>
  );
}

interface TriageRowProps {
  item: Item;
  index: number;
  axes: string[] | null;
  verdict?: Verdict;
  note?: string;
  allowNotes: boolean;
  locked: boolean;
  showIndex: boolean;
  cursored: boolean;
  interactions: Interactions;
  onDecide: (target: 'approved' | 'rejected') => void;
  noteOpen: boolean;
  noteDraft: string;
  noteRef: React.RefObject<HTMLTextAreaElement | null>;
  onNoteOpen: () => void;
  onNoteChange: (text: string) => void;
  onNoteSend: () => void;
  onNoteCancel: () => void;
}

function TriageRow({
  item,
  index,
  axes,
  verdict,
  note,
  allowNotes,
  locked,
  showIndex,
  cursored,
  interactions,
  onDecide,
  noteOpen,
  noteDraft,
  noteRef,
  onNoteOpen,
  onNoteChange,
  onNoteSend,
  onNoteCancel,
}: TriageRowProps) {
  const decided = verdict === 'approved' || verdict === 'rejected';
  const noteChannel = allowNotes && decided;
  return (
    <div className={`triage-item${cursored ? ' cursor' : ''}`} data-index={index + 1}>
      <div className="triage-item-main">
        {showIndex && (
          <kbd className="triage-index" aria-hidden>
            {index + 1}
          </kbd>
        )}
        <div className="triage-item-body">
          <span className="triage-item-label">{item.label}</span>
          {item.hint && (
            <div className="triage-item-hint" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(item.hint) }} />
          )}
          {item.md && <Clamped html={renderInlineMarkdown(item.md)} lines={3} className="triage-item-md prose" />}
          {item.facts && item.facts.length > 0 && (
            <span
              className="triage-facts"
              data-aligned={axes ? '' : undefined}
              style={axes ? ({ '--fact-count': axes.length } as CSSProperties) : undefined}
            >
              {item.facts.map((fact, fi) => (
                <span key={fi} className={`fact fact-${fact.tone ?? 'default'}`}>
                  <span className="fact-value">{fact.value}</span>
                  {fact.label && <span className="fact-label">{fact.label}</span>}
                </span>
              ))}
            </span>
          )}
          {(item.detail || item.visual) && (
            <DetailDisclosure detail={item.detail} visual={item.visual} interactions={interactions} />
          )}
        </div>
        <div className="verdict-pair triage-verdicts" role="radiogroup" aria-label={item.label}>
          <button
            type="button"
            role="radio"
            aria-checked={verdict === 'approved'}
            disabled={locked}
            className={`verdict verdict-approve${verdict === 'approved' ? ' active' : ''}`}
            onClick={() => onDecide('approved')}
          >
            <span className="verdict-glyph" aria-hidden>
              {verdict === 'approved' ? <Mark kind="check" /> : '✓'}
            </span>
            Approve
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={verdict === 'rejected'}
            disabled={locked}
            className={`verdict verdict-reject${verdict === 'rejected' ? ' active' : ''}`}
            onClick={() => onDecide('rejected')}
          >
            <span className="verdict-glyph" aria-hidden>
              {verdict === 'rejected' ? <Mark kind="cross" /> : '✕'}
            </span>
            Reject
          </button>
        </div>
      </div>

      {note !== undefined && note !== '' && !noteOpen && (
        <div className="triage-note">
          <span className="triage-note-text">{note}</span>
        </div>
      )}

      {noteChannel && !locked && (
        <div className="triage-note-affordance">
          {noteOpen ? (
            <div className="triage-note-editor">
              <textarea
                ref={noteRef}
                className="triage-note-input"
                rows={2}
                value={noteDraft}
                placeholder="Add a note for the agent…"
                onChange={(e) => onNoteChange(e.target.value)}
                onKeyDown={(e) => {
                  if (!e.repeat && (e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                    e.preventDefault();
                    onNoteSend();
                  }
                }}
              />
              <div className="triage-note-actions">
                <button type="button" className="primary" onClick={onNoteSend}>
                  Save
                </button>
                <button type="button" onClick={onNoteCancel}>
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <button type="button" className="link-btn" onClick={onNoteOpen}>
              {note ? 'Edit note' : 'Add note'}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
