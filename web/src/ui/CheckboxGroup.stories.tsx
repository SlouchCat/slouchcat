import type { Meta, StoryObj } from '@storybook/react-vite';
import { CheckboxGroup } from './CheckboxGroup';

const meta = {
  title: 'UI/CheckboxGroup',
  component: CheckboxGroup,
  args: {
    label: 'Bibliotheken wählen',
    defaultValue: ['1'],
    options: [
      { value: '1', label: 'Filme' },
      { value: '2', label: 'Serien' },
      { value: '3', label: 'Kinder' },
    ],
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: 'var(--sc-size-form-max)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CheckboxGroup>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Tab, then Space toggles the focused box. */
export const Focused: Story = { tags: ['press:Tab+Tab+Space'] };

/** Options with a line below the label (the image kinds of the metadata page). */
export const WithDescriptions: Story = {
  args: {
    label: 'Bildarten',
    defaultValue: ['logo'],
    options: [
      {
        value: 'logo',
        label: 'Logo',
        description: 'Titelschriftzug auf Detailseite und Spotlight statt des Text-Titels',
      },
      { value: 'season_poster', label: 'Staffelposter', description: 'Sonst das Serienposter' },
    ],
  },
};

export const WithDescriptionsCompact: Story = {
  ...WithDescriptions,
  globals: { density: 'compact' },
};

/**
 * One choice locked (concept 25, the request dialog's seasons): a season that is already there
 * keeps a dashed box and says why; Tab passes it by.
 */
export const DisabledOption: Story = {
  tags: ['press:Tab'],
  args: {
    label: 'Staffeln',
    defaultValue: ['2'],
    options: [
      { value: '1', label: 'Staffel 1', description: '8 Folgen · verfügbar', isDisabled: true },
      { value: '2', label: 'Staffel 2', description: '10 Folgen' },
      { value: '3', label: 'Staffel 3', description: '10 Folgen · bereits angefordert' },
    ],
  },
};
