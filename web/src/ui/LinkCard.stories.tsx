import type { Meta, StoryObj } from '@storybook/react-vite';
import { LinkCard } from './LinkCard';

const meta = {
  title: 'UI/LinkCard',
  component: LinkCard,
  args: {
    href: '#integrations',
    title: 'Integrationen',
    description: 'Schlüssel, E-Mail, Telegram, Web-Push und Update-Prüfung.',
    icon: 'plug',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '20rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof LinkCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Tab gives the card the focus ring and the lighter surface. */
export const Focused: Story = { tags: ['press:Tab'] };

export const WithoutDescription: Story = { args: { description: undefined, icon: 'archive' } };
