import { useCallback } from 'react';
import { tokens } from './host/present';
import type { PackComponentProps } from './host/present';

export function Rating({ block, value, submit, disabled }: PackComponentProps) {
  const t = tokens();
  const label = block.label as string;
  const scale = (block.scale as number | undefined) ?? 5;
  const current = (value as { value?: number } | null | undefined)?.value;
  const pick = useCallback((n: number) => submit({ value: n }), [submit]);
  const points = Array.from({ length: scale }, (_, i) => i + 1);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
      <div
        style={{
          fontFamily: t.fontMono,
          textTransform: 'uppercase',
          letterSpacing: t.trackCaps,
          fontSize: '0.7rem',
          color: t.dim,
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
                borderRadius: t.radiusMd,
                border: `1px solid ${selected ? t.accent : t.border}`,
                background: selected ? t.accent : t.surface,
                color: selected ? t.accentFg : t.text,
                fontFamily: t.fontMono,
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
