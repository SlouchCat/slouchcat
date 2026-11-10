import type { Meta, StoryObj } from '@storybook/react-vite';
import { ImageCandidateGrid, type ImageCandidateTile } from './ImageCandidateGrid';
import { art } from './stories/fixtures';

const posters: ImageCandidateTile[] = [
  {
    id: 'a',
    previewUrl: art.posterDusk,
    label: 'Deutsch',
    detail: '1000 × 1500 · TMDB · Bewertung 5,5',
    current: true,
    actionLabel: 'Deutsch wählen',
  },
  {
    id: 'b',
    previewUrl: art.posterEmber,
    label: 'Englisch',
    detail: '2000 × 3000 · TMDB · Bewertung 6',
    actionLabel: 'Englisch wählen',
  },
  {
    id: 'c',
    previewUrl: art.posterMoss,
    label: 'Ohne Text',
    detail: '600 × 900 · Lokale Datei',
    actionLabel: 'Ohne Text wählen',
  },
];

const meta = {
  title: 'UI/ImageCandidateGrid',
  component: ImageCandidateGrid,
  args: {
    label: 'Kandidaten für Poster',
    items: posters,
    emptyText: 'Keine Kandidaten.',
    currentText: 'aktuell',
    onAction: () => undefined,
  },
} satisfies Meta<typeof ImageCandidateGrid>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Posters stand; the current one carries its mark. */
export const Posters: Story = {};

/** Backdrops and logos lie. */
export const Landscape: Story = {
  args: {
    label: 'Kandidaten für Hintergrund',
    shape: 'landscape',
    items: [
      {
        id: 'd',
        previewUrl: art.backdropDusk,
        label: 'Ohne Text',
        detail: '1280 × 720 · TMDB',
        actionLabel: 'Ohne Text wählen',
      },
    ],
  },
};

/** No candidate at all. */
export const Empty: Story = { args: { items: [] } };

/** The covers of an audiobook are square: a folder image, the embedded picture (concept 19 D5.2). */
export const Square: Story = {
  args: {
    label: 'Kandidaten für Cover',
    shape: 'square',
    items: [
      {
        id: 'cover',
        previewUrl: art.posterDusk,
        label: 'cover.jpg',
        detail: '600 × 600 · Lokale Datei',
        current: true,
        actionLabel: 'cover.jpg wählen',
      },
      {
        id: 'embedded',
        previewUrl: art.posterEmber,
        label: 'Eingebettetes Bild',
        detail: '500 × 500 · Eingebettet',
        actionLabel: 'Eingebettetes Bild wählen',
      },
    ],
  },
};
