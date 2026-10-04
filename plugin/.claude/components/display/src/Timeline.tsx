import { useRef } from 'react';
import { renderMarkdown } from './host/present';
import type { PackComponentProps } from './host/present';
import { Controls, usePlayer } from './player';

interface TimelineEvent {
  when: string;
  title: string;
  md?: string;
  tag?: string;
  tone?: 'default' | 'ok' | 'warn' | 'danger';
  url?: string;
}

interface TimelineBlock {
  title?: string;
  autoplay?: boolean;
  intervalMs?: number;
  events: TimelineEvent[];
}

export function Timeline({ block }: PackComponentProps) {
  const b = block as unknown as TimelineBlock;
  const root = useRef<HTMLElement>(null);
  const player = usePlayer(b.events.length, b.intervalMs ?? 1200, b.autoplay ?? false, root);
  return (
    <section className="ccpd-frame" ref={root}>
      {b.title && <header className="ccpd-title">{b.title}</header>}
      <ol className="ccpd-timeline">
        {b.events.slice(0, player.shown).map((e, i) => (
          <li key={i} className={`ccpd-event${i === player.shown - 1 && player.playing ? ' ccpd-active' : ''}`}>
            <span className={`ccpd-dot ccpd-tone-${e.tone ?? 'default'}`} aria-hidden />
            <div className="ccpd-when">{e.when}</div>
            <div className="ccpd-event-head">
              {e.url ? (
                <a href={e.url} target="_blank" rel="noreferrer" className="ccpd-event-title">
                  {e.title}
                </a>
              ) : (
                <span className="ccpd-event-title">{e.title}</span>
              )}
              {e.tag && <span className="ccpd-tag">{e.tag}</span>}
            </div>
            {e.md && <div className="prose ccpd-event-body" dangerouslySetInnerHTML={{ __html: renderMarkdown(e.md) }} />}
          </li>
        ))}
      </ol>
      {b.events.length > 1 && <Controls player={player} unit="Event" />}
    </section>
  );
}
