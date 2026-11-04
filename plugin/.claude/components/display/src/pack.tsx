import './display.css';
import { Artifact } from './Artifact';
import { Compare } from './Compare';
import { DesignDoc } from './DesignDoc';
import { Page } from './Page';
import { Sequence } from './Sequence';
import { System } from './System';
import { Timeline } from './Timeline';

export default {
  hostApi: 1,
  blocks: { page: Page, sequence: Sequence, timeline: Timeline, compare: Compare, artifact: Artifact, system: System, designdoc: DesignDoc },
};
