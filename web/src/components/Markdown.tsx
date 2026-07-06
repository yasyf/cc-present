import type { Markdown as MarkdownBlock } from '../schema';
import { renderMarkdown } from '../markdown';
import { Clamped } from './Clamped';

export function Markdown({ block }: { block: MarkdownBlock }) {
  const html = renderMarkdown(block.md);
  if (block.struck) {
    return <div className="prose markdown-block struck" dangerouslySetInnerHTML={{ __html: html }} />;
  }
  return <Clamped html={html} lines={10} className="prose markdown-block" />;
}
