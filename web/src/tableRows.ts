import type { Column, Table } from './schema';

export interface TableQuery {
  text: string;
  chips: { [key: string]: string };
  sort?: { key: string; descending: boolean };
}

const NUMBER = /^-?[\d,.]+%?$/;

function compareCells(a: string, b: string): number {
  if (NUMBER.test(a) && NUMBER.test(b)) return Number.parseFloat(a.replaceAll(',', '')) - Number.parseFloat(b.replaceAll(',', ''));
  return a.localeCompare(b);
}

export function filterChoices(table: Table, col: Column): string[] {
  return [...new Set(table.rows.map((row) => row[col.key] ?? '').filter(Boolean))];
}

export function visibleRows(table: Table, query: TableQuery): Table['rows'] {
  const text = query.text.trim().toLowerCase();
  const matched = table.rows.filter(
    (row) =>
      Object.entries(query.chips).every(([key, value]) => row[key] === value) &&
      (!text || table.columns.some((col) => (row[col.key] ?? '').toLowerCase().includes(text))),
  );
  const { sort } = query;
  if (!sort) return matched;
  const direction = sort.descending ? -1 : 1;
  return [...matched].sort((a, b) => direction * compareCells(a[sort.key] ?? '', b[sort.key] ?? ''));
}
