// The present context: one interaction poster shared by every interactive block,
// plus the closed flag that disables inputs once the presentation terminates.
// Lives in its own module so the block components and the app root can both
// import it without a cycle.

import { createContext, useContext } from 'react';
import type { Interaction } from './events';

export interface PresentApi {
  post: (interaction: Interaction) => void;
  closed: boolean;
}

export const PresentContext = createContext<PresentApi | null>(null);

export function usePresent(): PresentApi {
  const value = useContext(PresentContext);
  if (!value) throw new Error('usePresent must be used within a PresentContext provider');
  return value;
}
