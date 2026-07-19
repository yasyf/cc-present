// @vitest-environment jsdom
import { afterEach, describe, expect, it } from 'vitest';
import { applyUrlTheme, resolveMode } from './theme';

describe('resolveMode', () => {
  it('follows the system preference when mode is system', () => {
    expect(resolveMode('system', true)).toBe('dark');
    expect(resolveMode('system', false)).toBe('light');
  });

  it('ignores the system preference for an explicit mode', () => {
    expect(resolveMode('light', true)).toBe('light');
    expect(resolveMode('dark', false)).toBe('dark');
  });
});

describe('applyUrlTheme', () => {
  afterEach(() => {
    delete document.documentElement.dataset.theme;
  });

  it('pins an explicit ?theme=dark on the document', () => {
    applyUrlTheme('?theme=dark');
    expect(document.documentElement.dataset.theme).toBe('dark');
  });

  it('pins an explicit ?theme=light on the document', () => {
    applyUrlTheme('?theme=light');
    expect(document.documentElement.dataset.theme).toBe('light');
  });

  it('ignores a garbage theme value, leaving the document untouched', () => {
    applyUrlTheme('?theme=purple');
    expect(document.documentElement.dataset.theme).toBeUndefined();
  });

  it('ignores an absent theme param', () => {
    applyUrlTheme('?block=b1');
    expect(document.documentElement.dataset.theme).toBeUndefined();
  });
});
