import type { Meta, StoryObj } from '@storybook/react-vite';
import { SquareCover } from './SquareCover';
import { Stack } from './Stack';
import { art } from './stories/fixtures';

const meta = {
  title: 'UI/SquareCover',
  component: SquareCover,
  args: { title: 'Wissenschaftsmagazin' },
} satisfies Meta<typeof SquareCover>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A cover from the server in the three sizes; a picture that is not square is cut to fit. */
export const Image: Story = {
  render: () => (
    <Stack direction="row" gap={4}>
      <SquareCover title="Wissenschaftsmagazin" src={art.posterMoss} size="sm" />
      <SquareCover title="Wissenschaftsmagazin" src={art.posterMoss} size="md" />
      <SquareCover title="Wissenschaftsmagazin" src={art.posterMoss} size="lg" />
    </Stack>
  ),
};

/** Without a cover: the initials on the colour the title gives. */
export const Initials: Story = {
  render: () => (
    <Stack gap={4}>
      <Stack direction="row" gap={4}>
        <SquareCover title="Echo der Zeit" size="sm" />
        <SquareCover title="ZEIT WISSEN. Woher weisst Du das?" size="md" />
        <SquareCover title="{ungeskriptet} - Der Meinungsfreiheit verpflichtet." size="lg" />
      </Stack>
      <Stack direction="row" gap={4}>
        {['Tagesgespräch', 'Weltwoche Daily', 'NZZ Akzent', 'Hotel Matze', 'Espresso'].map(
          (title) => (
            <SquareCover key={title} title={title} size="sm" />
          ),
        )}
      </Stack>
    </Stack>
  ),
};

/** A cover that fails to load gives way to the initials. */
export const Failed: Story = {
  render: () => <SquareCover title="Sternstunde Philosophie" src="missing-cover.webp" size="lg" />,
};
