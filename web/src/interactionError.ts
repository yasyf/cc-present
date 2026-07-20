// The toast text shown when a human interaction POST fails, shared by the full
// board and single-block views. The switch is exhaustive over Interaction.

import type { Interaction } from './events';

export function interactionErrorText(interaction: Interaction): string {
  switch (interaction.type) {
    case 'decision.created':
      return 'Could not record your verdict. Check your connection and try again.';
    case 'choice.selected':
      return 'Could not record your choice. Check your connection and try again.';
    case 'feedback.created':
      return 'Could not send your feedback. Check your connection and try again.';
    case 'annotation.created':
      return 'Could not save your note. Check your connection and try again.';
    case 'annotation.removed':
      return 'Could not remove your note. Check your connection and try again.';
    case 'triage.decided':
      return 'Could not record your verdict. Check your connection and try again.';
    case 'input.submitted':
      return 'Could not save your input. Check your connection and try again.';
    case 'pack.interaction':
      return 'Could not record your interaction. Check your connection and try again.';
    case 'submit':
      return 'Could not submit. Check your connection and try again.';
  }
}
