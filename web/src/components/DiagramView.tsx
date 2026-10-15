import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
import DOMPurify from 'dompurify';
import type { Diagram } from '../schema';
import { useResolvedTheme } from '../theme';
import { Lightbox } from './Lightbox';

const MIN_SCALE = 0.8;

interface DiagramSize {
  natural: number;
  fit: number;
}

function naturalWidth(svg: SVGSVGElement): number | null {
  const viewBox = svg.getAttribute('viewBox');
  try {
    const width = viewBox ? Number(viewBox.trim().split(/[\s,]+/)[2]) : svg.getBBox().width;
    return width > 0 ? width : null;
  } catch {
    return null;
  }
}

function fitWidth(natural: number, available: number): number {
  return Math.min(natural, Math.max(available, MIN_SCALE * natural));
}

// DiagramView renders a mermaid source to sanitized SVG, re-rendered on a theme flip,
// with the source under an error banner when rendering fails. Labels never shrink
// below MIN_SCALE: a wider diagram scrolls, and Expand shows it at natural size.
export function DiagramView({ block }: { block: Diagram }) {
  const [svg, setSvg] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const [size, setSize] = useState<DiagramSize | null>(null);
  const [expanded, setExpanded] = useState(false);
  const frame = useRef<HTMLDivElement>(null);
  const theme = useResolvedTheme();

  useEffect(() => {
    let alive = true;
    setSvg(null);
    setFailed(false);
    void (async () => {
      try {
        const { renderDiagram } = await import('../mermaid');
        const raw = await renderDiagram(block.source);
        if (!alive) return;
        setSvg(DOMPurify.sanitize(raw, { USE_PROFILES: { svg: true, svgFilters: true } }));
      } catch {
        if (alive) setFailed(true);
      }
    })();
    return () => {
      alive = false;
    };
  }, [block.source, theme]);

  useLayoutEffect(() => {
    const el = frame.current;
    if (!el) return;
    const natural = naturalWidth(el.querySelector('svg')!);
    if (natural === null) {
      setSize(null);
      return;
    }
    const measure = () => setSize({ natural, fit: fitWidth(natural, el.clientWidth) });
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, [svg]);

  const style = size
    ? ({ '--diagram-fit': `${size.fit}px`, '--diagram-natural': `${size.natural}px` } as CSSProperties)
    : undefined;

  return (
    <figure className="diagram-block" style={style}>
      {block.title ? <figcaption className="diagram-title">{block.title}</figcaption> : null}
      {svg ? (
        <>
          <div ref={frame} className="diagram-svg" dangerouslySetInnerHTML={{ __html: svg }} />
          <button
            type="button"
            className="link-btn diagram-expand"
            aria-haspopup="dialog"
            onClick={() => setExpanded(true)}
          >
            Expand
          </button>
          <Lightbox
            open={expanded}
            onClose={() => setExpanded(false)}
            label={block.title || 'Diagram'}
            caption={block.title}
          >
            {expanded ? <div className="diagram-expanded" dangerouslySetInnerHTML={{ __html: svg }} /> : null}
          </Lightbox>
        </>
      ) : failed ? (
        <>
          <p className="diagram-error" role="status">
            This diagram could not be rendered
          </p>
          <pre className="diagram-source">
            <code>{block.source}</code>
          </pre>
        </>
      ) : (
        <div className="diagram-skeleton" aria-hidden />
      )}
    </figure>
  );
}
