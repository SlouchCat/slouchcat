import type { Meta, StoryObj } from '@storybook/react-vite';
import { SpatialButton } from './Button';
import { SpatialGroup, SpatialPage } from './spatial';
import { Stack } from './Stack';

// Arrow-key navigation (norigin) as on the browsing pages: the first arrow press lands on the
// default element, the next ones move between the spatial items.
function SpatialDemo() {
  return (
    <SpatialPage defaultFocusKey="story-play">
      <Stack gap={4}>
        <SpatialGroup focusKey="story-actions">
          <Stack direction="row" gap={2}>
            <SpatialButton focusKey="story-play" variant="primary" icon="play">
              Abspielen
            </SpatialButton>
            <SpatialButton variant="secondary" icon="info">
              Details
            </SpatialButton>
            <SpatialButton variant="ghost" icon="back">
              Zurück
            </SpatialButton>
          </Stack>
        </SpatialGroup>
      </Stack>
    </SpatialPage>
  );
}

const meta = {
  title: 'UI/Spatial',
  component: SpatialDemo,
} satisfies Meta<typeof SpatialDemo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Rest: Story = {};

/** ArrowDown lands on the default element, ArrowRight moves on. */
export const ArrowNavigation: Story = { tags: ['press:ArrowDown+ArrowRight'] };
