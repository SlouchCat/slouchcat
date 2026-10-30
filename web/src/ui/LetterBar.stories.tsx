import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { LetterBar } from './LetterBar';
import { SpatialPage } from './spatial';

const letters = ['#', ...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'];
const available = ['#', 'A', 'B', 'D', 'G', 'H', 'M', 'S', 'T', 'Z'];

function Letters({ initial }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return (
    <SpatialPage defaultFocusKey="letters-all">
      <div style={{ height: '40rem' }}>
        <LetterBar
          label="Anfangsbuchstabe"
          letters={letters}
          available={available}
          value={value}
          onChange={setValue}
          allMark="•"
          allLabel="Alle Buchstaben"
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
    value: undefined,
    onChange: () => undefined,
    allMark: '•',
    allLabel: 'Alle Buchstaben',
  },
  render: () => <Letters />,
} satisfies Meta<typeof LetterBar>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Letters without titles are disabled; "•" (all) is chosen. */
export const Default: Story = {};

/** A chosen letter. */
export const Chosen: Story = { render: () => <Letters initial="S" /> };

/** Focused by Tab: the ring on "•". */
export const Focused: Story = { tags: ['press:Tab'] };
