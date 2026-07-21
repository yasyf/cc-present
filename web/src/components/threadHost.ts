import { createContext, useContext } from 'react';

// ThreadHost names where an approval/choice hangs its feedback conversation:
// 'inline' renders the composer and thread beneath the block (the default, and
// what the single-block iOS embed keeps); 'rail' collapses it to a CommentChip and
// routes the compose affordance into the margin rail.
export type ThreadHost = 'inline' | 'rail';

// ThreadHostContext defaults to 'inline' so a block rendered without a provider —
// SingleBlockView's WKWebView embed — behaves exactly as it did before the rail.
export const ThreadHostContext = createContext<ThreadHost>('inline');

export function useThreadHost(): ThreadHost {
  return useContext(ThreadHostContext);
}
