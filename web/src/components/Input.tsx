import type { Input as InputBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';

export function Input({ block, interactions }: { block: InputBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  // The committed value is the source of truth; the field is uncontrolled and
  // keyed on it so an echoed change re-seeds defaultValue without mirroring the
  // committed text into component state. Only blur (or Enter) commits, never a
  // keystroke.
  const committed = interactions.inputs[block.id]?.text ?? '';

  function save(value: string) {
    if (value === committed) return;
    post({ type: 'input.submitted', blockId: block.id, text: value });
  }

  return (
    <label className="input-block">
      <span className="input-label">{block.label}</span>
      {block.multiline ? (
        <textarea
          key={committed}
          defaultValue={committed}
          placeholder={block.placeholder}
          rows={3}
          disabled={closed}
          onBlur={(e) => save(e.currentTarget.value)}
        />
      ) : (
        <input
          key={committed}
          type="text"
          defaultValue={committed}
          placeholder={block.placeholder}
          disabled={closed}
          onBlur={(e) => save(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') e.currentTarget.blur();
          }}
        />
      )}
    </label>
  );
}
