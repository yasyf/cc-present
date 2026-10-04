import './display.css';
import { Artifact } from './Artifact';
import { Compare } from './Compare';
import { Page } from './Page';
import { Sequence } from './Sequence';
import { Timeline } from './Timeline';

export default {
  hostApi: 1,
  blocks: { page: Page, sequence: Sequence, timeline: Timeline, compare: Compare, artifact: Artifact },
};
