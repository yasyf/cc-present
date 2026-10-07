import { renderInlineMarkdown, usePackState } from './host/present';
import type { PackComponentProps } from './host/present';
import { callCounts, callsProblems, flattenCalls, struckBy, toggleStruck } from './logic';
import type { CallMark, CallRow } from './logic';

interface CallsBlock {
  title?: string;
  caption?: string;
  calls: CallRow[];
}

const markClass: Record<CallMark, string> = { '+': 'add', '-': 'del', '~': 'mod', '?': 'ask', ' ': 'ctx' };
const markGlyph: Record<CallMark, string> = { '+': '+', '-': '−', '~': '~', '?': '?', ' ': '' };

export function Calls({ block, value, submit, disabled }: PackComponentProps) {
  const b = block as unknown as CallsBlock;
  const struck = (value as { struck: string[] } | undefined)?.struck ?? [];
  const [open, setOpen] = usePackState<string | null>('open', null);
  const problems = callsProblems(b.calls);
  if (problems.length > 0) {
    return (
      <section className="ccpl-frame">
        <div className="ccpl-error" role="alert">
          {problems.map((p) => (
            <div key={p}>{p}</div>
          ))}
        </div>
      </section>
    );
  }
  const flat = flattenCalls(b.calls);
  const under = struckBy(flat, new Set(struck));
  const counts = callCounts(b.calls);
  const strike = (key: string) => submit({ struck: toggleStruck(struck, key) });

  return (
    <section className="ccpl-frame ccpl-calls">
      <header className="ccpl-head">
        <span className="ccpl-kind">calls</span>
        {b.title && <span className="ccpl-head-title">{b.title}</span>}
        <span className="ccpl-counts">
          <span className="ccpl-c-add">+{counts.added}</span> <span className="ccpl-c-del">−{counts.removed}</span>{' '}
          <span className="ccpl-c-mod">~{counts.changed}</span>
          {counts.proposed > 0 && (
            <>
              {' '}
              <span className="ccpl-c-ask">?{counts.proposed}</span>
            </>
          )}
          {' · '}
          {b.calls.length} {b.calls.length === 1 ? 'entrypoint' : 'entrypoints'}
        </span>
      </header>
      <div className="ccpl-tree" role="list">
        {flat.map((f) => {
          const isStruck = struck.includes(f.key);
          const inherited = under.has(f.key);
          const canStrike = f.row.mark !== ' ' && !inherited && !disabled;
          const isOpen = open === f.key;
          return (
            <div
              key={f.key}
              role="listitem"
              className={[
                'ccpl-row',
                `ccpl-m-${markClass[f.row.mark]}`,
                f.depth === 0 && f.root > 0 ? 'ccpl-root' : '',
                isStruck || inherited ? 'ccpl-struck' : '',
                isOpen ? 'ccpl-open' : '',
              ]
                .filter(Boolean)
                .join(' ')}
            >
              <div className="ccpl-line" onClick={() => setOpen(isOpen ? null : f.key)}>
                <span className="ccpl-mark">{markGlyph[f.row.mark]}</span>
                <span className="ccpl-glyph">{f.glyph}</span>
                <span className={f.row.new ? 'ccpl-call ccpl-new' : 'ccpl-call'}>{f.row.call}</span>
                {f.row.at && <span className="ccpl-at">{f.row.at}</span>}
                {canStrike && (
                  <button
                    type="button"
                    className="ccpl-strike"
                    aria-pressed={isStruck}
                    onClick={(e) => {
                      e.stopPropagation();
                      strike(f.key);
                    }}
                  >
                    {isStruck ? 'Restore' : 'Strike'}
                  </button>
                )}
              </div>
              {isOpen && f.row.note && (
                <div
                  className="ccpl-note"
                  style={{ paddingLeft: `calc(2.2em + ${f.glyph.length}ch)` }}
                  dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(f.row.note) }}
                />
              )}
            </div>
          );
        })}
      </div>
      {struck.length > 0 && (
        <div className="ccpl-foot">
          {struck.length} struck {struck.length === 1 ? 'row' : 'rows'}; a struck row takes its subtree with it.
        </div>
      )}
      {b.caption && <div className="ccpl-caption" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(b.caption) }} />}
    </section>
  );
}
