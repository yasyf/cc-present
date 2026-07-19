// The mermaid singleton, lazily imported like the Shiki highlighter. Its theme is
// resolved from the Blue Pencil alias tokens and re-initialized on a light/dark flip.

import type { Mermaid } from 'mermaid';

let loader: Promise<Mermaid> | null = null;
let themeKey: string | null = null;
let seq = 0;

// currentThemeKey names the palette a render draws against: an explicit data-theme
// wins, else the system preference.
function currentThemeKey(): string {
  const explicit = document.documentElement.dataset.theme;
  if (explicit) return explicit;
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

// themeVariables resolves the mermaid 'base' knobs to concrete colors — mermaid
// rejects var() strings, so the tokens are read off the document element.
function themeVariables(): Record<string, string> {
  const s = getComputedStyle(document.documentElement);
  const read = (name: string, fallback: string) => s.getPropertyValue(name).trim() || fallback;
  const surface = read('--surface', '#ffffff');
  const text = read('--text', '#1a1a1a');
  const border = read('--border', '#cccccc');
  const dim = read('--dim', '#666666');
  const accent = read('--accent', '#3b5bdb');
  const font = read('--font-prose', 'system-ui, sans-serif');
  return {
    background: surface,
    primaryColor: surface,
    primaryTextColor: text,
    primaryBorderColor: border,
    secondaryColor: surface,
    tertiaryColor: surface,
    mainBkg: surface,
    nodeBorder: border,
    lineColor: dim,
    textColor: text,
    titleColor: text,
    edgeLabelBackground: surface,
    clusterBkg: surface,
    clusterBorder: border,
    accent,
    fontFamily: font,
  };
}

function configure(mermaid: Mermaid): void {
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: 'strict',
    theme: 'base',
    themeVariables: themeVariables(),
    flowchart: { htmlLabels: false },
  });
  themeKey = currentThemeKey();
}

function load(): Promise<Mermaid> {
  if (!loader) {
    loader = import('mermaid').then((mod) => {
      configure(mod.default);
      return mod.default;
    });
  }
  return loader;
}

// renderDiagram renders a mermaid source to SVG markup, re-initializing the theme
// when the palette flipped since the last render. It throws on a parse or render
// error; DiagramView catches it and shows the source instead.
export async function renderDiagram(source: string): Promise<string> {
  const mermaid = await load();
  if (themeKey !== currentThemeKey()) configure(mermaid);
  const { svg } = await mermaid.render(`mmd-${(seq += 1)}`, source);
  return svg;
}
