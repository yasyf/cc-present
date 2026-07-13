import { describe, expect, it } from 'vitest';
import { langFromPath } from './highlight';

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
    // extensions outside the curated allowlist resolve to null
    ['Model.swift', null],
    ['Config.toml', null],
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
