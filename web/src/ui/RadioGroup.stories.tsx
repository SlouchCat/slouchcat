import type { Meta, StoryObj } from '@storybook/react-vite';
import { RadioGroup } from './RadioGroup';
import { SettingRow } from './SettingRow';

const meta = {
  title: 'UI/RadioGroup',
  component: RadioGroup,
  args: {
    label: 'Sprache',
    defaultValue: 'de',
    options: [
      { value: 'de', label: 'Deutsch' },
      { value: 'en', label: 'English' },
    ],
  },
} satisfies Meta<typeof RadioGroup>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Arrow keys move focus and selection together (focus duration). */
export const Focused: Story = { tags: ['press:Tab+ArrowRight'] };

/**
 * In a SettingRow the row shows the label on the left; `hideLabel` keeps it as the group's name
 * for screen readers only, so it does not show twice.
 */
const InSettingRow = () => (
  <div style={{ width: '48rem' }}>
    <SettingRow label="Medienart" help="Die Liste mischt Filme und Serien.">
      <RadioGroup
        name="media"
        label="Medienart"
        hideLabel
        defaultValue="both"
        options={[
          { value: 'movie', label: 'Nur Filme' },
          { value: 'show', label: 'Nur Serien' },
          { value: 'both', label: 'Filme und Serien' },
        ]}
      />
    </SettingRow>
  </div>
);

export const HiddenLabel: Story = { render: InSettingRow };

export const HiddenLabelCompact: Story = { render: InSettingRow, globals: { density: 'compact' } };
