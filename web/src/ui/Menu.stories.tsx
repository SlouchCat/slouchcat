import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { MenuButton } from './Menu';

const versions = [
  { id: 'mf-1', label: '1080p' },
  { id: 'mf-2', label: '720p' },
];

function Versions() {
  const [selected, setSelected] = useState('mf-1');
  return (
    <div style={{ paddingTop: '8rem' }}>
      <MenuButton
        buttonLabel="Version wählen"
        label={versions.find((version) => version.id === selected)?.label ?? ''}
        options={versions}
        selectedKey={selected}
        onSelect={setSelected}
      />
    </div>
  );
}

const qualities = [
  { id: 'auto', label: 'Automatisch' },
  { id: '1080p', label: '1080p', disabled: true },
  { id: '720p', label: '720p' },
  { id: '480p', label: '480p' },
  { id: '360p', label: '360p' },
];

/** Rungs the limits rule out stay visible but cannot be picked (W-8). */
function Qualities() {
  const [selected, setSelected] = useState('auto');
  return (
    <div style={{ paddingTop: '16rem' }}>
      <MenuButton
        buttonLabel="Qualität wählen"
        label={qualities.find((quality) => quality.id === selected)?.label ?? ''}
        options={qualities}
        selectedKey={selected}
        onSelect={setSelected}
      />
    </div>
  );
}

/** Two choices in one menu: an audio track and a subtitle track (W-4). */
function Tracks() {
  const [audio, setAudio] = useState('audio:1');
  const [subtitle, setSubtitle] = useState('subtitle:off');
  return (
    <div style={{ paddingTop: '18rem' }}>
      <MenuButton
        buttonLabel="Spuren"
        label="Spuren"
        groups={[
          {
            id: 'audio',
            label: 'Ton',
            selectedKey: audio,
            options: [
              { id: 'audio:1', label: 'Deutsch · AAC 2.0' },
              { id: 'audio:2', label: 'Englisch · AC3 5.1' },
            ],
          },
          {
            id: 'subtitle',
            label: 'Untertitel',
            selectedKey: subtitle,
            options: [
              { id: 'subtitle:off', label: 'Aus' },
              { id: 'subtitle:7', label: 'Deutsch (SRT)' },
              { id: 'subtitle:8', label: 'Englisch · PGS' },
            ],
          },
        ]}
        onSelect={(id, group) => (group === 'audio' ? setAudio(id) : setSubtitle(id))}
      />
    </div>
  );
}

const meta = {
  title: 'UI/MenuButton',
  component: MenuButton,
  args: {
    buttonLabel: 'Version wählen',
    label: '1080p',
    options: versions,
    selectedKey: 'mf-1',
    onSelect: () => undefined,
  },
  render: () => <Versions />,
} satisfies Meta<typeof MenuButton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

/** Opened from the keyboard: the current choice has the tick and the focus. */
export const Open: Story = { tags: ['press:Tab+Enter'] };

/** A disabled rung is skipped by the arrow keys. */
export const WithDisabled: Story = { render: () => <Qualities />, tags: ['press:Tab+Enter'] };

/** Groups with a title and a choice each. */
export const Groups: Story = { render: () => <Tracks />, tags: ['press:Tab+Enter'] };
