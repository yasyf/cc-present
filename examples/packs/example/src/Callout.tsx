import { renderMarkdown } from './host/present';
import type { PackComponentProps } from './host/present';

type Tone = 'info' | 'warn' | 'success';

const TONE_COLOR: Record<Tone, string> = {
  info: 'var(--accent)',
  warn: 'var(--warn)',
  success: 'var(--ok)',
};

export function Callout({ block }: PackComponentProps) {
  const tone = (block.tone as Tone | undefined) ?? 'info';
  const md = block.md as string;
  const color = TONE_COLOR[tone];
  return (
    <div
      role="note"
      style={{
        borderLeft: `3px solid ${color}`,
        background: `color-mix(in srgb, ${color} 8%, var(--surface))`,
        color: 'var(--text)',
        borderRadius: 'var(--radius-md)',
        padding: '0.75rem 1rem',
      }}
    >
      <div
        style={{
          fontFamily: 'var(--font-mono)',
          textTransform: 'uppercase',
          letterSpacing: '0.1em',
          fontSize: '0.7rem',
          color,
          marginBottom: '0.35rem',
        }}
      >
        {tone}
      </div>
      <div dangerouslySetInnerHTML={{ __html: renderMarkdown(md) }} />
    </div>
  );
}
