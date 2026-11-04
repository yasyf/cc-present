// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { holdCard, holdWhileTyping } from './touchHold';

describe('holdCard', () => {
  const fetchMock = vi.fn(async () => new Response('{}', { status: 200 }));

  beforeEach(() => {
    vi.useFakeTimers();
    fetchMock.mockClear();
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('reports at once, then every 10 seconds until the hold runs out, then stops', () => {
    holdCard('s', 'q', 25_000);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(20_000);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    vi.advanceTimersByTime(60_000);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it('extends a live hold without a second immediate report', () => {
    holdCard('s', 'r', 5_000);
    holdCard('s', 'r', 15_000);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(10_000);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    vi.advanceTimersByTime(60_000);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('holds the block around a typed field, wherever on the board it is', () => {
    document.body.innerHTML = '<div data-block-id="card-1"><textarea></textarea></div><button></button>';
    const stop = holdWhileTyping('s');
    document.querySelector('button')!.dispatchEvent(new Event('focusin', { bubbles: true }));
    expect(fetchMock).not.toHaveBeenCalled();

    document.querySelector('textarea')!.dispatchEvent(new Event('input', { bubbles: true }));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(JSON.parse(String(init.body))).toEqual({ subject: 's', blockId: 'card-1' });
    stop();
    document.body.innerHTML = '';
  });
});
