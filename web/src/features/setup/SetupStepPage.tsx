import { getRouteApi } from '@tanstack/react-router';
import { AccessStep } from './AccessStep';
import { HouseholdStep } from './HouseholdStep';
import { LibrariesStep } from './LibrariesStep';
import { PlaybackStep } from './PlaybackStep';
import { RegionStep } from './RegionStep';
import { ServicesStep } from './ServicesStep';
import { setupStepOf } from './setupState';
import { SummaryStep } from './SummaryStep';

const route = getRouteApi('/_authed/_active/setup/$step');

/** The owner's steps 3 to 9 (concept 22, 3.3), one file each; the route checked the step. */
export function SetupStepPage() {
  const { step } = route.useParams();
  switch (setupStepOf(step)) {
    case 'services':
      return <ServicesStep />;
    case 'playback':
      return <PlaybackStep />;
    case 'libraries':
      return <LibrariesStep />;
    case 'household':
      return <HouseholdStep />;
    case 'access':
      return <AccessStep />;
    case 'summary':
      return <SummaryStep />;
    default:
      return <RegionStep key={step} />;
  }
}
