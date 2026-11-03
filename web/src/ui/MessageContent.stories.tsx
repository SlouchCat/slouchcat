import type { Meta, StoryObj } from '@storybook/react-vite';
import { MessageContent } from './MessageDialog';

/**
 * The text of a MessageDialog without the dialog and its buttons, as the preview «So sehen es die
 * Profile» of the announcements page shows it (concept 12, 11.3). With `preview` the title is
 * plain text, not the dialog's heading.
 */
const meta = {
  title: 'UI/MessageContent',
  component: MessageContent,
  args: {
    kicker: 'Ankündigung',
    title: 'Wartung am Samstag',
    meta: 'Veröffentlicht am 21.10.2026',
    body: 'Am Samstag von 8 bis 10 Uhr ist Slouchcat nicht erreichbar.\nDownloads spielen weiter.\n\nDanach gibt es die neuen Sammlungen.',
    preview: true,
  },
  decorators: [
    (Story) => (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', width: '28rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof MessageContent>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The preview of an announcement. */
export const Preview: Story = {};
