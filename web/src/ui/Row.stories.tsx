import type { Meta, StoryObj } from '@storybook/react-vite';
import { PosterCard } from './PosterCard';
import { Row } from './Row';
import { accents, titles } from './stories/fixtures';

const accentOf = { dusk: accents.dusk, ember: accents.ember, moss: accents.moss } as const;

const cards = [...titles, ...titles].map((item, index) => (
  <PosterCard
    key={`${item.id}-${index}`}
    href="#"
    title={item.title}
    caption={String(item.year)}
    posterUrl={item.poster}
    accent={accentOf[item.id]}
    eager
  />
));

const meta = {
  title: 'UI/Row',
  component: Row,
  parameters: { layout: 'fullscreen' },
  args: { title: 'Neu hinzugefügt', children: cards },
  decorators: [
    (Story) => (
      <div style={{ paddingTop: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Row>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** First card focused by Tab (the paging buttons are pointer-only). */
export const Focused: Story = { tags: ['press:Tab'] };

/** A row with a page of all its entries: "Alle anzeigen ›" right of the title (concept 08). */
export const WithMoreLink: Story = {
  args: { title: "Cat's Empfehlungen", moreHref: '#', moreLabel: 'Alle anzeigen ›' },
};
