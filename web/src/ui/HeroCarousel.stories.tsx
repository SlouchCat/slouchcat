import type { Meta, StoryObj } from '@storybook/react-vite';
import { Chip } from './Chip';
import { HeroCarousel, type HeroSlide } from './HeroCarousel';
import { accents, art } from './stories/fixtures';

const slides: HeroSlide[] = [
  {
    id: 'dusk',
    title: 'Die Stunde der Dämmerung',
    backdropUrl: art.backdropDusk,
    posterUrl: art.posterDusk,
    accent: accents.dusk,
    caption: '2023',
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
    accent: accents.ember,
    caption: '2019',
    hasTrailer: true,
    playHref: '#play',
    detailsHref: '#details',
  },
  {
    id: 'moss',
    title: 'Moos und Stein',
    posterUrl: art.posterMoss,
    accent: accents.moss,
    caption: '2021',
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
    rowLabel: 'Spotlight-Titel',
    pauseLabel: 'Pause',
    resumeLabel: 'Weiter',
    onTrailerPress: () => undefined,
  },
} satisfies Meta<typeof HeroCarousel>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The framed band with the row of two posters under it, in the normal card size; the first slide
 * is active (ring in its accent, the other poster dimmed). Its title has a trailer, so the trailer
 * button stands after details (the third slide has none). Position and pause button at the top
 * right. Screenshots run with reduced motion, so the time bar is not shown.
 */
export const Default: Story = { args: { slides: slides.slice(0, 2) } };

/** Play focused: the primary button fills with the profile accent. */
export const Focused: Story = { args: { slides: slides.slice(0, 2) }, tags: ['press:Tab'] };

/** A title with a poster only: the poster, blurred, stands in for the backdrop; one slide, no row. */
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
 * The poster row with three slides, reached by Tab past the three buttons and the pause button:
 * the active slide's poster carries the accent ring and the focus ring, the others are dimmed.
 */
export const PosterStrip: Story = { tags: ['press:Tab+Tab+Tab+Tab+Tab'] };

/**
 * The band began with the server's draw (`start`, concept 08, F-2): the second slide is active, its
 * poster stands at the row's left edge, and the position reads «2 von 3».
 */
export const ActivePoster: Story = { args: { start: 1 } };

/** Held, here by an open trailer (`paused`): the band shows `data-paused` and no time bar. */
export const Paused: Story = { args: { paused: true, start: 2 } };

/**
 * In a phone-wide column. The card widths follow the viewport's media queries (three cards per row
 * only below 768 px), which the screenshot run at 1280 px does not reach; Storybook's viewport
 * toolbar shows the real phone layout.
 */
export const Phone: Story = {
  decorators: [
    (Story) => (
      <div style={{ width: '24.375rem' }}>
        <Story />
      </div>
    ),
  ],
};
