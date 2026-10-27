import { useRef, useState } from 'react';
import { useGroupReadOnly } from '@cc-interact/react';
import type { PRGroup, PRGroups as PRGroupsBlock, PRRow } from '../schema';
import type { Decision, Interactions } from '../events';
import { draftStore, useDrafts } from '../drafts';
import { usePresent } from '../present';
import { useDecidable } from '../keyboard';
import { renderInlineMarkdown, renderMarkdown } from '../markdown';
import {
  allRows,
  approveGroup,
  givenLine,
  prRowId,
  rowVerdict,
  staleLine,
  tally,
  toggleEntry,
  type VerdictEntry,
} from '../prgroups';
import { Button } from './Button';
import { PRView } from './PRView';

const ROW_NOTES = 'row-notes';

const FIELDS: readonly { key: 'change' | 'plan' | 'risk' | 'review' | 'blocker'; label: string }[] = [
  { key: 'change', label: 'What changes' },
  { key: 'plan', label: 'Against the plan' },
  { key: 'risk', label: 'Risk' },
  { key: 'review', label: 'Review and CI' },
  { key: 'blocker', label: 'Blocks landing' },
];

// PRGroups renders the pull requests still waiting on the human, grouped by what
// they change, one column at any width. Each row shows its five merge facts and an
// Approve or Hold verdict pinned to the row's head; a tap on the row expands its
// full text and the live PR in place. A verdict given at an older head asks for a
// new tap. A row whose verdict stands at its head leaves the groups for the
// collapsed answered list at the bottom.
export function PRGroups({ block, interactions }: { block: PRGroupsBlock; interactions: Interactions }) {
  const { post, closed, roundOver } = usePresent();
  const readOnly = useGroupReadOnly();
  const locked = closed || readOnly || (roundOver ?? false);
  const verdicts = interactions.triage[block.id] ?? {};
  const all = allRows(block);
  const total = tally(all, verdicts);
  const waiting = (row: PRRow) => rowVerdict(row, verdicts[prRowId(row.number)]).kind !== 'current';
  const rows = all.filter(waiting);
  const groups = block.groups.map((g) => ({ ...g, prs: g.prs.filter(waiting) })).filter((g) => g.prs.length > 0);
  const answered = all.filter((row) => !waiting(row));
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set());
  const [rowCursor, setRowCursor] = useState(0);

  function send(merge: Record<string, VerdictEntry>) {
    if (Object.keys(merge).length === 0) return Promise.resolve(true);
    return post({ type: 'triage.decided', blockId: block.id, verdicts: merge });
  }

  function toggleOpen(id: string) {
    const next = new Set(open);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setOpen(next);
  }

  const { ref, cursor } = useDecidable(block.id, {
    kind: 'triage',
    disabled: locked,
    verdict: (target) => {
      const row = rows[rowCursor];
      if (row) void send({ [prRowId(row.number)]: toggleEntry(row, verdicts[prRowId(row.number)], target) });
    },
    clear: () => {
      const row = rows[rowCursor];
      if (row && verdicts[prRowId(row.number)]) void send({ [prRowId(row.number)]: { verdict: 'cleared' } });
    },
    choose: (n) => {
      if (n >= 1 && n <= rows.length) setRowCursor(n - 1);
    },
    engage: () => {
      const row = rows[rowCursor];
      if (row) toggleOpen(prRowId(row.number));
    },
  });
  const cursorId = cursor && rows[rowCursor] ? prRowId(rows[rowCursor]!.number) : undefined;

  return (
    <div className="prg" ref={ref} data-kbd-cursor={cursor || undefined}>
      <div className="prg-head">
        {block.prompt && <p className="prg-prompt">{block.prompt}</p>}
        <p className="prg-tally" aria-live="polite">
          <span className="prg-tally-approved">{total.approved} approved</span>
          {total.held > 0 && <span className="prg-tally-held">{total.held} held</span>}
          {total.open > 0 && <span className="prg-tally-open">{total.open} waiting on you</span>}
        </p>
      </div>
      {groups.map((group) => (
        <Group
          key={group.id}
          block={block}
          group={group}
          verdicts={verdicts}
          locked={locked}
          open={open}
          cursorId={cursorId}
          onFocusRow={(id) => setRowCursor(rows.findIndex((r) => prRowId(r.number) === id))}
          onToggle={toggleOpen}
          send={send}
        />
      ))}
      {answered.length > 0 && <Answered block={block} rows={answered} verdicts={verdicts} locked={locked} send={send} />}
    </div>
  );
}

function Answered({
  block,
  rows,
  verdicts,
  locked,
  send,
}: {
  block: PRGroupsBlock;
  rows: PRRow[];
  verdicts: Record<string, Decision>;
  locked: boolean;
  send: (merge: Record<string, VerdictEntry>) => Promise<boolean>;
}) {
  const t = tally(rows, verdicts);
  const summary = [`${t.approved} approved, landing`, ...(t.held > 0 ? [`${t.held} held`] : [])].join(' · ');
  return (
    <details className="prg-answered">
      <summary className="prg-answered-summary">{summary}</summary>
      <ul className="prg-answered-rows">
        {rows.map((row) => {
          const id = prRowId(row.number);
          const v = rowVerdict(row, verdicts[id]);
          if (v.kind !== 'current') return null;
          const state = v.decision.verdict === 'approved' ? 'approved' : 'held';
          return (
            <li key={id} className={`prg-answered-row prg-${state}`} data-row={id}>
              <span className="prg-row-line">
                <span className="prg-num">#{row.number}</span>
                <span className={`prg-state prg-state-${state}`}>{STATE_LABEL[state]}</span>
              </span>
              <span className="prg-title">{row.title}</span>
              <p className="prg-given">{givenLine(v, row)}</p>
              {v.decision.note && <p className="prg-note">{v.decision.note}</p>}
              {!locked && <RowNote block={block} row={row} decision={v.decision} send={send} />}
            </li>
          );
        })}
      </ul>
    </details>
  );
}

interface GroupProps {
  block: PRGroupsBlock;
  group: PRGroup;
  verdicts: Record<string, Decision>;
  locked: boolean;
  open: ReadonlySet<string>;
  cursorId: string | undefined;
  onFocusRow: (id: string) => void;
  onToggle: (id: string) => void;
  send: (merge: Record<string, VerdictEntry>) => Promise<boolean>;
}

function Group({ block, group, verdicts, locked, open, cursorId, onFocusRow, onToggle, send }: GroupProps) {
  return (
    <section className="prg-group" aria-labelledby={`${block.id}-${group.id}`}>
      <header className="prg-group-head">
        <h3 className="prg-group-title" id={`${block.id}-${group.id}`}>
          {group.title}
        </h3>
        <p className="prg-group-summary" dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(group.summary) }} />
        <div className="prg-group-bar">
          <span className="prg-group-count">{group.prs.length} waiting on you</span>
          {!locked && (
            <Button variant="secondary" className="prg-approve-all" onClick={() => void send(approveGroup(group, verdicts))}>
              Approve all in this group
            </Button>
          )}
        </div>
      </header>
      <ol className="prg-rows">
        {group.prs.map((row) => {
          const id = prRowId(row.number);
          return (
            <Row
              key={id}
              block={block}
              row={row}
              decision={verdicts[id]}
              locked={locked}
              expanded={open.has(id)}
              cursored={cursorId === id}
              onFocus={() => onFocusRow(id)}
              onToggle={() => onToggle(id)}
              send={send}
            />
          );
        })}
      </ol>
    </section>
  );
}

interface RowProps {
  block: PRGroupsBlock;
  row: PRRow;
  decision: Decision | undefined;
  locked: boolean;
  expanded: boolean;
  cursored: boolean;
  onFocus: () => void;
  onToggle: () => void;
  send: (merge: Record<string, VerdictEntry>) => Promise<boolean>;
}


const STATE_LABEL = { approved: 'Approved', held: 'Held', stale: 'Needs your tap', open: 'Waiting on you' } as const;

function Row({ block, row, decision, locked, expanded, cursored, onFocus, onToggle, send }: RowProps) {
  const id = prRowId(row.number);
  const v = rowVerdict(row, decision);
  const state = v.kind === 'stale' ? 'stale' : 'open';
  const detailId = `${block.id}-row-${id}`;

  function decide(target: 'approved' | 'rejected') {
    void send({ [id]: toggleEntry(row, decision, target) });
  }

  return (
    <li className={`prg-row prg-${state}${expanded ? ' is-open' : ''}${cursored ? ' cursor' : ''}`} data-row={id} onFocus={onFocus}>
      <button type="button" className="prg-row-head" aria-expanded={expanded} aria-controls={detailId} onClick={onToggle}>
        <span className="prg-row-line">
          <span className="prg-num">#{row.number}</span>
          <span className={`prg-state prg-state-${state}`}>{STATE_LABEL[state]}</span>
          <span className="prg-caret" aria-hidden>
            {expanded ? '▾' : '▸'}
          </span>
        </span>
        <span className="prg-title">{row.title}</span>
      </button>

      <dl className="prg-fields">
        {FIELDS.map((f) => (
          <div key={f.key} className={`prg-field prg-field-${f.key}`}>
            <dt>{f.label}</dt>
            <dd dangerouslySetInnerHTML={{ __html: renderInlineMarkdown(row[f.key]) }} />
          </div>
        ))}
      </dl>

      {v.kind === 'stale' && (
        <p className="prg-stale" role="status">
          {staleLine(row, v.decision)}
        </p>
      )}

      <div className="decision-bar prg-verdicts" role="radiogroup" aria-label={`#${row.number} ${row.title}`}>
        <Button
          variant="ghost"
          role="radio"
          aria-checked={false}
          disabled={locked}
          className="verdict verdict-approve"
          onClick={() => decide('approved')}
        >
          <span className="verdict-glyph" aria-hidden>
            ✓
          </span>
          Approve
        </Button>
        <Button
          variant="ghost"
          role="radio"
          aria-checked={false}
          disabled={locked}
          className="verdict verdict-hold"
          onClick={() => decide('rejected')}
        >
          <span className="verdict-glyph" aria-hidden>
            ‖
          </span>
          Hold
        </Button>
      </div>

      {decision?.note && <p className="prg-note">{decision.note}</p>}

      {expanded && (
        <div className="prg-detail" id={detailId}>
          {row.md && <div className="prose markdown-block prg-md" dangerouslySetInnerHTML={{ __html: renderMarkdown(row.md) }} />}
          <PRView block={{ id: block.id, type: 'pr', repo: block.repo, number: row.number, files: row.files }} />
        </div>
      )}
    </li>
  );
}

function RowNote({
  block,
  row,
  decision,
  send,
}: {
  block: PRGroupsBlock;
  row: PRRow;
  decision: Decision;
  send: (merge: Record<string, VerdictEntry>) => Promise<boolean>;
}) {
  const id = prRowId(row.number);
  const drafts = useDrafts<string>(block.id, ROW_NOTES);
  const draft = drafts.get(id);
  const ref = useRef<HTMLTextAreaElement>(null);

  function openEditor() {
    draftStore.setEntry(block.id, ROW_NOTES, id, decision.note ?? '');
    requestAnimationFrame(() => ref.current?.focus());
  }

  async function save() {
    const sent = draft ?? '';
    const entry: VerdictEntry = { verdict: decision.verdict, head: row.head };
    if (sent.trim()) entry.note = sent.trim();
    const ok = await send({ [id]: entry });
    if (ok && draftStore.entries<string>(block.id, ROW_NOTES).get(id) === sent) {
      draftStore.clearEntry(block.id, ROW_NOTES, id);
    }
  }

  if (draft === undefined) {
    return (
      <button type="button" className="link-btn prg-note-open" onClick={openEditor}>
        {decision.note ? 'Edit note' : 'Add a note for the agent'}
      </button>
    );
  }
  return (
    <div className="prg-note-editor">
      <textarea
        ref={ref}
        className="prg-note-input"
        rows={3}
        value={draft}
        placeholder="Why approve or hold, for the agent…"
        onChange={(e) => draftStore.setEntry(block.id, ROW_NOTES, id, e.target.value)}
      />
      <div className="prg-note-actions">
        <Button variant="primary" onClick={() => void save()}>
          Save note
        </Button>
        <Button variant="ghost" onClick={() => draftStore.clearEntry(block.id, ROW_NOTES, id)}>
          Cancel
        </Button>
      </div>
    </div>
  );
}
