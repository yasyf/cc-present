import { CollapsedGroup } from '@cc-interact/react';
import { isPackBlock } from '../schema';
import type { Interactions, RoundRecord } from '../events';
import { flatten } from '../decide';
import { BlockRenderer } from './BlockRenderer';

// RoundGroup renders one closed round as a read-only CollapsedGroup: the frozen
// blocks and interaction snapshot from the record, with replies threaded live so
// a late agent reply still surfaces inside the old round.
export function RoundGroup({ record, interactions }: { record: RoundRecord; interactions: Interactions }) {
  const frozen: Interactions = {
    decisions: record.decisions,
    choices: record.choices,
    inputs: record.inputs,
    packs: record.packs,
    feedback: record.feedback,
    replies: interactions.replies,
    annotations: record.annotations,
    triage: record.triage,
    submitted: interactions.submitted,
    closed: interactions.closed,
  };

  const all = flatten(record.blocks);
  // Triage item verdicts are the same currency as approval verdicts, so the
  // approve/reject chips count them individually alongside approvals.
  const triageVerdicts = (b: (typeof all)[number], v: 'approved' | 'rejected') =>
    b.type === 'triage' ? Object.values(record.triage[b.id] ?? {}).filter((d) => d.verdict === v).length : 0;
  const approved =
    all.filter((b) => b.type === 'approval' && record.decisions[b.id]?.verdict === 'approved').length +
    all.reduce((n, b) => n + triageVerdicts(b, 'approved'), 0);
  const rejected =
    all.filter((b) => b.type === 'approval' && record.decisions[b.id]?.verdict === 'rejected').length +
    all.reduce((n, b) => n + triageVerdicts(b, 'rejected'), 0);
  const picks = all.filter(
    (b) =>
      (b.type === 'choice' &&
        ((record.choices[b.id]?.optionIds.length ?? 0) > 0 || record.choices[b.id]?.other !== undefined)) ||
      (isPackBlock(b) && record.packs[b.id] !== undefined),
  ).length;
  const filledInputs = all.filter((b) => b.type === 'input' && (record.inputs[b.id]?.text.trim() ?? '') !== '').length;
  const triageNotes = all.reduce(
    (n, b) =>
      b.type === 'triage' ? n + Object.values(record.triage[b.id] ?? {}).filter((d) => (d.note ?? '') !== '').length : n,
    0,
  );
  const annotationNotes = all.reduce((n, b) => n + (record.annotations[b.id]?.length ?? 0), 0);
  const notes =
    filledInputs +
    triageNotes +
    annotationNotes +
    all.reduce((n, b) => n + (record.feedback[b.id]?.length ?? 0), 0);

  const header = (
    <span className="round-header">
      <span className="round-title">
        Round {record.number}
        {record.title && ` · ${record.title}`} · {record.blocks.length} blocks
        {record.submittedRevision != null && ' · submitted'}
      </span>
      <span className="round-summary">
        {approved > 0 && <span className="round-chip">✓ {approved}</span>}
        {rejected > 0 && <span className="round-chip">✗ {rejected}</span>}
        {picks > 0 && (
          <span className="round-chip">
            {picks} {picks === 1 ? 'pick' : 'picks'}
          </span>
        )}
        {notes > 0 && (
          <span className="round-chip">
            {notes} {notes === 1 ? 'note' : 'notes'}
          </span>
        )}
      </span>
    </span>
  );

  return (
    <CollapsedGroup readOnly header={header}>
      {record.blocks.map((b) => (
        <BlockRenderer key={b.id} block={b} interactions={frozen} />
      ))}
    </CollapsedGroup>
  );
}
