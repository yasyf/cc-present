// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { Button } from './Button';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

describe('Button', () => {
  it('uses the secondary medium defaults and defaults the native type', () => {
    act(() => root.render(<Button>Continue</Button>));
    const button = container.querySelector('button')!;
    expect(button.className).toBe('btn btn-secondary');
    expect(button.getAttribute('type')).toBe('button');
  });

  it('appends classes and spreads native button props', () => {
    const onClick = vi.fn();
    act(() =>
      root.render(
        <Button
          variant="primary"
          size="lg"
          className="advancing"
          type="submit"
          role="radio"
          aria-checked
          data-step="final"
          onClick={onClick}
        >
          Review
        </Button>,
      ),
    );
    const button = container.querySelector('button')!;
    act(() => button.click());
    expect(button.className).toBe('btn btn-primary btn-lg advancing');
    expect(button.getAttribute('type')).toBe('submit');
    expect(button.getAttribute('role')).toBe('radio');
    expect(button.getAttribute('aria-checked')).toBe('true');
    expect(button.dataset.step).toBe('final');
    expect(onClick).toHaveBeenCalledOnce();
  });
});
