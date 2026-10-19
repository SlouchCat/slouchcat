import type { Meta, StoryObj } from '@storybook/react-vite';
import { ProfileAvatar, ProfileTile } from './ProfileTile';
import { Stack } from './Stack';

const meta = {
  title: 'UI/ProfileTile',
  component: ProfileTile,
  args: {
    name: 'Marco',
    accent: 'teal',
    hasPin: false,
    isActive: false,
    lockedLabel: 'mit PIN geschützt',
    onPress: () => undefined,
  },
} satisfies Meta<typeof ProfileTile>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** A lock sits on the avatar; screen readers hear "mit PIN geschützt". */
export const WithPin: Story = { args: { name: 'Lena', accent: 'rose', hasPin: true } };

/** The profile of the running session: a ring in its own colour, the primary profile named. */
export const ActivePrimary: Story = { args: { isActive: true, caption: 'Hauptprofil · aktiv' } };

/** First Tab stop: focus ring and the lifted tile (focus scale). */
export const Focused: Story = { tags: ['press:Tab'], args: { name: 'Noah', accent: 'sky' } };

/** The picker's row: every accent keeps its contrast on the dark base. */
export const Row: Story = {
  render: () => (
    <Stack direction="row" gap={4}>
      <ProfileTile
        {...meta.args}
        name="Marco"
        accent="teal"
        isActive
        caption="Hauptprofil · aktiv"
      />
      <ProfileTile {...meta.args} name="Lena" accent="rose" hasPin />
      <ProfileTile {...meta.args} name="Noah" accent="sky" />
      <ProfileTile {...meta.args} name="Mia" accent="amber" />
    </Stack>
  ),
};

export const Avatars: Story = {
  render: () => (
    <Stack direction="row" gap={4}>
      {['ember', 'coral', 'amber', 'lime', 'teal', 'sky', 'violet', 'rose'].map((accent) => (
        <ProfileAvatar key={accent} name={accent} accent={accent} />
      ))}
    </Stack>
  ),
};
