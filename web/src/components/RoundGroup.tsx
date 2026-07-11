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
    submitted: interactions.submitted,
    closed: interactions.closed,
  };

  const all = flatten(record.blocks);
  const approved = all.filter((b) => b.type === 'approval' && record.decisions[b.id]?.verdict === 'approved').length;
  const rejected = all.filter((b) => b.type === 'approval' && record.decisions[b.id]?.verdict === 'rejected').length;
  const picks = all.filter(
    (b) =>
      (b.type === 'choice' && (record.choices[b.id]?.optionIds.length ?? 0) > 0) ||
      (isPackBlock(b) && record.packs[b.id] !== undefined),
  ).length;
  const filledInputs = all.filter((b) => b.type === 'input' && (record.inputs[b.id]?.text.trim() ?? '') !== '').length;
  const notes = filledInputs + all.reduce((n, b) => n + (record.feedback[b.id]?.length ?? 0), 0);

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
