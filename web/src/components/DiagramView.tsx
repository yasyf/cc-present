import { useEffect, useState } from 'react';
import DOMPurify from 'dompurify';
import type { Diagram } from '../schema';

// DiagramView renders a mermaid source client-side: a skeleton while the lazily
// imported renderer resolves, the sanitized SVG on success, or an error banner over
// the raw source when parsing or rendering fails.
export function DiagramView({ block }: { block: Diagram }) {
  const [svg, setSvg] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

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
  }, [block.source]);

  return (
    <figure className="diagram-block">
      {block.title ? <figcaption className="diagram-title">{block.title}</figcaption> : null}
      {svg ? (
        <div className="diagram-svg" dangerouslySetInnerHTML={{ __html: svg }} />
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
