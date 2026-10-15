import type { FocusStep } from '../focus';
import type { Interactions } from '../events';
import { Button } from './Button';
import { StepDots } from './StepDots';

export interface WizardFooterProps {
  steps: FocusStep[];
  index: number;
  total: number;
  advancing: boolean;
  closed: boolean;
  interactions: Interactions;
  packInteractive: ReadonlySet<string>;
  onJump: (id: string) => void;
  onPrev: () => void;
  onNext: () => void;
}

export function WizardFooter({
  steps,
  index,
  total,
  advancing,
  closed,
  interactions,
  packInteractive,
  onJump,
  onPrev,
  onNext,
}: WizardFooterProps) {
  const onSummary = index >= total;
  return (
    <div className="wizard-bar">
      <Button variant="ghost" size="lg" onClick={onPrev} disabled={index <= 0}>
        ‹ Back
      </Button>
      <div className="wizard-progress">
        <StepDots
          steps={steps}
          index={index}
          interactions={interactions}
          packInteractive={packInteractive}
          onJump={onJump}
        />
        <p className="wizard-hint">{closed ? 'j/k or ←/→ move · ? shortcuts' : 'j/k or ←/→ move · 1–9 pick · ? shortcuts'}</p>
      </div>
      <Button
        variant="primary"
        size="lg"
        className={advancing ? 'advancing' : undefined}
        onClick={onNext}
        disabled={onSummary}
      >
        {index >= total - 1 ? 'Review' : 'Next ›'}
        {advancing && (
          <>
            <span className="focus-advance-fill" aria-hidden />
            <span className="focus-advance-text" aria-hidden>
              next in a moment
            </span>
          </>
        )}
      </Button>
    </div>
  );
}
