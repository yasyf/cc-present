// The pack component registry: a module-level store the loader writes and the
// renderer reads through useSyncExternalStore, so a bundle that finishes loading
// after first paint swaps its placeholders live. Reducers never touch this — pack
// metadata stays out of PresentState (fixture byte-parity with Go).

import { useSyncExternalStore } from 'react';
import type { ComponentType } from 'react';
import type { PackBlock } from '../schema';
import type { PackInfo } from './manifest';

// PackBlockContext decomposes the block's lifecycle for a pack component: the
// board is closed, the block's round is over, and the current round number.
export interface PackBlockContext {
  closed: boolean;
  roundOver: boolean;
  round: number;
}

// PackComponentProps is the contract every pack leaf component is called with.
export interface PackComponentProps {
  block: PackBlock;
  value: unknown;
  submit: (payload: unknown) => void;
  disabled: boolean;
  context: PackBlockContext;
}

export type PackComponent = ComponentType<PackComponentProps>;

// PackDefState is how far the pack owning a dotted type has resolved. `ready`
// with no matching export is the renderer's "component not exported" case.
export type PackDefState = 'loading' | 'unknown' | 'failed' | 'ready';

interface PackEntry {
  status: 'loading' | 'ready' | 'failed';
  interactiveTypes: string[];
  components: Map<string, PackComponent>;
}

let manifestLoaded = false;
const entries = new Map<string, PackEntry>();
const listeners = new Set<() => void>();
let interactiveTypes: ReadonlySet<string> = new Set();

function packNameOf(fullType: string): string {
  return fullType.slice(0, fullType.indexOf('.'));
}

function bareNameOf(fullType: string): string {
  return fullType.slice(fullType.indexOf('.') + 1);
}

function emit(): void {
  const next = new Set<string>();
  for (const entry of entries.values()) for (const t of entry.interactiveTypes) next.add(t);
  interactiveTypes = next;
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

// registerPack records a pack's declared types. Called first with no components
// (status loading) at manifest time, then again with the imported components
// (status ready) once the bundle resolves.
export function registerPack(def: PackInfo, components?: Record<string, PackComponent>): void {
  entries.set(def.name, {
    status: components ? 'ready' : 'loading',
    interactiveTypes: def.blocks.filter((b) => b.interactive).map((b) => b.type),
    components: new Map(components ? Object.entries(components) : []),
  });
  emit();
}

// markFailed flips a pack to failed and drops its types from the ring; its blocks
// render the "pack failed to load" placeholder.
export function markFailed(name: string): void {
  const entry = entries.get(name);
  if (!entry) return;
  entry.status = 'failed';
  entry.interactiveTypes = [];
  entry.components.clear();
  emit();
}

// markPacksLoaded records that GET /api/packs has resolved, so a dotted type with
// no registered pack resolves to `unknown` instead of a perpetual `loading`.
export function markPacksLoaded(): void {
  manifestLoaded = true;
  emit();
}

export function resetPacksForTest(): void {
  manifestLoaded = false;
  entries.clear();
  interactiveTypes = new Set();
  listeners.clear();
}

export function getPackComponent(fullType: string): PackComponent | undefined {
  return entries.get(packNameOf(fullType))?.components.get(bareNameOf(fullType));
}

export function getPackDefState(fullType: string): PackDefState {
  const entry = entries.get(packNameOf(fullType));
  if (!entry) return manifestLoaded ? 'unknown' : 'loading';
  return entry.status;
}

export function getInteractivePackTypes(): ReadonlySet<string> {
  return interactiveTypes;
}

export function usePackComponent(fullType: string): PackComponent | undefined {
  return useSyncExternalStore(subscribe, () => getPackComponent(fullType));
}

export function usePackDef(fullType: string): PackDefState {
  return useSyncExternalStore(subscribe, () => getPackDefState(fullType));
}

export function useInteractivePackTypes(): ReadonlySet<string> {
  return useSyncExternalStore(subscribe, getInteractivePackTypes);
}
