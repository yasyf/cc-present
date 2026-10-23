// Markdown rendering for content blocks and inline table cells. marked turns the
// agent-authored markdown into HTML and DOMPurify sanitizes it, so script tags,
// event-handler attributes, and other injection vectors never reach the DOM.

import DOMPurify from 'dompurify';
import { marked } from 'marked';

const refTitle = /^ref:([0-9a-f]{64})$/;

function escapeAttr(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
}

// A link whose title is ref:<sha> is a reference the CLI resolved at push: it
// renders as a ref chip that opens its GitHub permalink in a new tab and carries
// the stored hover snippet's sha.
marked.use({
  renderer: {
    link(token) {
      const sha = token.title ? refTitle.exec(token.title)?.[1] : undefined;
      if (!sha) return false;
      const label = this.parser.parseInline(token.tokens);
      const target = token.href
        ? ` href="${escapeAttr(token.href)}" target="_blank" rel="noopener noreferrer"`
        : ' role="button" tabindex="0"';
      return `<a class="ref"${target} data-ref="${sha}">${label}</a>`;
    },
  },
});

const sanitizeOptions = { ADD_ATTR: ['target'] };

export function renderMarkdown(md: string): string {
  return DOMPurify.sanitize(marked.parse(md, { async: false }), sanitizeOptions);
}

export function renderInlineMarkdown(md: string): string {
  return DOMPurify.sanitize(marked.parseInline(md, { async: false }), sanitizeOptions);
}

export function markdownText(md: string): string {
  const el = document.createElement('div');
  el.innerHTML = renderMarkdown(md);
  return el.textContent!.replace(/\s+/g, ' ').trim();
}
