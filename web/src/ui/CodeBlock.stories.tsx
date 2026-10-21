import type { Meta, StoryObj } from '@storybook/react-vite';
import { CodeBlock } from './CodeBlock';

const meta = {
  title: 'UI/CodeBlock',
  component: CodeBlock,
  args: {
    code: 'docker compose -f compose.yaml -f compose.nvidia.yaml up -d',
    label: 'Compose-Befehl für NVIDIA',
    copyLabel: 'Kopieren',
    copiedMessage: 'Befehl kopiert.',
    copyFailedMessage: 'Kopieren nicht möglich. Der Befehl ist markiert, kopiere ihn mit Strg+C.',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '32rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CodeBlock>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** The sysctl command for a low inotify limit. */
export const Sysctl: Story = {
  args: {
    code: 'sysctl fs.inotify.max_user_watches=524288',
    label: 'Befehl für das inotify-Limit',
  },
};

/** A long command wraps inside a narrow column instead of scrolling. */
export const Narrow: Story = {
  decorators: [
    (Story) => (
      <div style={{ width: '18rem' }}>
        <Story />
      </div>
    ),
  ],
};

/** Tab reaches the copy button. */
export const CopyFocused: Story = { tags: ['press:Tab'] };
