import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { RankedChoiceList, type RankedChoice } from './RankedChoiceList';

const languages: RankedChoice[] = [
  { value: 'de', label: 'Deutsch' },
  { value: 'en', label: 'Englisch' },
  { value: 'fr', label: 'Französisch' },
  { value: 'ja', label: 'Japanisch' },
];

const imageChoices: RankedChoice[] = [{ value: '', label: 'Ohne Text' }, ...languages];

/** The metadata languages: one select per place, a pick that sits elsewhere swaps (10.2). */
function Places({
  choices,
  initial,
  isDisabled,
}: {
  choices: RankedChoice[];
  initial: string[];
  isDisabled?: boolean;
}) {
  const [value, setValue] = useState(initial);
  return (
    <div style={{ width: '24rem' }}>
      <RankedChoiceList
        choices={choices}
        value={value}
        onChange={setValue}
        rankLabel={(index) => `${index + 1}. Sprache`}
        addLabel="Weitere Sprache"
        isAllowed={(list) => list.includes('de')}
        isDisabled={isDisabled}
      />
    </div>
  );
}

const meta = {
  title: 'UI/RankedChoiceList',
  component: Places,
  args: { choices: languages, initial: ['de', 'en', 'fr'] },
} satisfies Meta<typeof Places>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Three places; the list must keep Deutsch (a rule of the page): its «×» is off. */
export const Default: Story = {};

/** The image languages with «Ohne Text» as a choice. */
export const ImageLanguages: Story = {
  args: { choices: imageChoices, initial: ['de', 'en', ''] },
};

/** Read-only (a library that follows the default): no «×», no «Weitere Sprache». */
export const Disabled: Story = { args: { isDisabled: true } };

/**
 * ↓ on the closed select of place 1 picks Englisch, which held place 2: the two swap, the
 * status line names them and the select keeps the focus.
 */
export const SwappedByKeyboard: Story = { tags: ['press:Tab+ArrowDown'] };

export const Compact: Story = { globals: { density: 'compact' } };
