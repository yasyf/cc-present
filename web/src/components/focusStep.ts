import { createContext } from 'react';
import type { OptionVisual } from '../schema';

// FocusStepContextValue names the decidable whose prompt FocusCard hoisted into
// the step <h2>, so its inline copy is suppressed; null when the card title heads.
export interface FocusStepContextValue {
  headlineId: string | null;
}

// FocusStepContext is provided by FocusCard around a step body; a null value is
// board mode, where card heads and prompts render as authored.
export const FocusStepContext = createContext<FocusStepContextValue | null>(null);

// FocusStageValue lets a choice publish the visual its active option carries so the
// card mounts it in the shared visual stage above the controls.
export interface FocusStageValue {
  setVisual: (visual: OptionVisual | null) => void;
}

// FocusStageContext is provided by FocusCard in focus mode; null in board mode,
// where an option's visual renders in its own detail drill-down instead of a stage.
export const FocusStageContext = createContext<FocusStageValue | null>(null);
