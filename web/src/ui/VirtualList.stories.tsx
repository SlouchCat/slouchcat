import type { Meta, StoryObj } from '@storybook/react-vite';
import { InboxEntry } from './InboxEntry';
import { Skeleton } from './Skeleton';
import { StationTile } from './StationTile';
import styles from './stories/VirtualListStory.module.css';
import { VirtualList } from './VirtualList';

// A list operation's entries without «Mehr laden» (concept 18 C-3): a long list draws only the
// rows near the view; an entry whose page is on its way shows its placeholder.

interface Entry {
  id: number;
  title: string;
}

const entries = (count: number, missingFrom = Infinity): (Entry | undefined)[] =>
  Array.from({ length: count }, (_, index) =>
    index >= missingFrom
      ? undefined
      : { id: index, title: `S01E${String(index + 1).padStart(2, '0')} · Folge ${index + 1}` },
  );

const meta = {
  title: 'UI/VirtualList',
  component: VirtualList<Entry>,
  args: {
    slots: entries(6),
    virtual: false,
    label: 'Mitteilungen',
    estimate: 72,
    className: styles.list,
    itemKey: (entry: Entry) => entry.id,
    renderPlaceholder: () => <Skeleton />,
    renderItem: (entry: Entry) => (
      <InboxEntry
        href="#"
        title={entry.title}
        context="Neue Folge von Die Stunde der Dämmerung"
        time="21.10.2026, 08:15"
        unread={entry.id % 3 === 0}
        label={entry.title}
      />
    ),
  },
  decorators: [
    (Story) => (
      <div style={{ padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof VirtualList<Entry>>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A short list: every entry is drawn. */
export const Short: Story = {};

/** 5'000 entries: only the rows near the view are in the document; the rest waits for its page. */
export const Long: Story = { args: { slots: entries(5_000, 4), virtual: true } };

/** A grid of station tiles (the radio page): the columns come from the list's CSS grid. */
export const Grid: Story = {
  args: {
    layout: 'grid',
    virtual: true,
    className: styles.grid,
    slots: entries(40, 10),
    estimate: 200,
    renderPlaceholder: () => (
      <span className={styles.placeholder}>
        <Skeleton shape="poster" />
      </span>
    ),
    renderItem: (entry: Entry) => (
      <StationTile
        name={`Sender ${entry.id + 1}`}
        caption="Schweiz · Bern"
        playLabel={`Sender ${entry.id + 1} abspielen`}
        onPlay={() => undefined}
        favorite={{ pressed: entry.id === 1, label: 'Meine Sender', onPress: () => undefined }}
      />
    ),
  },
};
