import { Card } from './Card';
import { Commits } from './Commits';
import { Diff } from './Diff';

export default {
  hostApi: 1,
  blocks: { card: Card, diff: Diff, commits: Commits },
};
