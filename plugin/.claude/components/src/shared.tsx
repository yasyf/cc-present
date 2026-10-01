import type { CSSProperties, ReactNode } from 'react';
import { tokens } from './host/present';

export function prUrl(repo: string, number: number): string {
  return `https://github.com/${repo}/pull/${number}`;
}

export function commitUrl(repo: string, sha: string): string {
  return `https://github.com/${repo}/commit/${sha}`;
}

export function shortSha(sha: string): string {
  return sha.slice(0, 10);
}

// fetchedLabel renders an ISO instant as UTC minutes, the precision a board needs.
export function fetchedLabel(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `fetched ${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}Z`;
}

export function Link({ href, children, style }: { href: string; children: ReactNode; style?: CSSProperties }) {
  const t = tokens();
  return (
    <a href={href} target="_blank" rel="noreferrer" style={{ color: t.accent, textDecoration: 'none', ...style }}>
      {children}
    </a>
  );
}

export function Pill({ color, children }: { color: string; children: ReactNode }) {
  const t = tokens();
  return (
    <span
      style={{
        display: 'inline-block',
        fontFamily: t.fontMono,
        fontSize: '0.7rem',
        textTransform: 'uppercase',
        letterSpacing: t.trackCaps,
        color,
        border: `1px solid color-mix(in srgb, ${color} 45%, transparent)`,
        background: `color-mix(in srgb, ${color} 10%, transparent)`,
        borderRadius: t.radiusSm,
        padding: '0.05rem 0.4rem',
        whiteSpace: 'nowrap',
      }}
    >
      {children}
    </span>
  );
}

export function Meta({ children }: { children: ReactNode }) {
  const t = tokens();
  return <div style={{ color: t.dim, fontFamily: t.fontMono, fontSize: '0.72rem' }}>{children}</div>;
}
