import type { Meta, StoryObj } from '@storybook/react-vite';
import { VolumeControl } from './VolumeControl';

const noop = () => undefined;

const meta = {
  title: 'UI/VolumeControl',
  component: VolumeControl,
  args: { volume: 0.8, muted: false, onVolumeChange: noop, onToggleMute: noop },
} satisfies Meta<typeof VolumeControl>;

export default meta;
type Story = StoryObj<typeof meta>;

/** At rest only the mute button shows; the slider opens under the pointer or the focus. */
export const Default: Story = {};

/**
 * Tab to the button opens the slider beside it as a popover over the neighbours (concept 19 E2),
 * with the percentage.
 */
export const Focused: Story = { tags: ['press:Tab'] };

/** In the mini player the popover opens above the button. */
export const Above: Story = {
  args: { placement: 'above' },
  tags: ['press:Tab'],
  decorators: [
    (Story) => (
      <div style={{ paddingTop: '5rem', paddingLeft: '5rem' }}>
        <Story />
      </div>
    ),
  ],
};

/** On a touch screen (no hover) the slider stays open in the row and covers nothing. */
export const Inline: Story = { args: { inline: true } };

/** The focus on the slider itself. */
export const SliderFocused: Story = { tags: ['press:Tab+Tab'] };

/** Muted: the crossed-out speaker; the slider stands at 0. */
export const Muted: Story = { args: { muted: true }, tags: ['press:Tab'] };

/** With boost the slider goes to 150 %; above 100 % the fill and the number turn to a warning. */
export const Boosted: Story = { args: { boost: true, volume: 1.3 }, tags: ['press:Tab'] };

/** iPhone: the page cannot set the volume, only mute it. */
export const MuteOnly: Story = { args: { muteOnly: true }, tags: ['press:Tab'] };
