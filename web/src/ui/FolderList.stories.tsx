import type { LibraryKind, MetadataProviderInfo } from '@slouchcat/api-client';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { FolderList, newFolderEntry, type FolderEntry } from './FolderList';

const infos: MetadataProviderInfo[] = [
  {
    id: 'tmdb',
    configured: true,
    roles: ['identify', 'fill'],
    contents: ['movie', 'series'],
    languages: ['de', 'en'],
  },
  {
    id: 'tvdb',
    configured: true,
    roles: ['identify', 'fill'],
    contents: ['series'],
    languages: ['de', 'en'],
  },
  {
    id: 'anilist',
    configured: true,
    roles: ['fill'],
    contents: ['movie', 'series'],
    languages: ['en'],
  },
  {
    id: 'local',
    configured: true,
    roles: ['identify'],
    contents: ['audiobook', 'podcast'],
    languages: [],
  },
];

function Editable(props: { kind: LibraryKind; min: number; paths: string[]; stored?: boolean }) {
  const [entries, setEntries] = useState<FolderEntry[]>(() =>
    props.paths.reduce<FolderEntry[]>(
      (list, path) => [...list, { ...newFolderEntry(props.kind, list), path }],
      [],
    ),
  );
  // «Bearbeiten»: the stored folders start closed, the first one is marked for removal.
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const [marked, setMarked] = useState<ReadonlySet<string>>(
    () => new Set(entries.slice(0, 1).map((entry) => entry.key)),
  );
  if (!props.stored) {
    return (
      <FolderList
        kind={props.kind}
        entries={entries}
        onChange={setEntries}
        infos={infos}
        min={props.min}
        namePrefix="roots"
      />
    );
  }
  return (
    <FolderList
      kind={props.kind}
      entries={entries}
      onChange={setEntries}
      infos={infos}
      min={0}
      namePrefix="roots"
      expandedKeys={expanded}
      onExpandedChange={(key, open) =>
        setExpanded((current) => (open ? new Set([...current, key]) : new Set()))
      }
      marking={{
        isStored: () => true,
        marked,
        onMark: (key) => setMarked(new Set([...marked, key])),
        onUndo: (key) => setMarked(new Set([...marked].filter((entry) => entry !== key))),
      }}
    />
  );
}

const meta = {
  title: 'UI/FolderList',
  component: Editable,
  args: { kind: 'video', min: 1, paths: ['/media/nas/Doku/Filme', '/media/nas/Doku/Serien'] },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: 'var(--sc-size-form-max)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Editable>;

export default meta;
type Story = StoryObj<typeof meta>;

/** «Dokus»: a film folder and a series folder, two numbered cards. */
export const Mixed: Story = {};

/** The form of a new video library: one card, which the list keeps. */
export const One: Story = { args: { paths: ['/media/nas/Filme'] } };

/** A podcast library may stay without folders; the dialog «Ordner von X» starts empty as well. */
export const Empty: Story = { args: { kind: 'podcasts', min: 0, paths: [] } };

/** «Bibliothek bearbeiten»: every stored folder closed to one line, the first marked for removal. */
export const Stored: Story = {
  args: { paths: ['/media/nas/Filme', '/media/nas/Serien', '/media/nas/Doku'], stored: true },
};

/** Tab reaches the head of the first card. */
export const Focused: Story = { args: { paths: ['/media/nas/Filme'] }, tags: ['press:Tab'] };
