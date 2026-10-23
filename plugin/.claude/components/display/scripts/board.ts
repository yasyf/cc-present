import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

interface Problem {
  host: string;
  view: string;
  rule: string;
  text: string;
}

interface Reply {
  result?: { result?: { value?: unknown }; exceptionDetails?: unknown };
}

const root = new URL('..', import.meta.url).pathname;
const fixtures = ['examples/system.json', 'scripts/fixtures/tiny.json', 'scripts/fixtures/storage.json', 'scripts/fixtures/web.json', 'scripts/fixtures/dense.json'];
const sizes: [number, number][] = [
  [1440, 900],
  [1024, 768],
  [768, 1024],
  [390, 844],
];
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

const server = Bun.serve({
  port: 0,
  async fetch(req) {
    const file = Bun.file(join(root, decodeURIComponent(new URL(req.url).pathname)));
    return (await file.exists()) ? new Response(file) : new Response('not found', { status: 404 });
  },
});

const chrome = process.env.CHROME ?? (process.platform === 'darwin' ? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' : 'google-chrome');
const profile = mkdtempSync(join(tmpdir(), 'smap-'));
const browser = Bun.spawn(
  [chrome, '--headless=new', '--no-sandbox', '--disable-gpu', '--hide-scrollbars', '--remote-allow-origins=*', `--user-data-dir=${profile}`, '--remote-debugging-port=0', 'about:blank'],
  { stdout: 'ignore', stderr: 'ignore' },
);

let target: { webSocketDebuggerUrl: string } | null = null;
for (let i = 0; i < 100 && !target; i++) {
  const port = await Bun.file(join(profile, 'DevToolsActivePort'))
    .text()
    .then((t) => t.split('\n')[0])
    .catch(() => null);
  if (port) {
    target = await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT' })
      .then((r) => r.json() as Promise<{ webSocketDebuggerUrl: string }>)
      .catch(() => null);
  }
  if (!target) await sleep(200);
}
if (!target) throw new Error(`Chrome did not open its debugging port at ${chrome}`);

const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise((r) => ws.addEventListener('open', r, { once: true }));
const pending = new Map<number, (m: Reply) => void>();
let next = 0;
ws.addEventListener('message', (ev) => {
  const m = JSON.parse(String(ev.data)) as Reply & { id: number };
  pending.get(m.id)?.(m);
});
const send = (method: string, params: Record<string, unknown> = {}) =>
  new Promise<Reply>((resolve, reject) => {
    const id = ++next;
    const timer = setTimeout(() => reject(new Error(`${method} got no answer from Chrome in 120 s`)), 120_000);
    pending.set(id, (m) => {
      clearTimeout(timer);
      resolve(m);
    });
    ws.send(JSON.stringify({ id, method, params }));
  });
const value = async <T>(expression: string): Promise<T> => {
  const m = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
  if (m.result?.exceptionDetails) throw new Error(`${expression.slice(0, 60)}: ${JSON.stringify(m.result.exceptionDetails)}`);
  return m.result?.result?.value as T;
};

const AUDIT = `(async () => {
  const out = [];
  for (const h of document.querySelectorAll("[data-smap]")) for (const p of await h.smap.audit({ all: true })) out.push({ host: h.id, ...p });
  return out;
})()`;

await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: 'light' }] });
await send('Emulation.setFocusEmulationEnabled', { enabled: true });
await send('Page.bringToFront');
const only = process.argv.slice(2);
const errors: string[] = [];
for (const [width, height] of sizes) {
  await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 2, mobile: width < 900 });
  for (const fixture of fixtures.filter((f) => !only.length || only.some((o) => f.includes(o)))) {
    const where = `${fixture.split('/').pop()?.replace('.json', '')} ${width}x${height}`;
    await send('Page.navigate', { url: `http://127.0.0.1:${server.port}/scripts/board.html?f=${fixture}` });
    let ready = false;
    for (let i = 0; i < 150 && !ready; i++) {
      await sleep(100);
      ready = await value<boolean>('window.boardReady === true').catch(() => false);
    }
    if (!ready) {
      errors.push(`${where}: the map never finished its first render`);
      continue;
    }
    const problems = await value<Problem[]>(AUDIT);
    const [scroll, client] = await value<[number, number]>('[document.documentElement.scrollWidth, document.documentElement.clientWidth]');
    const font = await value<boolean>(`document.fonts.check('600 20px "Shantell Sans"')`);
    console.log(`${problems.length ? 'fail' : 'ok'} ${where}: ${problems.length} problems, Shantell Sans ${font ? 'loaded' : 'missing'}`);
    for (const p of problems) errors.push(`${where} ${p.host} ${p.view}: ${p.text}`);
    if (scroll > client) errors.push(`${where}: the page scrolls sideways, ${scroll} > ${client}`);
  }
}

for (const e of errors) console.log(`::error file=plugin/.claude/components/display/src/system-map.js::${e}`);
ws.close();
browser.kill();
await server.stop(true);
process.exit(errors.length ? 1 : 0);
