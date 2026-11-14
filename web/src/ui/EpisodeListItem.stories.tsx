import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState, type ComponentProps } from 'react';
import { EpisodeListItem } from './EpisodeListItem';
import { accents, art } from './stories/fixtures';
import { WatchedToggle } from './WatchedToggle';

type Props = ComponentProps<typeof EpisodeListItem>;

/** The row opens and closes as on the page: a pointer toggles, Enter opens. */
function Toggling(props: Props) {
  const [expanded, setExpanded] = useState(props.expanded);
  return (
    <EpisodeListItem
      {...props}
      expanded={expanded}
      onToggle={(keyboard) => setExpanded((open) => keyboard || !open)}
    />
  );
}

const meta = {
  title: 'UI/EpisodeListItem',
  component: EpisodeListItem,
  args: {
    focusKey: 'story-episode',
    number: '2',
    code: 'S01E02',
    title: 'Die lange Nacht',
    overview:
      'Ein Stromausfall legt die Stadt lahm. Weber sucht im Dunkeln nach dem Zeugen, der als Einziger weiss, wer in der Nacht am Hafen war, und findet mehr, als ihr lieb ist.',
    caption: '45 min',
    expanded: false,
    onToggle: () => undefined,
    playHref: '#',
    playLabel: 'Abspielen',
    playName: 'Abspielen: S01E02 · Die lange Nacht',
    detailsHref: '#',
    detailsLabel: 'Details',
    detailsName: 'Details: S01E02 · Die lange Nacht',
    stillUrl: art.backdropDusk,
    accent: accents.dusk,
  },
  render: (args) => <Toggling {...args} />,
  decorators: [
    (Story) => (
      <div style={{ padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof EpisodeListItem>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The closed line: number, title, one line of text (from 700 px), runtime, the empty check. */
export const Default: Story = {};

export const Watched: Story = { args: { watchedLabel: 'Gesehen' } };

/** The open row: still, code, longer text, «Abspielen» and «Details». */
export const Open: Story = { args: { expanded: true } };

/** The episode the big button plays, open as the page opens, started: «Weiterschauen». */
export const NextUp: Story = {
  args: {
    expanded: true,
    highlight: 'Weiterschauen',
    playLabel: 'Weiterschauen',
    playName: 'Weiterschauen: S01E02 · Die lange Nacht',
    progress: { percent: 42, label: '42 % gesehen' },
  },
};

/** No still and no overview yet (no metadata match), a double episode. */
export const Bare: Story = {
  args: {
    expanded: true,
    number: '3–4',
    code: 'S01E03–E04',
    title: 'Episode 3',
    stillUrl: undefined,
    overview: undefined,
  },
};

/** Focus on the line (Tab). */
export const Focused: Story = { tags: ['press:Tab'] };

/** Enter on the line opens the row and moves the focus to «Abspielen» (concept 21 O-11). */
export const EnterOpens: Story = { tags: ['press:Tab+Enter'] };

/** The check as a button (concept 20 P-5): an empty ring that sets «gesehen». */
export const WithToggle: Story = {
  args: {
    watchedControl: (
      <WatchedToggle
        appearance="row"
        state="none"
        label="Gesehen: S01E02 · Die lange Nacht"
        onPress={() => undefined}
      />
    ),
  },
};

/** The same button set: filled. */
export const WithToggleSet: Story = {
  args: {
    watchedControl: (
      <WatchedToggle
        appearance="row"
        state="full"
        label="Gesehen: S01E02 · Die lange Nacht"
        onPress={() => undefined}
      />
    ),
  },
};

/** Tab after the line reaches the check: line, check. */
export const ToggleFocused: Story = { ...WithToggle, tags: ['press:Tab+Tab'] };
