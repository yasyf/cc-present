// The Shiki singleton, built from the fine-grained core plus the pure-JS regex
// engine (no WASM) and a curated language set, so the highlighter chunk stays
// small and lazy. The Code block imports this module dynamically, so first paint
// is a plain <pre> and the highlighted HTML swaps in once the chunk resolves.

import { createHighlighterCore } from 'shiki/core';
import { createJavaScriptRegexEngine } from 'shiki/engine/javascript';
import type { HighlighterCore, ThemedToken } from 'shiki/core';
import githubDark from '@shikijs/themes/github-dark';
import githubLight from '@shikijs/themes/github-light';
import bash from '@shikijs/langs/bash';
import css from '@shikijs/langs/css';
import diff from '@shikijs/langs/diff';
import go from '@shikijs/langs/go';
import html from '@shikijs/langs/html';
import javascript from '@shikijs/langs/javascript';
import json from '@shikijs/langs/json';
import markdown from '@shikijs/langs/markdown';
import python from '@shikijs/langs/python';
import rust from '@shikijs/langs/rust';
import sql from '@shikijs/langs/sql';
import swift from '@shikijs/langs/swift';
import toml from '@shikijs/langs/toml';
import tsx from '@shikijs/langs/tsx';
import typescript from '@shikijs/langs/typescript';
import yaml from '@shikijs/langs/yaml';

export const CODE_LANGS = [
  'typescript',
  'tsx',
  'javascript',
  'go',
  'python',
  'rust',
  'bash',
  'json',
  'yaml',
  'html',
  'css',
  'sql',
  'swift',
  'toml',
  'diff',
  'markdown',
] as const;

export type CodeLang = (typeof CODE_LANGS)[number];

const ALIASES: Record<string, CodeLang> = {
  ts: 'typescript',
  js: 'javascript',
  jsx: 'tsx',
  py: 'python',
  rs: 'rust',
  sh: 'bash',
  shell: 'bash',
  zsh: 'bash',
  yml: 'yaml',
  md: 'markdown',
};

// resolveLang maps a block's lang tag to a loaded grammar, or null when the
// language is outside the curated set (rendered as plain text).
export function resolveLang(lang: string): CodeLang | null {
  const normal = lang.trim().toLowerCase();
  if ((CODE_LANGS as readonly string[]).includes(normal)) return normal as CodeLang;
  return ALIASES[normal] ?? null;
}

// langFromPath resolves a diff's file path to a grammar via its extension, or
// null when the extension is outside the curated set.
export function langFromPath(path: string): CodeLang | null {
  const base = path.slice(path.lastIndexOf('/') + 1);
  const dot = base.lastIndexOf('.');
  if (dot <= 0) return null;
  return resolveLang(base.slice(dot + 1));
}

let highlighterPromise: Promise<HighlighterCore> | null = null;

export function getHighlighter(): Promise<HighlighterCore> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighterCore({
      engine: createJavaScriptRegexEngine({ forgiving: true }),
      themes: [githubLight, githubDark],
      langs: [
        typescript,
        tsx,
        javascript,
        go,
        python,
        rust,
        bash,
        json,
        yaml,
        html,
        css,
        sql,
        swift,
        toml,
        diff,
        markdown,
      ],
    });
  }
  return highlighterPromise;
}

export function highlight(code: string, lang: CodeLang): Promise<string> {
  return getHighlighter().then((hl) =>
    hl.codeToHtml(code, { lang, themes: { light: 'github-light', dark: 'github-dark' } }),
  );
}

// highlightAnsi renders a terminal stream's SGR colors to dual-theme HTML. Shiki
// short-circuits lang 'ansi' to its ANSI tokenizer before any grammar lookup, so it
// stays out of CODE_LANGS/resolveLang — a special language, not a grammar a code
// block may claim.
export function highlightAnsi(output: string): Promise<string> {
  return getHighlighter().then((hl) =>
    hl.codeToHtml(output, { lang: 'ansi', themes: { light: 'github-light', dark: 'github-dark' } }),
  );
}

// tokenizeLines returns dual-theme tokens per line (aligned to input lines) for
// the Diff block; each token's htmlStyle carries the light color + --shiki-dark var.
export function tokenizeLines(code: string, lang: CodeLang): Promise<ThemedToken[][]> {
  return getHighlighter().then(
    (hl) =>
      hl.codeToTokens(code, { lang, themes: { light: 'github-light', dark: 'github-dark' } })
        .tokens,
  );
}
