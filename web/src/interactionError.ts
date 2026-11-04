// The toast text shown when a human interaction POST fails, shared by the full
// board and single-block views.

import type { Interaction } from './events';

const FAILED: Record<Interaction['type'], string> = {
  'decision.created': 'Could not record your verdict',
  'choice.selected': 'Could not record your choice',
  'feedback.created': 'Could not send your feedback',
  'annotation.created': 'Could not save your note',
  'annotation.removed': 'Could not remove your note',
  'triage.decided': 'Could not record your verdict',
  'input.submitted': 'Could not save your input',
  'pack.interaction': 'Could not record your interaction',
  submit: 'Could not submit',
};

export function interactionErrorText(interaction: Interaction, err: Error): string {
  const failed = FAILED[interaction.type];
  if (err instanceof TypeError) return `${failed}. Check your connection and try again.`;
  const refusal = /failed \(\d+\): ([\s\S]*)$/.exec(err.message)?.[1]?.trim();
  return refusal ? `${failed}: ${refusal}` : `${failed}.`;
}
