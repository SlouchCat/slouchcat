import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button, ButtonLink } from './Button';
import { Stack } from './Stack';

const meta = {
  title: 'UI/Button',
  component: Button,
  args: { children: 'Abspielen' },
} satisfies Meta<typeof Button>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Variants: Story = {
  render: () => (
    <Stack gap={4}>
      <Stack direction="row" gap={2}>
        <Button variant="primary" icon="play">
          Abspielen
        </Button>
        <Button variant="secondary" icon="info">
          Details
        </Button>
        <Button variant="ghost" icon="back">
          Zurück
        </Button>
        <Button variant="danger">Entfernen</Button>
      </Stack>
      <Stack direction="row" gap={2}>
        <Button variant="primary" size="lg" icon="play">
          Gross
        </Button>
        <Button variant="secondary" icon="refresh" iconOnly aria-label="Neu laden" />
        <Button variant="primary" isDisabled>
          Gesperrt
        </Button>
        <ButtonLink href="#" variant="secondary" icon="folder">
          Als Link
        </ButtonLink>
      </Stack>
    </Stack>
  ),
};

/** Keyboard focus: ring plus the profile accent as fill (focus duration, 180 ms). */
export const PrimaryFocused: Story = {
  tags: ['press:Tab'],
  args: { variant: 'primary', size: 'lg', icon: 'play' },
};

export const SecondaryFocused: Story = {
  tags: ['press:Tab'],
  args: { variant: 'secondary', icon: 'info', children: 'Details' },
};

export const GhostFocused: Story = {
  tags: ['press:Tab'],
  args: { variant: 'ghost', icon: 'back', children: 'Zurück' },
};

export const Block: Story = {
  args: { variant: 'primary', block: true, children: 'Weiter' },
};
