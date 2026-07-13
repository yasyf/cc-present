// ThemeTokens is the frozen ui.tokens contract: CSS-variable references (never
// resolved colors) so pack inline styles re-ink under theme flips.

export interface ThemeTokens {
  readonly bg: string;
  readonly bgSoft: string;
  readonly surface: string;
  readonly surfaceRaised: string;
  readonly text: string;
  readonly dim: string;
  readonly border: string;
  readonly borderStrong: string;
  readonly accent: string;
  readonly accentFg: string;
  readonly ok: string;
  readonly warn: string;
  readonly danger: string;
  readonly focusRing: string;
  readonly radiusSm: string;
  readonly radiusMd: string;
  readonly radiusLg: string;
  readonly fontProse: string;
  readonly fontMono: string;
  readonly trackCaps: string;
}

// tokens is the single frozen ThemeTokens instance handed to packs as ui.tokens.
export const tokens: ThemeTokens = Object.freeze({
  bg: 'var(--bg)',
  bgSoft: 'var(--bg-soft)',
  surface: 'var(--surface)',
  surfaceRaised: 'var(--surface-raised)',
  text: 'var(--text)',
  dim: 'var(--dim)',
  border: 'var(--border)',
  borderStrong: 'var(--border-strong)',
  accent: 'var(--accent)',
  accentFg: 'var(--accent-fg)',
  ok: 'var(--ok)',
  warn: 'var(--warn)',
  danger: 'var(--danger)',
  focusRing: 'var(--focus-ring)',
  radiusSm: 'var(--radius-sm)',
  radiusMd: 'var(--radius-md)',
  radiusLg: 'var(--radius-lg)',
  fontProse: 'var(--font-prose)',
  fontMono: 'var(--font-mono)',
  trackCaps: 'var(--track-caps)',
} as const);
