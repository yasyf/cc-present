import { useEffect, useState } from 'react';
import type { PackComponentProps } from './host/present';
import { type DesignDocLocation, locateDesignDoc } from './logic';

export function DesignDoc({ block }: PackComponentProps) {
  const slug = block.slug as string;
  const branch = block.branch as string | undefined;
  const title = (block.title as string | undefined) ?? 'Design doc';
  const [located, setLocated] = useState<DesignDocLocation | null>(null);

  useEffect(() => {
    let current = true;
    void locateDesignDoc(slug, branch).then((found) => {
      if (current) setLocated(found);
    });
    return () => {
      current = false;
    };
  }, [slug, branch]);

  return (
    <figure className="ccpd-frame ccpd-artifact ccpd-designdoc">
      <figcaption className="ccpd-designdoc-head">
        <span className="ccpd-title">{title}</span>
        {located?.src && (
          <a href={located.src} target="_blank" rel="noreferrer">
            Open full page
          </a>
        )}
      </figcaption>
      {located?.problem && <p className="ccpd-error">{located.problem}</p>}
      {located?.src && <iframe title={title} loading="lazy" src={located.src} />}
    </figure>
  );
}
