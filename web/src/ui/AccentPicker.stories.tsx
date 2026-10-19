import type { Meta, StoryObj } from '@storybook/react-vite';
import { AccentPicker } from './AccentPicker';

const options = [
  { value: 'ember', label: 'Glut' },
  { value: 'coral', label: 'Koralle' },
  { value: 'amber', label: 'Bernstein' },
  { value: 'lime', label: 'Limette' },
  { value: 'teal', label: 'Türkis' },
  { value: 'sky', label: 'Himmel' },
  { value: 'violet', label: 'Violett' },
  { value: 'rose', label: 'Rosa' },
];

const meta = {
  title: 'UI/AccentPicker',
  component: AccentPicker,
  args: { label: 'Farbe', options, defaultValue: 'teal' },
} satisfies Meta<typeof AccentPicker>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Tab lands on the selected swatch; arrow keys move between swatches. */
export const Focused: Story = { tags: ['press:Tab+ArrowRight'] };
