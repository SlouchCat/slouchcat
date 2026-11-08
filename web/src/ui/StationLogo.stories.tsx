import type { Meta, StoryObj } from '@storybook/react-vite';
import { Stack } from './Stack';
import { art } from './stories/fixtures';
import { StationLogo } from './StationLogo';

const meta = {
  title: 'UI/StationLogo',
  component: StationLogo,
  args: { name: 'Radio Swiss Pop' },
} satisfies Meta<typeof StationLogo>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A logo from the server (a picture of the Storybook build). */
export const Image: Story = {
  render: () => (
    <Stack direction="row" gap={4}>
      <StationLogo name="Radio Swiss Pop" src={art.posterMoss} size="sm" />
      <StationLogo name="Radio Swiss Pop" src={art.posterMoss} size="md" />
      <StationLogo name="Radio Swiss Pop" src={art.posterMoss} size="lg" />
    </Stack>
  ),
};

/** Without a logo: the initials on the colour the name gives, in every size. */
export const Initials: Story = {
  render: () => (
    <Stack gap={4}>
      <Stack direction="row" gap={4}>
        <StationLogo name="SRF 3" size="sm" />
        <StationLogo name="Radio Energy Zürich" size="md" />
        <StationLogo name="Couleur 3" size="lg" />
      </Stack>
      <Stack direction="row" gap={4}>
        {['SRF 1', 'Radio 24', 'Kanal K', 'Radio Bern1', 'FM4', 'Jazz Radio', 'Rete Uno'].map(
          (name) => (
            <StationLogo key={name} name={name} size="md" />
          ),
        )}
      </Stack>
    </Stack>
  ),
};

/** A long name keeps three characters; the tile fills its container. */
export const LongName: Story = {
  render: () => (
    <div style={{ width: '12rem' }}>
      <StationLogo name="Radio Rumantsch Grischun Engiadina Bassa Val Müstair" size="fill" />
    </div>
  ),
};

/** A cover that fails to load gives way to the station's logo (concept 16, 8 point 6). */
export const CoverFallback: Story = {
  render: () => (
    <Stack direction="row" gap={4}>
      <StationLogo
        name="Radio Swiss Pop"
        src="missing-cover.webp"
        fallbackSrc={art.posterMoss}
        size="lg"
      />
      <StationLogo name="Kanal K" src="missing-cover.webp" size="lg" />
    </Stack>
  ),
};
