import { DiagramView, renderMarkdown } from './host/present';
import type { PackComponentProps } from './host/present';
import { splitMermaid } from './logic';

export function Page({ block }: PackComponentProps) {
  const title = block.title as string | undefined;
  return (
    <article className="ccpd-frame ccpd-page">
      {title && <header className="ccpd-title">{title}</header>}
      {splitMermaid(block.md as string).map((s, i) =>
        s.kind === 'md' ? (
          <div key={i} className="prose" dangerouslySetInnerHTML={{ __html: renderMarkdown(s.text) }} />
        ) : (
          <DiagramView key={i} source={s.source} />
        ),
      )}
    </article>
  );
}
