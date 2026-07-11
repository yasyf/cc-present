// Renders one pack block: the interaction frame (ring registration + value/submit
// wiring) around the pack's own component, an error boundary that contains a
// crashing component and retries when the agent redrafts the block, and a labeled
// placeholder for every not-yet-renderable state.

import { Component, useCallback, useRef } from 'react';
import type { ReactNode } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { PackBlock } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { useDecidable } from '../keyboard';
import { usePackComponent, usePackDef, useInteractivePackTypes } from '../packs/registry';
import type { PackDefState } from '../packs/registry';

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function focusFirstFocusable(root: HTMLElement | null): void {
  root?.querySelector<HTMLElement>(FOCUSABLE)?.focus();
}

function reasonFor(state: PackDefState): string {
  switch (state) {
    case 'unknown':
      return 'unknown pack';
    case 'failed':
      return 'pack failed to load';
    case 'ready':
      return 'component not exported';
    case 'loading':
      return 'loading pack…';
  }
}

function UnknownBlock({ block, reason }: { block: PackBlock; reason: string }) {
  return (
    <div className="pack-placeholder" role="note">
      <span className="pack-placeholder-type">{block.type}</span>
      <span className="pack-placeholder-id">{block.id}</span>
      <span className="pack-placeholder-reason">{reason}</span>
    </div>
  );
}

interface BoundaryProps {
  resetKey: unknown;
  fallback: ReactNode;
  children: ReactNode;
}

interface BoundaryState {
  failed: boolean;
  key: unknown;
}

// PackBoundary contains a crashing pack component. It resets when resetKey (the
// block object) changes identity, so an agent redraft re-attempts the render.
class PackBoundary extends Component<BoundaryProps, BoundaryState> {
  state: BoundaryState = { failed: false, key: this.props.resetKey };

  static getDerivedStateFromError(): Partial<BoundaryState> {
    return { failed: true };
  }

  static getDerivedStateFromProps(props: BoundaryProps, state: BoundaryState): Partial<BoundaryState> | null {
    if (props.resetKey !== state.key) return { failed: false, key: props.resetKey };
    return null;
  }

  render(): ReactNode {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}

export function PackBlockView({ block, interactions }: { block: PackBlock; interactions: Interactions }) {
  const { post, closed } = usePresent();
  const readOnly = useGroupReadOnly();
  const interactive = useInteractivePackTypes().has(block.type);
  const disabled = closed || readOnly || !interactive;

  const PackComponent = usePackComponent(block.type);
  const defState = usePackDef(block.type);
  const value = interactions.packs[block.id]?.payload;

  const submit = useCallback(
    (payload: unknown) => {
      void post({ type: 'pack.interaction', blockId: block.id, payload });
    },
    [post, block.id],
  );

  const frameRef = useRef<HTMLDivElement | null>(null);
  const { ref: decidableRef, cursor } = useDecidable(block.id, {
    kind: 'pack',
    disabled,
    engage: () => focusFirstFocusable(frameRef.current),
  });
  const setRef = useCallback(
    (el: HTMLDivElement | null) => {
      frameRef.current = el;
      decidableRef(el);
    },
    [decidableRef],
  );

  const inner = PackComponent ? (
    <PackBoundary resetKey={block} fallback={<UnknownBlock block={block} reason="crashed while rendering" />}>
      <PackComponent block={block} value={value} submit={submit} disabled={disabled} />
    </PackBoundary>
  ) : (
    <UnknownBlock block={block} reason={reasonFor(defState)} />
  );

  return (
    <div className="pack-block" ref={setRef} data-kbd-cursor={cursor || undefined}>
      {inner}
    </div>
  );
}
