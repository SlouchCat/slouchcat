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
];

const meta = {
  title: 'UI/Icon',
  component: Icon,
  args: { name: 'play' },
} satisfies Meta<typeof Icon>;

export default meta;
type Story = StoryObj<typeof meta>;

export const All: Story = {
  render: () => (
    <Stack gap={4}>
      {(['sm', 'md', 'lg'] as const).map((size) => (
        <Stack key={size} direction="row" gap={4}>
          {names.map((name) => (
            <Icon key={name} name={name} size={size} />
          ))}
        </Stack>
      ))}
    </Stack>
  ),
};
