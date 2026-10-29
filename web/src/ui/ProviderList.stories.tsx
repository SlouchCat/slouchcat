import type { MetadataProvider, MetadataProviderInfo } from '@slouchcat/api-client';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { ProviderList } from './ProviderList';

/** A server with a TMDB key and without TVDB key. */
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
    configured: false,
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

function Editable(props: {
  content: 'movie' | 'series' | 'audiobook';
  initial: MetadataProvider[];
  infos?: MetadataProviderInfo[];
}) {
  const [value, setValue] = useState(props.initial);
  return (
    <ProviderList content={props.content} value={value} onChange={setValue} infos={props.infos} />
  );
}

const meta = {
  title: 'UI/ProviderList',
  component: Editable,
  args: { content: 'series', initial: ['tvdb', 'tmdb', 'anilist'], infos },
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

/** «Anime Serien»: TVDB identifies (without key, the hint says so), TMDB and AniList fill in. */
export const Series: Story = {};

/** A film folder: only TMDB may identify, AniList is a filler. */
export const Movie: Story = { args: { content: 'movie', initial: ['tmdb'] } };

/** A filler put in front: the list says what to do before the server answers 422. */
export const CannotIdentify: Story = { args: { content: 'movie', initial: ['anilist', 'tmdb'] } };

/** Audiobooks have one source today; nothing to tick. */
export const Fixed: Story = { args: { content: 'audiobook', initial: ['local'] } };

/** The setup wizard has no session to ask the server: the default list as text. */
export const WithoutServerAnswer: Story = {
  args: { content: 'series', initial: ['tmdb', 'tvdb'], infos: undefined },
};

/** Tab to the first checkbox, Tab twice more to «… nach oben» of the second row. */
export const Focused: Story = { tags: ['press:Tab+Tab+Tab+Tab'] };
