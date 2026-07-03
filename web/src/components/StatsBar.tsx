import type { Stat } from '../schema';

export function StatsBar({ stats }: { stats: Stat[] }) {
  return (
    <div className="stats">
      {stats.map((stat, i) => (
        <span key={i} className="stat">
          <b>{stat.value}</b>
          {stat.label}
        </span>
      ))}
    </div>
  );
}
