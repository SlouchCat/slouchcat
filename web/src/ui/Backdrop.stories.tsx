import type { Meta, StoryObj } from '@storybook/react-vite';
import { Backdrop } from './Backdrop';
import { Stack } from './Stack';
import { accents, art } from './stories/fixtures';
import { Text } from './Text';

const meta = {
  title: 'UI/Backdrop',
  component: Backdrop,
  parameters: { layout: 'fullscreen' },
  args: { url: art.backdropDusk },
  decorators: [
    (Story) => (
      <div style={{ position: 'relative', isolation: 'isolate', minHeight: '100vh' }}>
        <Story />
        <div style={{ padding: '22rem var(--sc-space-10) 0', maxWidth: '40rem' }}>
          <Stack gap={4}>
            <Text as="h1" variant="display">
              Die Stunde der Dämmerung
            </Text>
            <Text tone="secondary">
              Ein Leuchtturmwärter zählt die Schiffe, die nicht mehr kommen. Text auf dem Scrim
              behält seinen Kontrast, mit und ohne Tint.
            </Text>
          </Stack>
        </div>
      </div>
    ),
  ],
} satisfies Meta<typeof Backdrop>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Tinted by the title's accent: hue from the accent, luminance from artwork and scrim. */
export const WithAccent: Story = { args: { accent: accents.dusk } };

/** A strong, unrelated accent makes the effect easy to judge. */
export const WithContrastingAccent: Story = { args: { accent: accents.ember } };

export const WithoutAccent: Story = {};

/** No backdrop: the poster, blurred. */
export const PosterOnly: Story = {
  args: { url: undefined, posterUrl: art.posterMoss, accent: accents.moss },
};
