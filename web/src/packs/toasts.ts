// The pack toast bridge: ui.toast reaches the shell's ToastStack via a module sink.

import { useEffect } from 'react';

// PackToast is the two-kind subset a pack may raise, assignable to StreamToast.
export interface PackToast {
  kind: 'info' | 'error';
  text: string;
}

type ToastSink = (toast: PackToast) => void;

let sink: ToastSink | null = null;

// setPackToastSink registers (or clears, with null) the shell's toast destination.
export function setPackToastSink(fn: ToastSink | null): void {
  sink = fn;
}

// packToast is ui.toast: it forwards to the sink, or throws when none is set.
export function packToast(toast: PackToast): void {
  if (!sink) throw new Error('pack toast raised before the shell mounted');
  sink(toast);
}

// usePackToastSink registers `notify` during render — beating descendant mount
// effects so a pack that toasts on mount reaches a live stack — and clears on unmount.
export function usePackToastSink(notify: ToastSink): void {
  setPackToastSink(notify);
  useEffect(() => () => setPackToastSink(null), []);
}
