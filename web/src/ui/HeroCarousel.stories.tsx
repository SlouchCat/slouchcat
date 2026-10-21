import type { Meta, StoryObj } from '@storybook/react-vite';
import { Chip } from './Chip';
import { HeroCarousel, type HeroSlide } from './HeroCarousel';
import { art } from './stories/fixtures';

const slides: HeroSlide[] = [
  {
    id: 'dusk',
    title: 'Die Stunde der Dämmerung',
    backdropUrl: art.backdropDusk,
    meta: (
      <>
        <Chip tone="outline">FSK 12</Chip>
        <span>2023 · 1 h 52 min</span>
      </>
    ),
    playHref: '#play',
    detailsHref: '#details',
  },
  {
    id: 'ember',
    title: 'Glut',
    posterUrl: art.posterEmber,
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
  },
} satisfies Meta<typeof HeroCarousel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Play focused: the primary button fills with the profile accent. */
export const Focused: Story = { tags: ['press:Tab'] };

/** A title with a poster only: the poster, blurred, stands in for the backdrop. */
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
