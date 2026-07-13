import { useCallback } from 'react';
import type { CSSProperties } from 'react';
import { toast, tokens, usePackState } from './host/present';
import type { PackComponentProps } from './host/present';

interface SurveyStep {
  prompt: string;
  placeholder?: string;
}

// Survey is a two-step wizard whose per-step drafts and step index live in
// ui.usePackState, so they survive remounts and board↔focus navigation.
export function Survey({ block, value, submit, disabled, context }: PackComponentProps) {
  const t = tokens();
  const title = block.title as string;
  const steps = block.steps as [SurveyStep, SurveyStep];
  const last = steps.length - 1;

  const committed = value as { summary?: string; detail?: string } | null | undefined;

  const [step, setStep] = usePackState<number>('step', 0);
  const [summary, setSummary] = usePackState<string>('summary', committed?.summary ?? '');
  const [detail, setDetail] = usePackState<string>('detail', committed?.detail ?? '');
  const current = step === 0 ? steps[0] : steps[1];
  const draft = step === 0 ? summary : detail;
  const setDraft = step === 0 ? setSummary : setDetail;

  const onSubmit = useCallback(() => {
    submit({ ...(committed ?? {}), summary, detail });
    toast({ kind: 'info', text: 'Survey sent' });
  }, [submit, committed, summary, detail]);

  const caps: CSSProperties = {
    fontFamily: t.fontMono,
    fontSize: '0.7rem',
    letterSpacing: t.trackCaps,
    textTransform: 'uppercase',
    color: t.dim,
  };
  const button = (primary: boolean): CSSProperties => ({
    minWidth: '4.5rem',
    padding: '0.5rem 0.9rem',
    fontFamily: t.fontProse,
    fontSize: '0.85rem',
    borderRadius: t.radiusMd,
    border: `1px solid ${primary ? t.accent : t.border}`,
    background: primary ? t.accent : t.surface,
    color: primary ? t.accentFg : t.text,
    cursor: disabled ? 'not-allowed' : 'pointer',
    opacity: disabled ? 0.55 : 1,
  });
  const footer = context.closed ? 'Board closed' : context.roundOver ? 'Round over' : `Round ${context.round}`;

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: '0.75rem',
        width: '100%',
        boxSizing: 'border-box',
        padding: '1rem',
        background: t.surface,
        color: t.text,
        border: `1px solid ${t.border}`,
        borderRadius: t.radiusLg,
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: '0.5rem' }}>
        <div style={{ fontFamily: t.fontProse, fontWeight: 600, fontSize: '1rem' }}>{title}</div>
        <span style={caps}>
          Step {step + 1} / {steps.length}
        </span>
      </div>

      <label style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem' }}>
        <span style={{ fontFamily: t.fontProse, fontSize: '0.9rem' }}>{current.prompt}</span>
        <textarea
          rows={3}
          value={draft}
          placeholder={current.placeholder}
          disabled={disabled}
          onChange={(e) => setDraft(e.target.value)}
          style={{
            width: '100%',
            boxSizing: 'border-box',
            resize: 'vertical',
            padding: '0.5rem 0.65rem',
            fontFamily: t.fontProse,
            fontSize: '0.9rem',
            color: t.text,
            background: t.bg,
            border: `1px solid ${t.border}`,
            borderRadius: t.radiusMd,
          }}
        />
      </label>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.5rem' }}>
        <span style={caps}>{footer}</span>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          {step > 0 && (
            <button type="button" disabled={disabled} onClick={() => setStep(step - 1)} style={button(false)}>
              Back
            </button>
          )}
          {step < last ? (
            <button type="button" disabled={disabled} onClick={() => setStep(step + 1)} style={button(true)}>
              Next
            </button>
          ) : (
            <button type="button" disabled={disabled} onClick={onSubmit} style={button(true)}>
              Submit
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
