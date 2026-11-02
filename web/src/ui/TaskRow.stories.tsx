import type { Meta, StoryObj } from '@storybook/react-vite';
import { TaskRow } from './TaskRow';

// A running background job on the overview's card «Aufgaben» (concept 11, 11.1).

const meta = {
  title: 'UI/TaskRow',
  component: TaskRow,
  args: {
    type: 'Keyframe-Index',
    subject: 'Oppenheimer (2023)',
    progress: 30,
    progressLabel: 'Fortschritt Keyframe-Index',
    progressText: '12 / 40',
    remaining: 'noch etwa 3 min',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '32rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof TaskRow>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Job type and subject, progress with its figure, the remaining time. */
export const Default: Story = {};

/** A job without subject and without an estimate yet. */
export const Minimal: Story = {
  args: {
    type: 'Bibliotheksscan',
    subject: undefined,
    progressText: '8 %',
    progress: 8,
    remaining: undefined,
  },
};
