import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { ToggleToolbar } from './ToggleToolbar';

// The kind filter of the streams page (concept 18, 17.3): one pressed button, counts beside.

const options = [
  { value: 'all', label: 'Alle', count: '6' },
  { value: 'video', label: 'Video', count: '2' },
  { value: 'audiobook', label: 'Hörbücher', count: '1' },
  { value: 'podcast', label: 'Podcasts', count: '2' },
  { value: 'radio', label: 'Radio', count: '1' },
];

const meta = {
  title: 'UI/ToggleToolbar',
  component: ToggleToolbar,
  args: { label: 'Nach Art filtern', options, value: 'all', onChange: () => {} },
  render: function Render(args) {
    const [value, setValue] = useState(args.value);
    return <ToggleToolbar {...args} value={value} onChange={setValue} />;
  },
} satisfies Meta<typeof ToggleToolbar>;

export default meta;
type Story = StoryObj<typeof meta>;

/** «Alle» pressed. */
export const Default: Story = {};

/** Another kind pressed; a count the page does not know (a failed request) shows «–». */
export const Unknown: Story = {
  args: {
    value: 'podcast',
    options: options.map((option) =>
      option.value === 'radio' || option.value === 'all' ? { ...option, count: '–' } : option,
    ),
  },
};

/** In the administration's compact density (ADR 0020). */
export const Compact: Story = { globals: { density: 'compact' } };
