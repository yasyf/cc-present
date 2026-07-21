import { Callout } from './Callout';
import { Rating } from './Rating';
import { Survey } from './Survey';

// Default export = the pack module. The host qualifies these bare names with the
// manifest's pack name (example.callout, example.rating, example.survey).
export default {
  hostApi: 1,
  blocks: { callout: Callout, rating: Rating, survey: Survey },
};
