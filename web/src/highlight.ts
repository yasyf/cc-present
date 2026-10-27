// The Shiki singleton, built from the fine-grained core plus the pure-JS regex
// engine (no WASM) and a curated language set, so the highlighter chunk stays
// small and lazy. The Code block imports this module dynamically, so first paint
// is a plain <pre> and the highlighted HTML swaps in once the chunk resolves.

import { createHighlighterCore } from 'shiki/core';
import { createJavaScriptRegexEngine } from 'shiki/engine/javascript';
import type { HighlighterCore, LanguageInput, ThemedToken } from 'shiki/core';
import { sanitizeForSgr } from './ansi';
import githubDark from '@shikijs/themes/github-dark';
import githubLight from '@shikijs/themes/github-light';

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

const GRAMMARS: Record<CodeLang, LanguageInput> = {
  typescript: () => import('@shikijs/langs/typescript'),
  tsx: () => import('@shikijs/langs/tsx'),
  javascript: () => import('@shikijs/langs/javascript'),
  go: () => import('@shikijs/langs/go'),
  python: () => import('@shikijs/langs/python'),
  rust: () => import('@shikijs/langs/rust'),
  bash: () => import('@shikijs/langs/bash'),
  json: () => import('@shikijs/langs/json'),
  yaml: () => import('@shikijs/langs/yaml'),
  html: () => import('@shikijs/langs/html'),
  css: () => import('@shikijs/langs/css'),
  sql: () => import('@shikijs/langs/sql'),
  swift: () => import('@shikijs/langs/swift'),
  toml: () => import('@shikijs/langs/toml'),
  diff: () => import('@shikijs/langs/diff'),
  markdown: () => import('@shikijs/langs/markdown'),
};

let highlighterPromise: Promise<HighlighterCore> | null = null;
const grammarLoads = new Map<CodeLang, Promise<void>>();

function getHighlighter(langs: CodeLang[]): Promise<HighlighterCore> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighterCore({
      engine: createJavaScriptRegexEngine({ forgiving: true }),
      themes: [githubLight, githubDark],
    });
  }
  return highlighterPromise.then(async (hl) => {
    for (const lang of langs) {
      if (!grammarLoads.has(lang)) grammarLoads.set(lang, hl.loadLanguage(GRAMMARS[lang]));
    }
    await Promise.all(langs.map((lang) => grammarLoads.get(lang)));
    return hl;
  });
}

function grammarsFor(code: string, lang: CodeLang): CodeLang[] {
  if (lang !== 'markdown') return [lang];
  const fenced = [...code.matchAll(/^\s*(?:`{3,}|~{3,})\s*([\w+#-]+)/gm)].flatMap((m) => resolveLang(m[1]!) ?? []);
  return [lang, ...fenced];
}

export function highlight(code: string, lang: CodeLang): Promise<string> {
  return getHighlighter(grammarsFor(code, lang)).then((hl) =>
    hl.codeToHtml(code, { lang, themes: { light: 'github-light', dark: 'github-dark' } }),
  );
}

// highlightAnsi renders a terminal stream's SGR colors to dual-theme HTML. Shiki
// short-circuits lang 'ansi' to its ANSI tokenizer before any grammar lookup, so it
// stays out of CODE_LANGS/resolveLang — a special language, not a grammar a code
// block may claim.
export function highlightAnsi(output: string): Promise<string> {
  return getHighlighter([]).then((hl) =>
    hl.codeToHtml(sanitizeForSgr(output), {
      lang: 'ansi',
      themes: { light: 'github-light', dark: 'github-dark' },
    }),
  );
}

// tokenizeLines returns dual-theme tokens per line (aligned to input lines) for
// the Diff and Draft blocks; each token's htmlStyle carries the light color +
// --shiki-dark var, which blocks.css swaps in dark mode.
export function tokenizeLines(code: string, lang: CodeLang): Promise<ThemedToken[][]> {
  return getHighlighter(grammarsFor(code, lang)).then(
    (hl) =>
      hl.codeToTokens(code, { lang, themes: { light: 'github-light', dark: 'github-dark' } })
        .tokens,
  );
}
