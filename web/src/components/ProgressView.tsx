import type { Progress as ProgressBlock } from '../schema';

export function ProgressView({ block }: { block: ProgressBlock }) {
  const pct = Math.max(0, Math.min(100, (block.value / block.max) * 100));
  return (
    <div className={`progress-block progress-state-${block.state ?? 'active'}`}>
      <div className="progress-head">
        <span className="progress-label">{block.label}</span>
        <span className="progress-count">
          {block.value}/{block.max}
        </span>
      </div>
      <div className="progress-track">
        <span className="progress-fill" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
