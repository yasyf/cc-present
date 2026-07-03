// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { renderInlineMarkdown, renderMarkdown } from './markdown';

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
