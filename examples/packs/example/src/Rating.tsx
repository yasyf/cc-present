import { useCallback } from 'react';
import type { PackComponentProps } from './host/present';

export function Rating({ block, value, submit, disabled }: PackComponentProps) {
  const label = block.label as string;
  const scale = (block.scale as number | undefined) ?? 5;
  const current = (value as { value?: number } | null | undefined)?.value;
  const pick = useCallback((n: number) => submit({ value: n }), [submit]);
  const points = Array.from({ length: scale }, (_, i) => i + 1);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
      <div
        style={{
          fontFamily: 'var(--font-mono)',
          textTransform: 'uppercase',
          letterSpacing: '0.1em',
          fontSize: '0.7rem',
          color: 'var(--muted)',
        }}
      >
        {label}
      </div>
      <div style={{ display: 'flex', gap: '0.375rem', flexWrap: 'wrap' }}>
        {points.map((n) => {
          const selected = current === n;
          return (
            <button
              key={n}
              type="button"
              disabled={disabled}
              aria-pressed={selected}
              onClick={() => pick(n)}
              style={{
                minWidth: '2.5rem',
                padding: '0.5rem 0.25rem',
                cursor: disabled ? 'not-allowed' : 'pointer',
                borderRadius: 'var(--radius-md)',
                border: `1px solid ${selected ? 'var(--accent)' : 'var(--border)'}`,
                background: selected ? 'var(--accent)' : 'var(--surface)',
                color: selected ? 'var(--accent-fg)' : 'var(--text)',
                fontFamily: 'var(--font-mono)',
                fontSize: '0.85rem',
                opacity: disabled ? 0.55 : 1,
              }}
            >
              {n}
            </button>
          );
        })}
      </div>
    </div>
  );
}
