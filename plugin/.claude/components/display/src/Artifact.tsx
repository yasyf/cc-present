import { useEffect, useMemo, useRef, useState } from 'react';
import type { PackComponentProps } from './host/present';
import { artifactDocument } from './logic';

export function Artifact({ block }: PackComponentProps) {
  const kind = block.kind as 'html' | 'svg';
  const source = block.source as string;
  const fixed = block.height as number | undefined;
  const title = block.title as string | undefined;
  const frame = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState(fixed ?? 160);
  const doc = useMemo(() => artifactDocument(kind, source, block.id), [kind, source, block.id]);

  useEffect(() => {
    if (fixed !== undefined) return;
    const onMessage = (e: MessageEvent) => {
      const data = e.data as { ccPresentArtifact?: string; height?: number } | null;
      if (e.source !== frame.current?.contentWindow || data?.ccPresentArtifact !== block.id) return;
      if (typeof data.height === 'number') setHeight(Math.min(4000, Math.max(40, data.height)));
    };
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [fixed, block.id]);

  return (
    <figure className="ccpd-frame ccpd-artifact">
      {title && <figcaption className="ccpd-title">{title}</figcaption>}
      <iframe ref={frame} title={title ?? 'artifact'} sandbox="allow-scripts allow-popups" srcDoc={doc} style={{ height }} />
    </figure>
  );
}
