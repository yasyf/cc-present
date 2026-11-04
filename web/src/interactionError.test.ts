import { describe, expect, it } from 'vitest';
import { interactionErrorText } from './interactionError';
import type { Interaction } from './events';

const pick: Interaction = { type: 'choice.selected', blockId: 'q', optionIds: ['a'] };

describe('interactionErrorText', () => {
  it('blames the connection only when the request never reached the daemon', () => {
    expect(interactionErrorText(pick, new TypeError('Failed to fetch'))).toBe(
      'Could not record your choice. Check your connection and try again.',
    );
  });

  it('shows the reason the daemon gave and never the request url', () => {
    const text = interactionErrorText(
      pick,
      new Error('POST /api/interactions?token=secret failed (400): choice "q" has no option "z"\n'),
    );
    expect(text).toBe('Could not record your choice: choice "q" has no option "z"');
  });

  it('never leaks the url when the daemon gave no reason', () => {
    expect(interactionErrorText(pick, new Error('POST /api/interactions?token=secret failed (500): '))).toBe(
      'Could not record your choice.',
    );
  });
});
