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

/** A dangerous action in a row or toolbar: red text without a fill; it asks before it acts. */
export const DangerQuiet: Story = {
  render: () => (
    <Stack direction="row" gap={2}>
      <Button variant="dangerQuiet">Löschen …</Button>
      <Button variant="dangerQuiet" icon="trash">
        Vergessen
      </Button>
      <Button variant="dangerQuiet" isDisabled>
        Gesperrt
      </Button>
    </Stack>
  ),
};

export const DangerQuietFocused: Story = {
  tags: ['press:Tab'],
  args: { variant: 'dangerQuiet', children: 'Löschen …' },
};

/** A small tool over a picture: the pen on a cover (concept 15 7.4), 32 px. */
export const Small: Story = {
  render: () => (
    <Stack direction="row" gap={2}>
      <Button variant="secondary" size="sm" icon="pen" iconOnly aria-label="Metadaten bearbeiten" />
      <Button variant="secondary" size="sm" icon="info">
        Details
      </Button>
    </Stack>
  ),
};

export const SmallFocused: Story = {
  tags: ['press:Tab'],
  args: {
    variant: 'secondary',
    size: 'sm',
    icon: 'pen',
    iconOnly: true,
    children: undefined,
    'aria-label': 'Metadaten bearbeiten',
  },
};

export const Block: Story = {
  args: { variant: 'primary', block: true, children: 'Weiter' },
};
