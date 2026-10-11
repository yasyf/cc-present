import { useCallback, useEffect, useRef, useState } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Input as InputBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { draftStore, useDraft } from '../drafts';
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
  // In a live round the base value clears once the round advances past the entry,
  // so each round starts fresh; a read-only group always shows its frozen text.
  const v = interactions.inputs[block.id];
  const committed = v && (readOnly || v.round === currentRound) ? v.text : '';
  const typed = useDraft<string>(block.id, 'input');
  const shown = readOnly ? committed : (typed ?? committed);

  function edit(value: string) {
    if (value === committed) draftStore.clear(block.id, 'input');
    else draftStore.set(block.id, 'input', value);
  }

  async function save(value: string) {
    if (value === committed) {
      draftStore.clear(block.id, 'input');
      return;
    }
    // Stamp optimistically, then retract the tick if the post is rejected so the
    // rolled-back value and the mark agree; the draft keeps the rejected text.
    const epoch = draftStore.epoch;
    setCommittedAt(Date.now());
    const ok = await post({ type: 'input.submitted', blockId: block.id, text: value });
    const latest = draftStore.get<string>(block.id, 'input');
    if (ok) {
      if (latest === value) draftStore.clear(block.id, 'input');
      return;
    }
    setCommittedAt(0);
    if (latest === undefined && draftStore.epoch === epoch) draftStore.set(block.id, 'input', value);
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
          ref={attachField}
          value={shown}
          placeholder={block.placeholder}
          rows={3}
          disabled={closed || readOnly}
          onChange={(e) => edit(e.target.value)}
          onBlur={(e) => save(e.currentTarget.value)}
        />
      ) : (
        <input
          ref={attachField}
          type="text"
          value={shown}
          placeholder={block.placeholder}
          disabled={closed || readOnly}
          onChange={(e) => edit(e.target.value)}
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
