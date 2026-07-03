import type { Markdown as MarkdownBlock } from '../schema';
import { renderMarkdown } from '../markdown';

export function Markdown({ block }: { block: MarkdownBlock }) {
  return (
    <div
      className={`prose markdown-block${block.struck ? ' struck' : ''}`}
      dangerouslySetInnerHTML={{ __html: renderMarkdown(block.md) }}
    />
  );
}
