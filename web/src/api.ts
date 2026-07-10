// The query layer: the daemon-tuned QueryClient, the subject/scope context, the
// cache keys, and the single optimistic mutation every interaction flows
// through. POST /api/interactions carries {subject, nonce, interaction}; the
// optimistic patch runs the same reduction locally and the SSE echo reconciles.

import {
  createQueryClient,
  createSubjectContext,
  request,
  scopedKey,
  useOptimisticMutation,
} from '@cc-interact/react';
import type { UseMutationResult } from '@tanstack/react-query';
import { applyInteraction } from './reduce';
import type { Interaction, PresentState } from './events';

export const queryClient = createQueryClient();

const subjectContext = createSubjectContext<string>();
export const SubjectProvider = subjectContext.SubjectProvider;
export const useSubject = subjectContext.useSubject;

export function presentKey(subject: string) {
  return scopedKey('present', subject, undefined);
}

export function revisionKey(subject: string) {
  return scopedKey('present-revision', subject, undefined);
}

export function usePostInteraction(
  subject: string,
  onError: (err: Error, interaction: Interaction) => void,
): UseMutationResult<unknown, Error, Interaction> {
  return useOptimisticMutation<Interaction, unknown, PresentState>({
    mutationFn: (interaction) =>
      request<unknown>('/api/interactions', {
        method: 'POST',
        body: JSON.stringify({ subject, nonce: crypto.randomUUID(), interaction }),
      }),
    queryKey: () => presentKey(subject),
    applyOptimistic: (cache, interaction) => applyInteraction(cache, interaction),
    onError,
    // Serialize posts per subject so append order matches action order — the
    // keyboard submit blurs a focused input first, and that blur-commit must
    // reach the daemon before the submit it triggered.
    scope: subject,
  });
}
