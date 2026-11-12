import type { Meta, StoryObj } from '@storybook/react-vite';
import { EpisodeListItem } from './EpisodeListItem';
import { accents, art } from './stories/fixtures';

const meta = {
  title: 'UI/EpisodeListItem',
  component: EpisodeListItem,
  args: {
    playHref: '#',
    playLabel: 'Abspielen',
    detailsHref: '#',
    detailsLabel: 'Details',
    detailsName: 'Details: S01E02 · Die lange Nacht',
    code: 'S01E02',
    title: 'Die lange Nacht',
    overview:
      'Ein Stromausfall legt die Stadt lahm. Weber sucht im Dunkeln nach dem Zeugen, der als Einziger weiss, wer in der Nacht am Hafen war, und findet mehr, als ihr lieb ist.',
    caption: '45 min',
    stillUrl: art.backdropDusk,
    accent: accents.dusk,
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '52rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof EpisodeListItem>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Watched: Story = { args: { watchedLabel: 'Gesehen' } };

/** The episode the big button plays: marked, with its hint after the code. */
export const NextUp: Story = {
  args: { highlight: 'Weiterschauen', progress: { percent: 42, label: '42 % gesehen' } },
};

/** No still and no overview yet (no metadata match). */
export const Bare: Story = {
  args: { code: 'S01E03–E04', title: 'Episode 3', stillUrl: undefined, overview: undefined },
};

/** Focus on the row's play link (Tab): the round play button turns to the accent. */
export const Focused: Story = { tags: ['press:Tab'] };

/** Tab again: «Details», which opens the episode's page (concept 20 P-4). */
export const DetailsFocused: Story = { tags: ['press:Tab+Tab'] };
