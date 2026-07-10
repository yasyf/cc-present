import { useCallback, useRef } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { Input as InputBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { useDecidable } from '../keyboard';

export function Input({ block, interactions }: { block: InputBlock; interactions: Interactions }) {
  const { post, closed, currentRound } = usePresent();
  const readOnly = useGroupReadOnly();
  const fieldRef = useRef<HTMLElement | null>(null);
  const attachField = useCallback((el: HTMLElement | null) => {
    fieldRef.current = el;
  }, []);
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
  }

  return (
    <label className="input-block" ref={ref} data-kbd-cursor={cursor || undefined}>
      <span className="input-label">{block.label}</span>
      {!readOnly && v && v.round < currentRound && (
        <div className="input-last-round">last round: {v.text}</div>
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
    </label>
  );
}
