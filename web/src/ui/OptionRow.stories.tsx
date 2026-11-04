import type { Meta, StoryObj } from '@storybook/react-vite';
import { OptionList, OptionRow } from './OptionRow';

const noop = () => undefined;

// The subtitles of a film: off, the profile's languages, a forced track and a picture subtitle.
const Subtitles = ({ selectedId }: { selectedId: string | null }) => (
  <OptionList label="Untertitel" selectedId={selectedId} onSelect={noop}>
    <OptionRow id="off" label="Aus" />
    <OptionRow id="de-forced" label="Deutsch" detail="SRT" marks={['forced']} />
    <OptionRow id="de" label="Deutsch" detail="SRT" marks={['default']} />
    <OptionRow id="en-sdh" label="Englisch" detail="SDH" />
    <OptionRow id="en-pgs" label="Englisch" detail="Bild-Untertitel (PGS)" marks={['burnIn']} />
    <OptionRow id="fr" label="Französisch" isDisabled />
  </OptionList>
);

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
