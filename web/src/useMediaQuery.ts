import { useEffect, useState } from 'react';

// useMediaQuery subscribes to a CSS media query, mirroring useTheme's subscription,
// so a component re-renders whenever the match state flips (e.g. a viewport crossing
// a breakpoint). Returns the current match.
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches);
  useEffect(() => {
    const mql = window.matchMedia(query);
    const onChange = (e: MediaQueryListEvent) => setMatches(e.matches);
    setMatches(mql.matches);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, [query]);
  return matches;
}
