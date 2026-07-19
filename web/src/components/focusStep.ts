import { createContext } from 'react';

// FocusStepContextValue names the decidable whose prompt FocusCard hoisted into
// the step <h2>, so its inline copy is suppressed; null when the card title heads.
export interface FocusStepContextValue {
  headlineId: string | null;
}

// FocusStepContext is provided by FocusCard around a step body; a null value is
// board mode, where card heads and prompts render as authored.
export const FocusStepContext = createContext<FocusStepContextValue | null>(null);
