import type { Meta, StoryObj } from '@storybook/react-vite';
import { PlayedList } from './PlayedList';
import { art } from './stories/fixtures';

const meta = {
  title: 'UI/PlayedList',
  component: PlayedList,
  args: {
    heading: 'Zuvor gespielt',
    empty: 'Vor diesem Titel hat der Sender noch keine Titel gemeldet.',
    stationName: 'Radio Swiss Pop',
    logoUrl: art.posterMoss,
    entries: [
      {
        key: '3',
        title: 'Flowers',
        subtitle: 'Miley Cyrus',
        time: '14:28',
        dateTime: '2026-11-08T13:28:00.000Z',
        coverUrl: art.posterEmber,
      },
      {
        key: '2',
        title: 'Hello',
        subtitle: 'Adele',
        time: '14:24',
        dateTime: '2026-11-08T13:24:00.000Z',
        coverUrl: art.posterDusk,
      },
      {
        key: '1',
        title: 'A title so long that it does not fit into the line of the list at all',
        subtitle: 'An artist with a long name as well',
        time: '14:19',
        dateTime: '2026-11-08T13:19:00.000Z',
      },
    ],
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '28rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PlayedList>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Covers where found; an entry without one shows the station's logo. */
export const WithCovers: Story = {};

/** A station without a logo: the initials stand in for the missing covers. */
export const WithoutLogo: Story = {
  args: { stationName: 'Kanal K', logoUrl: undefined },
};

/** No earlier titles yet. */
export const Empty: Story = { args: { entries: [] } };
