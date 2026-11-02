import type { Meta, StoryObj } from '@storybook/react-vite';
import { ActivityFeed } from './ActivityFeed';
import { activityEntries } from './ActivityFeed.stories';
import { Button } from './Button';
import { DashCard } from './DashCard';
import { Stack } from './Stack';
import { StatTile } from './StatTile';
import { Text } from './Text';
import { TextLink } from './TextLink';

// A card of the administration's overview (concept 11, 11.1). The overview lays them out in two
// columns; every card fills its cell, so a row's cards are equally high.

const meta = {
  title: 'UI/DashCard',
  component: DashCard,
  args: {
    icon: 'server',
    title: 'Server',
    children: (
      <Text tone="secondary">Slouchcat 1.7.0, läuft seit 3 Tagen. Transcoding mit NVENC.</Text>
    ),
    footer: (
      <>
        <Button icon="refresh">Nach Updates suchen</Button>
        <Button variant="ghost" icon="power">
          Neu starten
        </Button>
      </>
    ),
  },
  decorators: [
    (Story) => (
      <div style={{ width: '32rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof DashCard>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Head with icon and title, body, buttons in the footer. */
export const Default: Story = {};

/** A number in the head, read out as "3 Hinweise"; a link in the footer. */
export const WithCount: Story = {
  args: {
    icon: 'warning',
    title: 'Hinweise',
    count: '3',
    countLabel: '3 Hinweise',
    children: <Text tone="secondary">265 Aufgaben fehlgeschlagen.</Text>,
    footer: <TextLink href="#jobs">Hintergrundjobs</TextLink>,
  },
};

/** Two cards in a row: the shorter one stretches, its footer stays at the bottom. */
export const Row: Story = {
  decorators: [
    (Story) => (
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
          gap: 'var(--sc-space-4)',
          width: '56rem',
        }}
      >
        <Story />
      </div>
    ),
  ],
  render: () => (
    <>
      <DashCard
        icon="refresh"
        title="Aufgaben"
        footer={<TextLink href="#jobs">Hintergrundjobs</TextLink>}
      >
        <Stack direction="row" gap={2}>
          <StatTile label="Wartend" value="4" />
          <StatTile label="Laufend" value="1" tone="success" />
          <StatTile label="Fehlgeschlagen" value="265" tone="danger" />
        </Stack>
      </DashCard>
      <DashCard icon="activity" title="Letzte Aktivität">
        <ActivityFeed label="Letzte Aktivität" entries={activityEntries} />
      </DashCard>
    </>
  ),
};
