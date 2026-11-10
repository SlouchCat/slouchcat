import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button } from './Button';
import { accents } from './stories/fixtures';
import { VolumeRow } from './VolumeRow';

const meta = {
  title: 'UI/VolumeRow',
  component: VolumeRow,
  args: {
    href: '#',
    number: '1',
    title: 'Die Stunde der Dämmerung',
    caption: 'Mara Lindqvist',
    accent: accents.dusk,
    duration: '11 h 11 min',
    playHref: '#',
    playLabel: 'Abspielen: Die Stunde der Dämmerung',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '48rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof VolumeRow>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Half heard: the time left and the bar under the title. */
export const WithProgress: Story = {
  args: { caption: 'noch 5 h 50 min', progress: { percent: 48, label: '48 % gehört' } },
};

/** Finished: a check mark on the dimmed cover. */
export const Heard: Story = { args: { caption: 'Gehört', watchedLabel: 'Gehört' } };

/** A prequel with a half number, and the pen for the administrator. */
export const PrequelWithPen: Story = {
  args: {
    number: '0,5',
    tool: (
      <Button
        variant="secondary"
        size="sm"
        icon="pen"
        iconOnly
        overlay
        aria-label="Die Stunde der Dämmerung: Metadaten bearbeiten"
      />
    ),
  },
};

/** A volume without a number: the column stays empty. */
export const WithoutNumber: Story = { args: { number: undefined } };

/** Focus on the volume (Tab), then on its play button (Tab again). */
export const Focused: Story = { tags: ['press:Tab'] };

export const PlayFocused: Story = { tags: ['press:Tab+Tab'] };
