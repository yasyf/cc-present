import { describe, expect, it } from 'vitest';
import type { Table } from './schema';
import { filterChoices, visibleRows } from './tableRows';

const table: Table = {
  id: 't',
  type: 'table',
  columns: [
    { key: 'team', label: 'Team', filter: true },
    { key: 'behind', label: 'Behind', sort: true },
  ],
  rows: [
    { team: 'SoFi', behind: '10' },
    { team: 'AIG', behind: '9' },
    { team: 'AIG', behind: '1,200' },
  ],
};

describe('visibleRows', () => {
  it('keeps every row in order with no query', () => {
    expect(visibleRows(table, { text: '', chips: {} })).toEqual(table.rows);
  });

  it('filters by a chip and by text across columns', () => {
    expect(visibleRows(table, { text: '', chips: { team: 'AIG' } })).toHaveLength(2);
    expect(visibleRows(table, { text: 'sofi', chips: {} })).toEqual([table.rows[0]]);
  });

  it('sorts numbers as numbers, both ways', () => {
    const behind = (descending: boolean) =>
      visibleRows(table, { text: '', chips: {}, sort: { key: 'behind', descending } }).map((r) => r.behind);
    expect(behind(false)).toEqual(['9', '10', '1,200']);
    expect(behind(true)).toEqual(['1,200', '10', '9']);
  });
});

describe('filterChoices', () => {
  it('lists each value once', () => {
    expect(filterChoices(table, table.columns[0]!)).toEqual(['SoFi', 'AIG']);
  });
});
