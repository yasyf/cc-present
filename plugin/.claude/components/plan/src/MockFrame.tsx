import { useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode, RefObject } from 'react';
import { mockDocument, pinSpots } from './logic';
import type { MockSpec, Rect } from './logic';

function randomHex(): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) => b.toString(16).padStart(2, '0')).join('');
}

export function useWidth(el: RefObject<HTMLElement | null>): number {
  const [width, setWidth] = useState(0);
  useEffect(() => {
    if (!el.current) return;
    const ro = new ResizeObserver((entries) => setWidth(entries[0]?.contentRect.width ?? 0));
    ro.observe(el.current);
    return () => ro.disconnect();
  }, [el]);
  return width;
}

interface Measured {
  height: number;
  rects: Record<string, Rect>;
}

export interface MockFrameProps {
  spec: MockSpec;
  pinRefs?: string[];
  noted?: ReadonlySet<string>;
  active?: string | null;
  onPin?: (ref: string) => void;
  popover?: ReactNode;
}

export function MockFrame({ spec, pinRefs = [], noted, active, onPin, popover }: MockFrameProps) {
  const outer = useRef<HTMLDivElement>(null);
  const frame = useRef<HTMLIFrameElement>(null);
  const width = useWidth(outer);
  const [measured, setMeasured] = useState<Measured>({ height: 120, rects: {} });
  const kind = spec.frame ?? 'none';
  const { doc, token } = useMemo(() => {
    const token = randomHex();
    return { doc: mockDocument({ html: spec.html, css: spec.css, frame: kind }, randomHex(), token), token };
  }, [spec.html, spec.css, kind]);

  useEffect(() => {
    const onMessage = (e: MessageEvent) => {
      const data = e.data as { ccPresentMock?: string; height?: number; rects?: Record<string, Rect> } | null;
      if (e.source !== frame.current?.contentWindow || data?.ccPresentMock !== token) return;
      if (typeof data.height !== 'number') return;
      setMeasured({ height: Math.min(8000, Math.max(20, data.height)), rects: data.rects ?? {} });
    };
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [token]);

  const w = spec.w ?? 480;
  const inset = kind === 'phone' ? 24 : 2;
  const scale = width > 0 ? Math.min(1, (width - inset) / w) : 1;
  const fullH = spec.h ? Math.min(measured.height, spec.h) : measured.height;
  const shownW = Math.floor(w * scale);
  const shownH = Math.ceil(fullH * scale);
  const spots = pinSpots(pinRefs, measured.rects, scale, shownW, shownH);
  const activeSpot = spots.find((s) => s.ref === active);
  const bar = kind === 'browser' ? spec.url : kind === 'desktop' || kind === 'terminal' ? spec.title : undefined;

  return (
    <div className={`ccpl-mock ccpl-mock-${kind}`} ref={outer}>
      <div className="ccpl-device" style={{ width: shownW + inset }}>
        {(kind === 'browser' || kind === 'desktop' || kind === 'terminal') && (
          <div className="ccpl-chrome">
            <span className="ccpl-dots">
              <i />
              <i />
              <i />
            </span>
            {bar && <span className={kind === 'browser' ? 'ccpl-url' : 'ccpl-bar-title'}>{bar}</span>}
          </div>
        )}
        <div className="ccpl-screen" style={{ width: shownW, height: shownH }}>
          <div className="ccpl-clip" style={{ width: shownW, height: shownH }}>
            <iframe
              ref={frame}
              title={spec.title ?? 'mock'}
              sandbox="allow-scripts"
              srcDoc={doc}
              style={{ width: w, height: fullH, transform: `scale(${scale})` }}
            />
          </div>
          {spots.map((s) => (
            <button
              key={s.ref}
              type="button"
              className={`ccpl-pin${s.ref === active ? ' ccpl-on' : ''}${noted?.has(s.ref) ? ' ccpl-noted' : ''}`}
              style={{ left: s.x, top: s.y }}
              aria-label={`Pin ${s.n}`}
              onClick={() => onPin?.(s.ref)}
            >
              {s.n}
            </button>
          ))}
          {activeSpot && popover && (
            <div className="ccpl-pop-anchor" style={{ left: Math.min(activeSpot.x, Math.max(0, shownW - 280)), top: activeSpot.y + 16 }}>
              {popover}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
