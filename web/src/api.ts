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
import { useQuery } from '@tanstack/react-query';
import type { UseMutationResult } from '@tanstack/react-query';
import { applyInteraction } from './reduce';
import { withToken } from './token';
import { uuid } from './uuid';
import { isRoundStub } from './events';
import type { Interaction, PresentState, RoundEntry, RoundRecord } from './events';

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

export function snapshotKey(subject: string) {
  return scopedKey('present-snapshot', subject, undefined);
}

function roundsKey(subject: string) {
  return scopedKey('present-rounds', subject, undefined);
}

export async function loadSnapshot(subject: string): Promise<number> {
  const snapshot = await request<{ seq: number; revision: number; state: PresentState }>(
    withToken(`/api/state?session=${encodeURIComponent(subject)}`),
  );
  queryClient.setQueryData(presentKey(subject), snapshot.state);
  queryClient.setQueryData(revisionKey(subject), snapshot.revision);
  return snapshot.seq;
}

export interface RoundRecords {
  records?: RoundRecord[];
  failed: boolean;
}

export function useRoundRecords(subject: string, history: RoundEntry[], enabled: boolean): RoundRecords {
  const stubbed = history.some(isRoundStub);
  const { data, isError } = useQuery({
    queryKey: roundsKey(subject),
    queryFn: () =>
      request<{ history: RoundRecord[] }>(withToken(`/api/rounds?session=${encodeURIComponent(subject)}`)).then(
        (res) => res.history,
      ),
    enabled: enabled && stubbed,
    staleTime: Infinity,
    gcTime: Infinity,
  });
  if (!stubbed) return { records: history.filter((entry): entry is RoundRecord => !isRoundStub(entry)), failed: false };
  if (!data) return { failed: isError };
  return {
    records: history.map((entry) => (isRoundStub(entry) ? data.find((r) => r.number === entry.number)! : entry)),
    failed: false,
  };
}

export function usePostInteraction(
  subject: string,
  onError: (err: Error, interaction: Interaction) => void,
): UseMutationResult<unknown, Error, Interaction> {
  return useOptimisticMutation<Interaction, unknown, PresentState>({
    mutationFn: (interaction) =>
      request<unknown>(withToken('/api/interactions'), {
        method: 'POST',
        body: JSON.stringify({ subject, nonce: uuid(), interaction }),
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
