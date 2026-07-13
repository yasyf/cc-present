import { renderMarkdown, tokens } from './host/present';
import type { PackComponentProps } from './host/present';

type Tone = 'info' | 'warn' | 'success';

export function Callout({ block }: PackComponentProps) {
  const t = tokens();
  const tone = (block.tone as Tone | undefined) ?? 'info';
  const md = block.md as string;
  const color = { info: t.accent, warn: t.warn, success: t.ok }[tone];
  return (
    <div
      role="note"
      style={{
        borderLeft: `3px solid ${color}`,
        background: `color-mix(in srgb, ${color} 8%, ${t.surface})`,
        color: t.text,
        borderRadius: t.radiusMd,
        padding: '0.75rem 1rem',
      }}
    >
      <div
        style={{
          fontFamily: t.fontMono,
          textTransform: 'uppercase',
          letterSpacing: t.trackCaps,
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
