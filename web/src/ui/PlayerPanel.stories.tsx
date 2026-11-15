import type { Meta, StoryObj } from '@storybook/react-vite';
import { useRef } from 'react';
import { AriaButton } from './AriaButton';
import { OptionList, OptionRow } from './OptionRow';
import { PlayerPanel, PlayerPanelAnchor } from './PlayerPanel';

const noop = () => undefined;

const languages = [
  'Arabisch',
  'Dänisch',
  'Finnisch',
  'Griechisch',
  'Italienisch',
  'Japanisch',
  'Koreanisch',
  'Niederländisch',
  'Norwegisch',
  'Polnisch',
  'Portugiesisch',
  'Schwedisch',
  'Spanisch',
  'Tschechisch',
  'Türkisch',
];

const meta = {
  title: 'UI/PlayerPanel',
  component: PlayerPanel,
  parameters: { layout: 'fullscreen' },
  args: {
    title: 'Ton',
    onClose: noop,
    children: (
      <OptionList label="Ton" selectedId="de" onSelect={noop}>
        <OptionRow id="de" label="Deutsch" detail="DTS 5.1" marks={['default']} />
        <OptionRow id="en" label="Englisch" detail="TrueHD 7.1 · Originalton" />
        <OptionRow id="en-ad" label="Englisch" detail="AC3 2.0 · Audiodeskription" />
      </OptionList>
    ),
  },
  // The player: without a button to stand above, the panel sits at its bottom right.
  decorators: [
    (Story) => (
      <div
        style={{
          position: 'relative',
          height: '100vh',
          backgroundColor: 'var(--sc-color-media-backdrop)',
        }}
      >
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PlayerPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Opened, the focus lies on the chosen row. */
export const Audio: Story = {};

/** A long list scrolls inside the panel; the chosen row far down has the focus and is in view. */
export const LongList: Story = {
  args: {
    title: 'Untertitel',
    children: (
      <OptionList label="Untertitel" selectedId="Türkisch" onSelect={noop}>
        <OptionRow id="off" label="Aus" />
        {languages.map((language) => (
          <OptionRow key={language} id={language} label={language} detail="SRT" />
        ))}
      </OptionList>
    ),
  },
};

/** A page inside the gear panel: a back button in the head, Backspace goes up a level. */
export const SubPage: Story = {
  args: {
    title: 'Qualität',
    onBack: noop,
    children: (
      <OptionList label="Qualität" selectedId="1080" onSelect={noop}>
        <OptionRow id="auto" label="Automatisch" detail="Zurzeit 1080p" />
        <OptionRow id="original" label="Original" detail="HEVC · 24 Mbit/s" />
        <OptionRow id="1080" label="1080p" detail="8 Mbit/s" />
        <OptionRow id="720" label="720p" detail="4 Mbit/s" />
      </OptionList>
    ),
  },
};

/**
 * The player's bar button: the menu opens right above it, right aligned, as tall as its content
 * (concept 23 P-2). No close button shows; the hidden one is last in the panel.
 */
function AnchoredMenu() {
  const anchor = useRef<HTMLButtonElement>(null);
  return (
    <>
      <div
        style={{ position: 'absolute', right: 'var(--sc-space-16)', bottom: 'var(--sc-space-6)' }}
      >
        <AriaButton ref={anchor} variant="ghost" icon="audioTrack" aria-expanded>
          Deutsch
        </AriaButton>
      </div>
      <PlayerPanelAnchor value={anchor}>
        <PlayerPanel title="Ton" onClose={noop}>
          <OptionList label="Ton" selectedId="de" onSelect={noop}>
            <OptionRow id="de" label="Deutsch" detail="DTS 5.1" marks={['default']} />
            <OptionRow id="en" label="Englisch" detail="TrueHD 7.1 · Originalton" />
          </OptionList>
        </PlayerPanel>
      </PlayerPanelAnchor>
    </>
  );
}

export const AboveItsButton: Story = {
  render: () => <AnchoredMenu />,
};
