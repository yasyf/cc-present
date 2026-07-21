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
import { installHost } from './packs/host';
import { loadPacks } from './packs/load';
import './domain.css';

// Only the single-block route a native WKWebView loads carries ?theme; the board
// view has no theme param and is untouched.
if (new URLSearchParams(window.location.search).has('block')) {
  applyUrlTheme(window.location.search);
}

// Publish the host before any pack bundle imports, then kick the load off the
// first-paint path — placeholders swap live as bundles resolve.
installHost();
void loadPacks();

const root = document.getElementById('root');
if (!root) throw new Error('missing #root');

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
);
