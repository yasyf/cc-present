import { useQuery } from '@tanstack/react-query';

import type { Live as LiveBlock } from '../schema';
import { useSubject } from '../api';
import { withToken } from '../token';

interface LiveItem {
  ref: string;
  title: string;
  state: string;
  says: string;
  cells?: Record<string, string>;
  actions?: string[];
}

interface LiveResult {
  items: LiveItem[];
  total: number;
}

async function fetchLive(subject: string, block: string): Promise<LiveResult> {
  const query = new URLSearchParams({ subject, block });
  const res = await fetch(withToken(`/api/live?${query.toString()}`));
  if (!res.ok) throw new Error((await res.text()).trim() || `live read failed (${res.status})`);
  return (await res.json()) as LiveResult;
}

export function LiveView({ block }: { block: LiveBlock }) {
  const { subject } = useSubject();
  const query = useQuery<LiveResult, Error>({
    queryKey: ['live', subject, block.id],
    queryFn: () => fetchLive(subject, block.id),
    staleTime: 10_000,
    retry: false,
  });
  if (query.error) return <p className="live-error">{query.error.message}</p>;
  if (!query.data) return <p className="live-loading">Reading…</p>;
  if (block.view === 'count') return <p className="live-count">{query.data.total}</p>;
  return (
    <ul className={`live-items live-${block.view}`}>
      {query.data.items.map((item) => (
        <li key={item.ref} data-state={item.state}>
          <b>{item.title}</b>
          <span>{item.says}</span>
        </li>
      ))}
    </ul>
  );
}
