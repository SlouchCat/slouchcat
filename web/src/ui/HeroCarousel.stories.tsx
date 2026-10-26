import type { Meta, StoryObj } from '@storybook/react-vite';
import { Chip } from './Chip';
import { HeroCarousel, type HeroSlide } from './HeroCarousel';
import { art } from './stories/fixtures';

const slides: HeroSlide[] = [
  {
    id: 'dusk',
    title: 'Die Stunde der Dämmerung',
    backdropUrl: art.backdropDusk,
    posterUrl: art.posterDusk,
    meta: (
      <>
        <Chip tone="outline">FSK 12</Chip>
        <span>2023 · 1 h 52 min</span>
      </>
    ),
    hasTrailer: true,
    playHref: '#play',
    detailsHref: '#details',
  },
  {
    id: 'ember',
    title: 'Glut',
    posterUrl: art.posterEmber,
    hasTrailer: true,
    playHref: '#play',
    detailsHref: '#details',
  },
  {
    id: 'moss',
    title: 'Moos und Stein',
    posterUrl: art.posterMoss,
    playHref: '#play',
    detailsHref: '#details',
  },
];

const meta = {
  title: 'UI/HeroCarousel',
  component: HeroCarousel,
  parameters: { layout: 'fullscreen' },
  args: {
    label: "Cat's Empfehlungen",
    slides,
    playLabel: 'Abspielen',
    detailsLabel: 'Details',
    trailerLabel: 'Trailer',
    onTrailerPress: () => undefined,
  },
} satisfies Meta<typeof HeroCarousel>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The framed band with the strip of two posters under it; the first slide is active. Its title has a
 * trailer, so the trailer button stands after details (the third slide has none).
 */
export const Default: Story = { args: { slides: slides.slice(0, 2) } };

/** Play focused: the primary button fills with the profile accent. */
export const Focused: Story = { args: { slides: slides.slice(0, 2) }, tags: ['press:Tab'] };

/** A title with a poster only: the poster, blurred, stands in for the backdrop; one slide, no strip. */
export const PosterOnly: Story = { args: { slides: [slides[1] as HeroSlide] } };

/** A pick with the main user's note (E-2): a line under the title. */
export const WithNote: Story = {
  args: {
    slides: [
      { ...(slides[0] as HeroSlide), note: 'Unbedingt im Dunkeln schauen, mit Ton laut.' },
      slides[1] as HeroSlide,
    ],
  },
};

/**
 * The poster strip with three slides, reached by Tab past the three buttons: the active slide's
 * poster carries the accent edge and the focus ring, the others are dimmed.
 */
export const PosterStrip: Story = { tags: ['press:Tab+Tab+Tab+Tab'] };
