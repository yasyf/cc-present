// The window.CcPresent host surface (contract hostApi 1): packs share the host's
// single React instance via aliased shims. installHost publishes it before imports.

import * as React from 'react';
import * as jsxRuntime from 'react/jsx-runtime';
import { createPortal } from 'react-dom';
import { Clamped } from '../components/Clamped';
import { renderInlineMarkdown, renderMarkdown } from '../markdown';
import { tokens } from './tokens';
import type { ThemeTokens } from './tokens';
import { packToast } from './toasts';
import type { PackToast } from './toasts';
import { usePackState } from './state';

export interface CcPresentHost {
  hostApi: 1;
  React: typeof React;
  jsxRuntime: typeof jsxRuntime;
  reactDom: { createPortal: typeof createPortal };
  ui: {
    Clamped: typeof Clamped;
    renderMarkdown: typeof renderMarkdown;
    renderInlineMarkdown: typeof renderInlineMarkdown;
    tokens: ThemeTokens;
    toast: (toast: PackToast) => void;
    usePackState: typeof usePackState;
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
    ui: { Clamped, renderMarkdown, renderInlineMarkdown, tokens, toast: packToast, usePackState },
  };
}
