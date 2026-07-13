import { afterEach, describe, expect, it } from 'vitest';
import { packToast, setPackToastSink } from './toasts';
import type { PackToast } from './toasts';

afterEach(() => setPackToastSink(null));

describe('pack toasts', () => {
  it('throws before a sink is registered (fail loud)', () => {
    setPackToastSink(null);
    expect(() => packToast({ kind: 'info', text: 'x' })).toThrow(/before the shell mounted/);
  });

  it('delivers each toast to the registered sink in order', () => {
    const seen: PackToast[] = [];
    setPackToastSink((t) => seen.push(t));
    packToast({ kind: 'info', text: 'saved' });
    packToast({ kind: 'error', text: 'boom' });
    expect(seen).toEqual([
      { kind: 'info', text: 'saved' },
      { kind: 'error', text: 'boom' },
    ]);
  });

  it('throws again once the sink is cleared', () => {
    const seen: PackToast[] = [];
    setPackToastSink((t) => seen.push(t));
    setPackToastSink(null);
    expect(() => packToast({ kind: 'info', text: 'x' })).toThrow();
    expect(seen).toEqual([]);
  });
});
