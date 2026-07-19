// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import { LazyMotion, MotionConfig, domMax } from 'motion/react';
import { FocusSummary } from './FocusSummary';
import { focusSteps } from '../focus';
import { emptyState } from '../reduce';
import type { Interactions } from '../events';
import type { Block } from '../schema';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
window.matchMedia = ((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener() {},
  removeEventListener() {},
  addListener() {},
  removeListener() {},
  dispatchEvent: () => false,
})) as typeof window.matchMedia;

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

const choice = (id: string, prompt: string, labels: string[]): Block => ({
  id,
  type: 'choice',
  prompt,
  options: labels.map((label, i) => ({ id: `${id}o${i}`, label })),
});

function renderSummary(blocks: Block[], interactions: Interactions): void {
  const steps = focusSteps(blocks, new Set());
  act(() =>
    root.render(
      <LazyMotion features={domMax} strict>
        <MotionConfig reducedMotion="user">
          <FocusSummary steps={steps} interactions={interactions} packInteractive={new Set()} onJump={() => {}} />
        </MotionConfig>
      </LazyMotion>,
    ),
  );
}

const withChoices = (choices: Interactions['choices']): Interactions => ({ ...emptyState().interactions, choices });

describe('FocusSummary receipt answer', () => {
  it('surfaces the picked option label', () => {
    renderSummary([choice('c1', 'Which transport?', ['HTTP batch', 'gRPC'])], withChoices({ c1: { optionIds: ['c1o1'] } }));
    expect(container.querySelector('.focus-receipt-answer')?.textContent).toBe('gRPC');
  });

  it('surfaces a standalone write-in, quoted', () => {
    renderSummary(
      [choice('c1', 'Where should the log live?', ['Postgres', 'S3'])],
      withChoices({ c1: { optionIds: [], other: 'Cassandra per-site keyspaces' } }),
    );
    expect(container.querySelector('.focus-summary')?.textContent).toContain('Cassandra per-site keyspaces');
    expect(container.querySelector('.focus-receipt-answer')?.textContent).toBe('"Cassandra per-site keyspaces"');
  });

  it('joins a selected label with its coexisting write-in', () => {
    renderSummary(
      [choice('c1', 'Pick stores', ['Redis', 'S3'])],
      withChoices({ c1: { optionIds: ['c1o0'], other: 'Cassandra' } }),
    );
    expect(container.querySelector('.focus-receipt-answer')?.textContent).toBe('Redis, "Cassandra"');
  });
});
