// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { markdownText, renderInlineMarkdown, renderMarkdown } from './markdown';

describe('markdown sanitization', () => {
  it('strips <script> tags', () => {
    const html = renderMarkdown('Hello there\n\n<script>alert(1)</script>');
    expect(html).not.toMatch(/<script/i);
    expect(html).toContain('Hello there');
  });

  it('strips onerror attributes from injected img', () => {
    const html = renderMarkdown('<img src=x onerror="alert(1)">');
    expect(html.toLowerCase()).not.toContain('onerror');
  });

  it('sanitizes inline markdown as well', () => {
    const html = renderInlineMarkdown('a <img src=x onerror=alert(1)> b');
    expect(html.toLowerCase()).not.toContain('onerror');
    expect(html).not.toMatch(/<script/i);
  });

  it('keeps benign inline formatting', () => {
    expect(renderInlineMarkdown('**bold** and `code`')).toContain('<strong>');
  });
});

describe('markdownText', () => {
  it('strips emphasis and links down to their text', () => {
    expect(markdownText('**Bold** and _soft_ with [a link](https://example.com)')).toBe('Bold and soft with a link');
  });

  it('keeps a code fence’s source without the fence', () => {
    expect(markdownText('Run this:\n\n```ts\nconst a = 1;\n```')).toBe('Run this: const a = 1;');
  });

  it('collapses whitespace across paragraphs and lists', () => {
    expect(markdownText('one\n\n\ntwo   three\n\n- a\n- b\n')).toBe('one two three a b');
  });
});
