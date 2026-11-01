import type { Meta, StoryObj } from '@storybook/react-vite';
import { ActivityFeed, type ActivityEntry } from './ActivityFeed';

// The newest entries of the activity log (concept 11, 11.4), as on the overview's card.

export const activityEntries: ActivityEntry[] = [
  {
    id: '6',
    icon: 'play',
    tone: 'accent',
    text: 'Anna hat «Oppenheimer» gestartet',
    time: 'vor 5 min',
    dateTime: '2026-11-01T12:40:00Z',
  },
  {
    id: '5',
    icon: 'warning',
    tone: 'danger',
    text: 'Keyframe-Index fehlgeschlagen: ffprobe: exit status 1',
    time: 'vor 12 min',
    dateTime: '2026-11-01T12:33:00Z',
  },
  {
    id: '4',
    icon: 'check',
    tone: 'success',
    text: 'Marco hat «Die Stunde der Dämmerung» fertig geschaut',
    time: '11:02',
    dateTime: '2026-11-01T10:02:00Z',
  },
  {
    id: '3',
    icon: 'archive',
    text: 'Sicherung erstellt',
    time: '03:00',
    dateTime: '2026-11-01T02:00:00Z',
  },
  {
    id: '2',
    icon: 'power',
    text: 'Server gestartet, Version 1.7.0',
    time: 'gestern',
    dateTime: '2026-10-31T07:12:00Z',
  },
  {
    id: '1',
    icon: 'folder',
    text: 'Bibliotheksscan Filme: 3 neue Titel, 1 entfernt, 2 Ordner ohne Zugriff übersprungen',
    time: 'gestern',
    dateTime: '2026-10-31T06:00:00Z',
  },
];

const meta = {
  title: 'UI/ActivityFeed',
  excludeStories: ['activityEntries'],
  component: ActivityFeed,
  args: { label: 'Letzte Aktivität', entries: activityEntries },
  decorators: [
    (Story) => (
      <div style={{ width: '32rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ActivityFeed>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Six entries, the newest first; a sentence too long for the line is cut. */
export const Default: Story = {};

/** On a phone: the sentences cut earlier, the times stay whole. */
export const Narrow: Story = {
  decorators: [
    (Story) => (
      <div style={{ width: '20rem' }}>
        <Story />
      </div>
    ),
  ],
};
