// The present context: one interaction poster shared by every interactive block,
// plus the closed flag that disables inputs once the presentation terminates.
// Lives in its own module so the block components and the app root can both
// import it without a cycle.

import { createContext, useContext } from 'react';
import type { Interaction } from './events';

export interface PresentApi {
  // Resolves true once the daemon accepts the interaction, false when the POST
  // fails; never rejects, so fire-and-forget call sites stay clean.
  post: (interaction: Interaction) => Promise<boolean>;
  closed: boolean;
  currentRound: number;
}

export const PresentContext = createContext<PresentApi | null>(null);

export function usePresent(): PresentApi {
  const value = useContext(PresentContext);
  if (!value) throw new Error('usePresent must be used within a PresentContext provider');
  return value;
}
