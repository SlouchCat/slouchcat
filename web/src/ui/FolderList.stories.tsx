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

function Editable(props: { kind: LibraryKind; min: number; paths: string[] }) {
  const [entries, setEntries] = useState<FolderEntry[]>(() =>
    props.paths.reduce<FolderEntry[]>(
      (list, path) => [...list, { ...newFolderEntry(props.kind, list), path }],
      [],
    ),
  );
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

/** Tab reaches the path of the first card. */
export const Focused: Story = { args: { paths: ['/media/nas/Filme'] }, tags: ['press:Tab'] };
