import type { Meta, StoryObj } from '@storybook/react-vite';
import { PosterCard } from './PosterCard';
import { Row, RowList } from './Row';
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

/**
 * A row with a page of all its entries (concept 20 P-7): the title itself is the link, with a small
 * arrow after it.
 */
export const WithTitleLink: Story = { args: { title: 'Neu hinzugefügt', href: '#' } };

/** The title link focused by Tab: the first stop of the row, before the cards. */
export const TitleLinkFocused: Story = {
  args: { title: 'Neu hinzugefügt', href: '#' },
  tags: ['press:Tab'],
};

/** A separate link right of the title (the radio page's «Alle aus …»). */
export const WithMoreLink: Story = {
  args: { title: 'Beliebt in der Schweiz', moreHref: '#', moreLabel: 'Alle aus der Schweiz ›' },
};

/**
 * Fixed paging buttons beside the row (concept 20 P-1, `arrows="always"`: the season shelf): at the
 * start the left one is disabled.
 */
export const FixedArrows: Story = {
  render: () => (
    <RowList arrows="always" aria-label="Staffeln">
      {cards}
    </RowList>
  ),
};

/** Every card fits: no paging buttons at all. */
export const FixedArrowsAllFit: Story = {
  render: () => (
    <RowList arrows="always" aria-label="Staffeln">
      {cards.slice(0, 3)}
    </RowList>
  ),
};
