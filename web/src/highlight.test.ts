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
});
