import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { AriaButton } from './AriaButton';
import { MessageDialog } from './MessageDialog';
import { Notice } from './Notice';

const meta = {
  title: 'UI/MessageDialog',
  component: MessageDialog,
  args: {
    isOpen: true,
    onDismiss: () => undefined,
    kicker: 'Ankündigung',
    title: 'Wartung am Samstag',
    meta: 'Veröffentlicht am 21.10.2026',
    body: 'Am Samstag von 8 bis 10 Uhr ist Slouchcat nicht erreichbar.\nDownloads spielen weiter.\n\nDanach gibt es die neuen Sammlungen.',
    dismissLabel: 'Später',
    confirmLabel: 'Verstanden',
    onConfirm: () => undefined,
  },
  decorators: [
    (Story) => (
      <>
        {/* The dialog sits in a portal; this keeps the story root from being empty. */}
        <AriaButton>Start</AriaButton>
        <Story />
      </>
    ),
  ],
} satisfies Meta<typeof MessageDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Focus sits on "Verstanden". */
export const Default: Story = {};

/** The second of three messages, the first confirm failed. */
export const WithStatusAndError: Story = {
  args: {
    status: '2 von 3',
    error: <Notice tone="danger">Der Server ist nicht erreichbar.</Notice>,
  },
};

/**
 * Opened by Enter on a link (a "virtual" click for react-aria): the focus still starts on
 * "Verstanden" (e2e-storybook/dialog-focus.spec.ts).
 */
function MessageFromLink(args: Parameters<typeof MessageDialog>[0]) {
  const [isOpen, setOpen] = useState(false);
  return (
    <>
      <a
        href="#message"
        onClick={(event) => {
          event.preventDefault();
          setOpen(true);
        }}
      >
        Ankündigung lesen
      </a>
      <MessageDialog
        {...args}
        isOpen={isOpen}
        onDismiss={() => setOpen(false)}
        onConfirm={() => setOpen(false)}
      />
    </>
  );
}

export const OpenedByLink: Story = { render: (args) => <MessageFromLink {...args} /> };
