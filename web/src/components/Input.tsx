import { useCallback, useEffect, useRef, useState } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Input as InputBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { useDecidable } from '../keyboard';
import { Mark } from './Mark';

export function Input({ block, interactions }: { block: InputBlock; interactions: Interactions }) {
  const { post, closed, currentRound } = usePresent();
  const readOnly = useGroupReadOnly();
  const fieldRef = useRef<HTMLElement | null>(null);
  const attachField = useCallback((el: HTMLElement | null) => {
    fieldRef.current = el;
  }, []);
  // A commit stamps a drawn tick and sweeps a pencil underline for ~1.2s; the
  // timestamp keys the marks so a second commit replays them. Zero = at rest.
  const [committedAt, setCommittedAt] = useState(0);
  useEffect(() => {
    if (committedAt === 0) return;
    const t = setTimeout(() => setCommittedAt(0), 1200);
    return () => clearTimeout(t);
  }, [committedAt]);
  const { ref, cursor } = useDecidable(block.id, {
    kind: 'input',
    disabled: closed || readOnly,
    engage: () => fieldRef.current?.focus(),
  });
  // The committed value is the source of truth; the field is uncontrolled and
  // keyed on it so an echoed change re-seeds defaultValue without mirroring the
  // committed text into component state. Only blur (or plain Enter) commits, never
  // a keystroke. In a live round the base value clears once the round advances past
  // the entry, so each round starts fresh; a read-only group always shows its
  // frozen text.
  const v = interactions.inputs[block.id];
  const committed = v && (readOnly || v.round === currentRound) ? v.text : '';

  function save(value: string) {
    if (value === committed) return;
    post({ type: 'input.submitted', blockId: block.id, text: value });
    setCommittedAt(Date.now());
  }

  const committing = committedAt !== 0;

  return (
    <label className="input-block" ref={ref} data-kbd-cursor={cursor || undefined}>
      <span className="input-label">
        {block.label}
        {committing && <Mark key={committedAt} kind="check" className="input-tick" />}
      </span>
      {!readOnly && v && v.round < currentRound && (
        <div className="input-last-round">
          <span className="input-last-round-caret" aria-hidden>
            ↳
          </span>
          <span className="input-last-round-text">{v.text}</span>
        </div>
      )}
      {block.multiline ? (
        <textarea
          key={committed}
          ref={attachField}
          defaultValue={committed}
          placeholder={block.placeholder}
          rows={3}
          disabled={closed || readOnly}
          onBlur={(e) => save(e.currentTarget.value)}
        />
      ) : (
        <input
          key={committed}
          ref={attachField}
          type="text"
          defaultValue={committed}
          placeholder={block.placeholder}
          disabled={closed || readOnly}
          onBlur={(e) => save(e.currentTarget.value)}
          onKeyDown={(e) => {
            // Plain Enter commits; a mod+Enter chord is left to the global
            // submit path (which blurs first) so the draft posts only once.
            if (e.key === 'Enter' && !e.metaKey && !e.ctrlKey) e.currentTarget.blur();
          }}
        />
      )}
      {committing && <span key={committedAt} className="input-sweep" aria-hidden />}
    </label>
  );
}
