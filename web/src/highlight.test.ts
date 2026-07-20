import { describe, expect, it } from 'vitest';
import { highlightAnsi, langFromPath } from './highlight';

const ESC = String.fromCharCode(27);

describe('langFromPath', () => {
  const cases: [string, string | null][] = [
    ['main.go', 'go'],
    ['src/app.ts', 'typescript'],
    ['x.mts', null],
    ['component.tsx', 'tsx'],
    ['widget.jsx', 'tsx'],
    ['index.js', 'javascript'],
    ['greet.py', 'python'],
    ['lib.rs', 'rust'],
    ['data.json', 'json'],
    ['conf.yaml', 'yaml'],
    ['conf.yml', 'yaml'],
    ['style.css', 'css'],
    ['page.html', 'html'],
    ['README.md', 'markdown'],
    ['run.sh', 'bash'],
    ['query.sql', 'sql'],
    ['Model.swift', 'swift'],
    ['Config.toml', 'toml'],
    // extensions outside the curated allowlist resolve to null
    ['archive.tar.gz', null],
    // no extension / dotfiles / device paths
    ['Makefile', null],
    ['.gitignore', null],
    ['/dev/null', null],
  ];

  it.each(cases)('resolves %s to %s', (path, want) => {
    expect(langFromPath(path)).toBe(want);
  });

  it('is case-insensitive on the extension', () => {
    expect(langFromPath('a/b/C.PY')).toBe('python');
  });
});

const textOf = (html: string): string => html.replace(/<[^>]+>/g, '').replace(/\s/g, '');

describe('highlightAnsi', () => {
  it('tokenizes an SGR run to a dual-theme colored span and drops the escapes', async () => {
    const html = await highlightAnsi(`${ESC}[32mgreen${ESC}[0m plain`);
    expect(html).toContain('green');
    expect(html).toContain('plain');
    // 'ansi' is a special language shiki short-circuits — the 32m run reads github-light green.
    expect(html).toMatch(/color:#28a745/i);
    expect(html).toContain('--shiki-dark');
    expect(html).not.toContain(ESC);
  });

  it('pre-strips cursor/erase controls so a 2K/1A stream renders just its final text', async () => {
    const html = await highlightAnsi(`a${ESC}[2K${ESC}[1Ab`);
    expect(textOf(html)).toBe('ab');
    expect(html).not.toContain(ESC);
  });

  it('honors the parameterless reset ESC[m, re-coloring after it differently', async () => {
    const html = await highlightAnsi(`${ESC}[31mred${ESC}[mplain`);
    const color = (label: string) =>
      html.match(new RegExp(`color:(#[0-9a-f]+)[^>]*>${label}<`, 'i'))?.[1];
    expect(textOf(html)).toBe('redplain');
    // The reset drops red: "plain" reverts to the default fg, a different color than "red".
    expect(color('red')).not.toBe(color('plain'));
  });
});
