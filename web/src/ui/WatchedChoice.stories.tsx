import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { Button } from './Button';
import { WatchedChoice } from './WatchedChoice';

const meta = {
  title: 'UI/WatchedChoice',
  component: WatchedChoice,
  args: {
    isOpen: false,
    onOpenChange: () => undefined,
    title: '«Staffel 1» als ungesehen markieren?',
    body: 'Bei 8 Folgen fällt der Haken. Das lässt sich nicht rückgängig machen.',
    cancelLabel: 'Abbrechen',
    removeLabel: 'Alle Haken entfernen',
    onRemove: () => undefined,
  },
  render: function Render(args) {
    const [open, setOpen] = useState(false);
    return (
      <>
        <Button icon="check" onPress={() => setOpen(true)}>
          Gesehen
        </Button>
        <WatchedChoice {...args} isOpen={open} onOpenChange={setOpen} />
      </>
    );
  },
} satisfies Meta<typeof WatchedChoice>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

/** A full check of a season: the removal asks, «Abbrechen» has the focus. */
export const Remove: Story = { tags: ['press:Tab+Enter'] };

/** A half check: mark all (focused) or remove all. */
export const Mixed: Story = {
  tags: ['press:Tab+Enter'],
  args: {
    title: '«Nordlicht» ist teilweise gesehen',
    body: '19 von 96 Folgen tragen den Haken. Alle Folgen markieren oder alle Haken entfernen? Das Entfernen lässt sich nicht rückgängig machen.',
    markAllLabel: 'Alle Folgen als gesehen markieren',
    onMarkAll: () => undefined,
  },
};
