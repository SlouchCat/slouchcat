import type { Meta, StoryObj } from '@storybook/react-vite';
import { WatchedToggle } from './WatchedToggle';
import { art } from './stories/fixtures';

const meta = {
  title: 'UI/WatchedToggle',
  component: WatchedToggle,
  args: {
    state: 'none',
    label: 'Gesehen: Die Stunde der Dämmerung',
    title: 'Als gesehen markieren',
    onPress: () => undefined,
  },
  decorators: [
    (Story) => (
      // A piece of a poster under the cover disc, so its dark ground shows as on a card.
      <div
        style={{
          display: 'inline-flex',
          padding: 'var(--sc-space-6)',
          backgroundImage: `url(${art.posterDusk})`,
          backgroundSize: 'cover',
        }}
      >
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof WatchedToggle>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Not watched: the disc with a faint check (on a poster card only on hover or focus, O-1). */
export const CoverNone: Story = {};

/** Partly watched (a season, series or podcast, F-12): the left half in the accent. */
export const CoverHalf: Story = {
  args: { state: 'half', label: 'Gesehen: Nordlicht', title: 'Teilweise gesehen' },
};

/** Watched: filled in the accent. */
export const CoverFull: Story = { args: { state: 'full', title: 'Haken entfernen' } };

/** Keyboard focus: the ring and the tool's growth. */
export const CoverFocused: Story = { tags: ['press:Tab'] };

/** Offline the check is a display only (rule 12). */
export const CoverReadOnly: Story = {
  args: { state: 'full', readOnly: true, stateLabel: 'Gesehen' },
};

/** The ring at the far right of an episode row: empty. */
export const RowNone: Story = { args: { appearance: 'row', label: 'Gesehen: S01E02' } };

export const RowHalf: Story = { args: { appearance: 'row', state: 'half' } };

export const RowFull: Story = { args: { appearance: 'row', state: 'full' } };

export const RowFocused: Story = { tags: ['press:Tab'], args: { appearance: 'row' } };

/** Offline in a row: the empty ring stays as a display. */
export const RowReadOnly: Story = {
  args: { appearance: 'row', state: 'none', readOnly: true },
};
