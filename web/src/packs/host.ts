// The window.CcPresent host surface (contract hostApi 1). Packs build with
// react/react/jsx-runtime aliased to shims that re-export from this global, so a
// pack bundle shares the host's single React instance instead of bundling its
// own. installHost publishes it once, before any pack bundle imports.

import * as React from 'react';
import * as jsxRuntime from 'react/jsx-runtime';
import { createPortal } from 'react-dom';
import { Clamped } from '../components/Clamped';
import { renderInlineMarkdown, renderMarkdown } from '../markdown';

export interface CcPresentHost {
  hostApi: 1;
  React: typeof React;
  jsxRuntime: typeof jsxRuntime;
  reactDom: { createPortal: typeof createPortal };
  ui: {
    Clamped: typeof Clamped;
    renderMarkdown: typeof renderMarkdown;
    renderInlineMarkdown: typeof renderInlineMarkdown;
  };
}

declare global {
  interface Window {
    CcPresent?: CcPresentHost;
  }
}

// installHost publishes window.CcPresent. Idempotent: a second call (StrictMode
// double-invoke, HMR) leaves the first instance in place.
export function installHost(): void {
  if (window.CcPresent) return;
  window.CcPresent = {
    hostApi: 1,
    React,
    jsxRuntime,
    reactDom: { createPortal },
    ui: { Clamped, renderMarkdown, renderInlineMarkdown },
  };
}
