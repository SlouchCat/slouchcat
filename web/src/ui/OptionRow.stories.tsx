import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { OptionList, OptionMoreRow, OptionRow, OptionSection } from './OptionRow';

const noop = () => undefined;

// The subtitles of a film: off, the profile's languages, a forced track and a picture subtitle.
const Subtitles = ({ selectedId }: { selectedId: string | null }) => (
  <OptionList label="Untertitel" selectedId={selectedId} onSelect={noop}>
    <OptionRow id="off" label="Aus" />
    <OptionRow id="de-forced" label="Deutsch" detail="SRT" marks={['forced']} />
    <OptionRow id="de" label="Deutsch" detail="SRT" marks={['default']} />
    <OptionRow id="en-sdh" label="Englisch" detail="SRT" marks={['sdh']} />
    <OptionRow id="en-pgs" label="Englisch" detail="PGS" marks={['burnIn']} />
    <OptionRow id="fr" label="Französisch" isDisabled />
  </OptionList>
);

// The profile's languages first, the others behind «Weitere Sprachen (n)» in the same list (11.10):
// ↑/↓ run across both groups, the more row is never chosen.
const others = ['Finnisch', 'Italienisch', 'Japanisch'];
const Grouped = ({ expanded = false }: { expanded?: boolean }) => {
  const [more, setMore] = useState(expanded);
  return (
    <OptionList label="Untertitel" selectedId="de" onSelect={noop} actionIds={['more']}>
      <OptionSection title="Deine Sprachen">
        <OptionRow id="off" label="Aus" />
        <OptionRow id="de" label="Deutsch" detail="SRT" marks={['default']} />
        <OptionRow id="en" label="Englisch" detail="SDH" />
      </OptionSection>
      <OptionSection title="Weitere Sprachen">
        <OptionMoreRow
          id="more"
          label={`Weitere Sprachen (${others.length})`}
          isExpanded={more}
          onToggle={() => setMore((shown) => !shown)}
        />
        {more &&
          others.map((language) => (
            <OptionRow key={language} id={language} label={language} detail="SRT" />
          ))}
      </OptionSection>
    </OptionList>
  );
};

const meta = {
  title: 'UI/OptionRow',
  component: OptionRow,
  args: { id: 'de', label: 'Deutsch' },
  render: () => <Subtitles selectedId="de" />,
  decorators: [
    (Story) => (
      <div style={{ width: '22rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof OptionRow>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The chosen row carries the tick and a bolder name; marks sit on the second line. */
export const Default: Story = {};

/** Tab focuses the chosen row; ↓ moves the ring to the next one without choosing it. */
export const Focused: Story = { tags: ['press:Tab+ArrowDown'] };

/** Nothing chosen yet: no tick anywhere. */
export const NoneChosen: Story = { render: () => <Subtitles selectedId={null} /> };

/** Two groups in one list; the other languages are folded behind the more row. */
export const Groups: Story = { render: () => <Grouped /> };

/** Unfolded, with the focus walked from the chosen row down across the group heading. */
export const GroupsExpanded: Story = {
  render: () => <Grouped expanded />,
  tags: ['press:Tab+ArrowDown+ArrowDown+ArrowDown'],
};
