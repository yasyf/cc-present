// Pure derivations for the SubmitBar's armed confirm.

// undecidedKey folds an unordered set of undecided approval ids into a stable
// string so the armed confirm can key on the exact set, not just its count. A
// same-round block swap that preserves the count still changes the set, so the
// key changes and the stale "Submit anyway?" derives false.
export function undecidedKey(ids: readonly string[]): string {
  return [...ids].sort().join('\n');
}
