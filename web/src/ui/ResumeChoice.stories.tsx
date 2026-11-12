import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { Button } from './Button';
import { ResumeChoice } from './ResumeChoice';

const meta = {
  title: 'UI/ResumeChoice',
  component: ResumeChoice,
  args: {
    isOpen: false,
    onOpenChange: () => undefined,
    title: 'S01E04 · Die lange Nacht',
    body: 'Du hast diese Folge angefangen.',
    resumeLabel: 'Weiterschauen ab 21:36',
    restartLabel: 'Von vorn starten',
    cancelLabel: 'Abbrechen',
    onResume: () => undefined,
    onRestart: () => undefined,
  },
  render: function Render(args) {
    const [open, setOpen] = useState(false);
    return (
      <>
        <Button icon="play" onPress={() => setOpen(true)}>
          S01E04 abspielen
        </Button>
        <ResumeChoice {...args} isOpen={open} onOpenChange={setOpen} />
      </>
    );
  },
} satisfies Meta<typeof ResumeChoice>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

/** Opened from the keyboard: the focus sits on «Weiterschauen ab …». */
export const Open: Story = { tags: ['press:Tab+Enter'] };

/** A podcast episode: «Weiterhören ab …». */
export const Listening: Story = {
  tags: ['press:Tab+Enter'],
  args: {
    title: 'Folge 12: Ein langer Abend',
    body: 'Du hast diese Episode angefangen.',
    resumeLabel: 'Weiterhören ab 8:05',
  },
};
