// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import type { Root } from 'react-dom/client';
import type { Record as RecordBlock } from '../schema';
import { RecordView } from './RecordView';

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

function renderRecord(block: RecordBlock): void {
  act(() => root.render(<RecordView block={block} />));
}

const record = (links?: RecordBlock['links']): RecordBlock => ({
  id: 'record-1',
  type: 'record',
  title: 'Release',
  chips: [
    { label: 'Stable' },
    { label: 'Preview', tone: 'demo' },
    { label: 'Blocked', tone: 'flag' },
  ],
  facts: [
    { label: 'Status', value: 'Ready', tone: 'good' },
    { label: 'Risk', value: 'Medium', tone: 'warn' },
    { label: 'Failures', value: '2', tone: 'bad' },
    { label: 'Owner', value: 'Platform' },
  ],
  links,
});

describe('RecordView', () => {
  it('renders chips and the toned fact grid', () => {
    renderRecord(record());

    const chips = [...container.querySelectorAll('.chips .chip')];
    expect(chips.map((chip) => chip.textContent)).toEqual(['Stable', 'Preview', 'Blocked']);
    expect(chips[0]?.classList.contains('chip-default')).toBe(true);
    expect(chips[1]?.classList.contains('chip-demo')).toBe(true);
    expect(chips[2]?.classList.contains('chip-flag')).toBe(true);

    const facts = [...container.querySelectorAll('.record-facts .fact')];
    expect(facts.map((fact) => fact.className)).toEqual([
      'fact fact-good',
      'fact fact-warn',
      'fact fact-bad',
      'fact fact-default',
    ]);
    expect(facts.map((fact) => fact.querySelector('.fact-label')?.textContent)).toEqual([
      'Status',
      'Risk',
      'Failures',
      'Owner',
    ]);
    expect(facts.map((fact) => fact.querySelector('.fact-value')?.textContent)).toEqual([
      'Ready',
      'Medium',
      '2',
      'Platform',
    ]);
  });

  it('renders links as external anchors', () => {
    renderRecord(
      record([
        { label: 'Documentation', url: 'https://example.com/docs' },
        { label: 'Dashboard', url: 'https://example.com/dashboard' },
      ]),
    );

    const links = [...container.querySelectorAll<HTMLAnchorElement>('.record-links a')];
    expect(links.map((link) => link.textContent)).toEqual(['Documentation', 'Dashboard']);
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      'https://example.com/docs',
      'https://example.com/dashboard',
    ]);
    for (const link of links) {
      expect(link.getAttribute('target')).toBe('_blank');
      expect(link.getAttribute('rel')).toBe('noopener noreferrer');
    }
  });

  it('omits the links row when links are absent or empty', () => {
    renderRecord(record());
    expect(container.querySelector('.record-links')).toBeNull();

    renderRecord(record([]));
    expect(container.querySelector('.record-links')).toBeNull();
  });
});
