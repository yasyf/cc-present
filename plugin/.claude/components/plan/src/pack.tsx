import './plan.css';
import { Calls } from './Calls';
import { Machine } from './Machine';
import { Mock } from './Mock';

export default {
  hostApi: 1,
  blocks: { calls: Calls, machine: Machine, mock: Mock },
};
