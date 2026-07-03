import type { Section as SectionBlock } from '../schema';
import { renderMarkdown } from '../markdown';

export function Section({ block }: { block: SectionBlock }) {
  return (
    <section className="doc-section">
      <h2 className="section-title">{block.title}</h2>
      {block.md && (
        <div className="prose section-md" dangerouslySetInnerHTML={{ __html: renderMarkdown(block.md) }} />
      )}
    </section>
  );
}
