import { useCallback, useEffect, useState } from 'react';

export type ThemeMode = 'system' | 'light' | 'dark';

// Duplicated by the FOUC script in index.html; keep both literals in sync.
const STORAGE_KEY = 'cc-present:theme';
const DARK_QUERY = '(prefers-color-scheme: dark)';

export function resolveMode(mode: ThemeMode, systemDark: boolean): 'light' | 'dark' {
  if (mode !== 'system') return mode;
  return systemDark ? 'dark' : 'light';
}

// applyUrlTheme pins an explicit ?theme=dark|light on the document before first
// render, so the single-block page a native WKWebView loads renders in the host's
// appearance instead of resolving color-scheme's light-dark() to the light side. An
// absent or invalid value leaves today's behavior — the FOUC script, then the OS
// preference — untouched.
export function applyUrlTheme(search: string): void {
  const theme = new URLSearchParams(search).get('theme');
  if (theme === 'light' || theme === 'dark') {
    document.documentElement.dataset.theme = theme;
  }
}

function readStored(): ThemeMode {
  const raw = localStorage.getItem(STORAGE_KEY);
  return raw === 'light' || raw === 'dark' || raw === 'system' ? raw : 'system';
}

export interface Theme {
  mode: ThemeMode;
  resolved: 'light' | 'dark';
  set: (mode: ThemeMode) => void;
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

  const set = useCallback((next: ThemeMode) => setMode(next), []);

  return { mode, resolved: resolveMode(mode, systemDark), set };
}

// currentResolvedTheme reads the palette the document actually applies: an explicit
// data-theme wins, else the OS preference. Mirrors mermaid.ts currentThemeKey.
function currentResolvedTheme(): 'light' | 'dark' {
  const explicit = document.documentElement.dataset.theme;
  if (explicit === 'light' || explicit === 'dark') return explicit;
  return window.matchMedia(DARK_QUERY).matches ? 'dark' : 'light';
}

// useResolvedTheme tracks the palette applied to the document — the data-theme
// attribute and the OS preference — so a consumer re-renders on any flip, including
// one driven by another component's ThemeToggle.
export function useResolvedTheme(): 'light' | 'dark' {
  const [resolved, setResolved] = useState(currentResolvedTheme);
  useEffect(() => {
    const update = () => setResolved(currentResolvedTheme());
    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    const mql = window.matchMedia(DARK_QUERY);
    mql.addEventListener('change', update);
    update();
    return () => {
      observer.disconnect();
      mql.removeEventListener('change', update);
    };
  }, []);
  return resolved;
}
