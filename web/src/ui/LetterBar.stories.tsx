import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { LetterBar } from './LetterBar';
import { SpatialPage } from './spatial';

const letters = ['#', ...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'];
const available = ['#', 'A', 'B', 'D', 'G', 'H', 'M', 'S', 'T', 'Z'];

function Letters({ initial }: { initial?: string }) {
  // The list's jump puts the chosen letter's titles in view: the bar then marks it.
  const [current, setCurrent] = useState(initial);
  return (
    <SpatialPage defaultFocusKey="letters-#">
      <div style={{ height: '40rem' }}>
        <LetterBar
          label="Anfangsbuchstabe"
          letters={letters}
          available={available}
          current={current}
          onJump={setCurrent}
          letterLabels={{ '#': 'Ziffer oder Zeichen' }}
          focusKey="letters"
        />
      </div>
    </SpatialPage>
  );
}

const meta = {
  title: 'UI/LetterBar',
  component: LetterBar,
  args: {
    label: 'Anfangsbuchstabe',
    letters,
    available,
    current: undefined,
    onJump: () => undefined,
  },
  render: () => <Letters initial="#" />,
} satisfies Meta<typeof LetterBar>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Letters without titles are disabled; the titles in view start with "#". */
export const Default: Story = {};

/** The titles in view start with "S" (after a jump or a scroll). */
export const Chosen: Story = { render: () => <Letters initial="S" /> };

/** Focused by Tab: the ring on "#". */
export const Focused: Story = { tags: ['press:Tab'] };
