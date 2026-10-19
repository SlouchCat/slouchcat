import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button } from './Button';
import { Notice } from './Notice';
import { Stack } from './Stack';

const meta = {
  title: 'UI/Notice',
  component: Notice,
  args: { title: 'Titel nicht gefunden' },
} satisfies Meta<typeof Notice>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Tones: Story = {
  render: () => (
    <Stack gap={4}>
      <Notice title="Titel nicht gefunden">
        Dieser Titel ist nicht oder nicht mehr in der Bibliothek.
      </Notice>
      <Notice tone="warning" title="Datei fehlt">
        Die Bibliothekswurzel ist nicht erreichbar; nichts wird gelöscht.
      </Notice>
      <Notice
        tone="danger"
        title="Server nicht erreichbar"
        actions={
          <Button variant="primary" icon="refresh">
            Erneut versuchen
          </Button>
        }
      >
        Die Verbindung ist abgebrochen.
      </Notice>
    </Stack>
  ),
};
