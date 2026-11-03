import type { Meta, StoryObj } from '@storybook/react-vite';
import { AriaButton } from './AriaButton';
import { Chip } from './Chip';
import { SectionHeader } from './SectionHeader';
import { Stack } from './Stack';

const meta = {
  title: 'UI/SectionHeader',
  component: SectionHeader,
  args: {
    title: 'Letzte Fehlschläge',
    description: 'Die zehn jüngsten Fehlschläge aller Jobarten.',
    actions: <Chip tone="danger">265 fehlgeschlagen</Chip>,
  },
  decorators: [
    (Story) => (
      <div style={{ width: '48rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof SectionHeader>;

export default meta;
type Story = StoryObj<typeof meta>;

/** h2 of a section with a line below and state at its end. */
export const Default: Story = {};

/** h2 and an h3 of a thing inside the section (S-3). */
const Levels = () => (
  <Stack gap={4}>
    <SectionHeader title="Schlüssel der Anbieter" actions={<AriaButton>Neu</AriaButton>} />
    <SectionHeader title="TMDB" level={3} description="Erkennt Filme und Serien." />
  </Stack>
);

export const WithLevels: Story = { render: Levels };

export const Compact: Story = { globals: { density: 'compact' } };

export const CompactLevels: Story = { render: Levels, globals: { density: 'compact' } };
