import { useCallback, useEffect, useState } from 'react';

export type ThemeMode = 'system' | 'light' | 'dark';

const STORAGE_KEY = 'cc-present:theme';
const DARK_QUERY = '(prefers-color-scheme: dark)';

export function nextMode(mode: ThemeMode): ThemeMode {
  return mode === 'system' ? 'light' : mode === 'light' ? 'dark' : 'system';
}

export function resolveMode(mode: ThemeMode, systemDark: boolean): 'light' | 'dark' {
  if (mode !== 'system') return mode;
  return systemDark ? 'dark' : 'light';
}

function readStored(): ThemeMode {
  const raw = localStorage.getItem(STORAGE_KEY);
  return raw === 'light' || raw === 'dark' || raw === 'system' ? raw : 'system';
}

export interface Theme {
  mode: ThemeMode;
  resolved: 'light' | 'dark';
  cycle: () => void;
}

export function useTheme(): Theme {
  const [mode, setMode] = useState<ThemeMode>(readStored);
  const [systemDark, setSystemDark] = useState(() => window.matchMedia(DARK_QUERY).matches);

  useEffect(() => {
    const mql = window.matchMedia(DARK_QUERY);
    const onChange = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    if (mode === 'system') {
      delete root.dataset.theme;
      localStorage.removeItem(STORAGE_KEY);
    } else {
      root.dataset.theme = mode;
      localStorage.setItem(STORAGE_KEY, mode);
    }
  }, [mode]);

  const cycle = useCallback(() => setMode(nextMode), []);

  return { mode, resolved: resolveMode(mode, systemDark), cycle };
}
