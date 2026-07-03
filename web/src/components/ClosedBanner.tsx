import { renderMarkdown } from '../markdown';

export function ClosedBanner({ summary }: { summary?: string }) {
  return (
    <div className="closed-banner">
      <span className="closed-badge">Closed</span>
      {summary && (
        <div className="closed-summary prose" dangerouslySetInnerHTML={{ __html: renderMarkdown(summary) }} />
      )}
    </div>
  );
}
