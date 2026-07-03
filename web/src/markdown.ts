// Markdown rendering for content blocks and inline table cells. marked turns the
// agent-authored markdown into HTML and DOMPurify sanitizes it, so script tags,
// event-handler attributes, and other injection vectors never reach the DOM.

import DOMPurify from 'dompurify';
import { marked } from 'marked';

export function renderMarkdown(md: string): string {
  return DOMPurify.sanitize(marked.parse(md, { async: false }));
}

export function renderInlineMarkdown(md: string): string {
  return DOMPurify.sanitize(marked.parseInline(md, { async: false }));
}
