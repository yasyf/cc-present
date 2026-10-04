import { useEffect, useRef, useState } from 'react';
import type { RefObject } from 'react';

export interface Player {
  shown: number;
  total: number;
  playing: boolean;
  play: () => void;
  pause: () => void;
  step: (delta: number) => void;
  restart: () => void;
}

export function usePlayer(total: number, intervalMs: number, autoplay: boolean, root: RefObject<HTMLElement | null>): Player {
  const [shown, setShown] = useState(autoplay ? 1 : total);
  const [playing, setPlaying] = useState(false);
  const started = useRef(!autoplay);

  useEffect(() => {
    if (started.current || !root.current) return;
    const el = root.current;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting) && !started.current) {
          started.current = true;
          setPlaying(true);
          io.disconnect();
        }
      },
      { threshold: 0.4 },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [root]);

  useEffect(() => {
    if (!playing) return;
    if (shown >= total) {
      setPlaying(false);
      return;
    }
    const timer = setTimeout(() => setShown((n) => Math.min(total, n + 1)), intervalMs);
    return () => clearTimeout(timer);
  }, [playing, shown, total, intervalMs]);

  return {
    shown,
    total,
    playing,
    play: () => {
      started.current = true;
      if (shown >= total) setShown(1);
      setPlaying(true);
    },
    pause: () => setPlaying(false),
    step: (delta) => {
      started.current = true;
      setPlaying(false);
      setShown((n) => Math.max(1, Math.min(total, n + delta)));
    },
    restart: () => {
      started.current = true;
      setShown(1);
      setPlaying(true);
    },
  };
}

export function Controls({ player, unit }: { player: Player; unit: string }) {
  const { shown, total, playing } = player;
  return (
    <div className="ccpd-controls">
      <button type="button" className="ccpd-btn" onClick={player.restart} aria-label="Restart" title="Restart">
        ⏮
      </button>
      <button type="button" className="ccpd-btn" onClick={() => player.step(-1)} disabled={shown <= 1} aria-label="Previous" title="Previous">
        ◀
      </button>
      <button
        type="button"
        className="ccpd-btn ccpd-btn-primary"
        onClick={playing ? player.pause : player.play}
        aria-label={playing ? 'Pause' : 'Play'}
        title={playing ? 'Pause' : 'Play'}
      >
        {playing ? '❚❚' : '▶'}
      </button>
      <button type="button" className="ccpd-btn" onClick={() => player.step(1)} disabled={shown >= total} aria-label="Next" title="Next">
        ▶▶
      </button>
      <div className="ccpd-progress" aria-hidden>
        <div className="ccpd-progress-fill" style={{ width: `${(shown / total) * 100}%` }} />
      </div>
      <span className="ccpd-count">
        {unit} {shown} / {total}
      </span>
    </div>
  );
}
