import type { Meta, StoryObj } from '@storybook/react-vite';
import { Icon, type IconName } from './Icon';
import { Stack } from './Stack';

const names: IconName[] = [
  'play',
  'pause',
  'info',
  'back',
  'chevronLeft',
  'chevronRight',
  'fullscreen',
  'fullscreenExit',
  'volume',
  'volumeMute',
  'logout',
  'refresh',
  'plus',
  'warning',
  'folder',
  'lock',
  'check',
  'copy',
  'external',
  'download',
  'trash',
  'previous',
  'next',
  'replay',
  'forward',
  'list',
  'moon',
  'mic',
  'wrench',
  'users',
  'user',
  'monitor',
  'star',
  'stack',
  'plug',
  'bell',
  'megaphone',
  'chart',
  'archive',
  'spotlight',
  'grid',
  'film',
  'server',
  'power',
  'disk',
  'activity',
  'radio',
  'heart',
];

const meta = {
  title: 'UI/Icon',
  component: Icon,
  args: { name: 'play' },
} satisfies Meta<typeof Icon>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Every icon in the three sizes; the rows wrap, so all of them stay inside the screenshot. */
export const All: Story = {
  render: () => (
    <Stack gap={4}>
      {(['sm', 'md', 'lg'] as const).map((size) => (
        <div key={size} style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--sc-space-4)' }}>
          {names.map((name) => (
            <Icon key={name} name={name} size={size} />
          ))}
        </div>
      ))}
    </Stack>
  ),
};
