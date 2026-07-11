import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClientProvider } from '@tanstack/react-query';
import '@cc-interact/react/base.css';
import { queryClient } from './api';
import { App } from './app';
import { installHost } from './packs/host';
import { loadPacks } from './packs/load';
import './domain.css';

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
