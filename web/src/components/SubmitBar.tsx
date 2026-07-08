import { useQuery } from '@tanstack/react-query';
import type { Block, Doc } from '../schema';
import type { Interactions } from '../events';
import { usePresent } from '../present';
import { revisionKey } from '../api';

export interface SubmitBarProps {
  // The current round's live blocks; the decided/total tally spans only these.
  blocks: Block[];
  doc: Doc;
  interactions: Interactions;
  subject: string;
  hasHistory: boolean;
}

// flatten yields every top-level block plus every card child, so the decided /
// total tally spans approvals and choices wherever they nest.
function flatten(blocks: Block[]): Block[] {
  const out: Block[] = [];
  for (const block of blocks) {
    out.push(block);
    if (block.type === 'card') out.push(...block.children);
  }
  return out;
}

export function SubmitBar({ blocks, doc, interactions, subject, hasHistory }: SubmitBarProps) {
  const { post, closed, currentRound } = usePresent();
  const { data: revision } = useQuery<number>({
    queryKey: revisionKey(subject),
    queryFn: () => 0,
    initialData: 0,
    staleTime: Infinity,
  });

  const all = flatten(blocks);
  const approvalIds = all.filter((b) => b.type === 'approval').map((b) => b.id);
  const choiceIds = all.filter((b) => b.type === 'choice').map((b) => b.id);
  const total = approvalIds.length + choiceIds.length;

  if (total === 0 && !doc.submit) return null;

  const decidedApprovals = approvalIds.filter((id) => interactions.decisions[id] !== undefined).length;
  const decidedChoices = choiceIds.filter((id) => (interactions.choices[id]?.optionIds.length ?? 0) > 0).length;
  const decided = decidedApprovals + decidedChoices;
  const undecidedApprovals = approvalIds.length - decidedApprovals;

  const label = doc.submit?.label ?? 'Submit';
  const submitted = interactions.submitted;
  // Right after a submit the round advances and its blocks belong to history, so
  // the live round is momentarily empty; there is nothing to submit until the
  // next round's content lands.
  const empty = blocks.length === 0;

  function submit() {
    if (undecidedApprovals > 0) {
      const noun = undecidedApprovals === 1 ? 'approval is' : 'approvals are';
      if (!window.confirm(`${undecidedApprovals} ${noun} still undecided. Submit anyway?`)) return;
    }
    post({ type: 'submit', revision });
  }

  return (
    <div className="submit-bar">
      <div className="submit-status">
        {hasHistory && <span className="submit-round">Round {currentRound}</span>}
        <span className="submit-count">
          {decided}/{total} decided
        </span>
        {empty && submitted.value && (
          <span className="submit-done">submitted · rev {submitted.revision}</span>
        )}
        {doc.submit?.note && <span className="submit-note">{doc.submit.note}</span>}
      </div>
      <button type="button" className="primary submit-btn" disabled={closed || empty} onClick={submit}>
        {label}
      </button>
    </div>
  );
}
