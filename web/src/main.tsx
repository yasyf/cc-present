import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClientProvider } from '@tanstack/react-query';
import '@cc-interact/react/base.css';
import '@fontsource/barlow-condensed/latin-500.css';
import '@fontsource/barlow-condensed/latin-700.css';
import '@fontsource/barlow/latin-400.css';
import '@fontsource/barlow/latin-600.css';
import '@fontsource/barlow/latin-700.css';
import '@fontsource-variable/source-serif-4';
import '@fontsource-variable/jetbrains-mono';
import { queryClient } from './api';
import { App } from './app';
import { applyUrlTheme } from './theme';
import { loadTheme } from './preferences';
import { installHost } from './packs/host';
import { loadPacks } from './packs/load';
import './domain.css';

// Only the single-block route a native WKWebView loads carries ?theme; the board
// view has no theme param and is untouched.
const isBlockRoute = new URLSearchParams(window.location.search).has('block');

// Publish the host before any pack bundle imports, then kick the load off the
// first-paint path — placeholders swap live as bundles resolve.
installHost();
void loadPacks();

const root = document.getElementById('root');
if (!root) throw new Error('missing #root');

let preferenceError: Error | null = null;
try {
  const theme = loadTheme();
  if (theme === 'system') {
    delete document.documentElement.dataset.theme;
  } else {
    document.documentElement.dataset.theme = theme;
  }
} catch (error) {
  preferenceError = error instanceof Error ? error : new Error(String(error));
}
if (isBlockRoute) applyUrlTheme(window.location.search);

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      {preferenceError ? (
        <div className="connect-error">
          <div className="connect-error-title">Preferences unavailable</div>
          <div className="connect-error-sub">{preferenceError.message}. Remove the stored value manually, then reload.</div>
        </div>
      ) : (
        <App />
      )}
    </QueryClientProvider>
  </StrictMode>,
);
