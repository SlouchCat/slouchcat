import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button } from './Button';
import { PosterCard } from './PosterCard';
import { accents, art } from './stories/fixtures';

const meta = {
  title: 'UI/PosterCard',
  component: PosterCard,
  args: {
    href: '#',
    title: 'Die Stunde der Dämmerung',
    caption: '2023',
    posterUrl: art.posterDusk,
    eager: true,
  },
  decorators: [
    (Story) => (
      <div style={{ width: 'var(--sc-size-poster-lg)', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PosterCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { args: { accent: accents.dusk } };

/** Focus: ring, scale (focus.scale) and a glow in the title's accent, all in 180 ms. */
export const FocusedWithAccent: Story = {
  tags: ['press:Tab'],
  args: { accent: accents.dusk },
};

/** A title without accent glows in the profile accent. */
export const FocusedWithoutAccent: Story = {
  tags: ['press:Tab'],
  args: { title: 'Glut', caption: '2019', posterUrl: art.posterEmber },
};

/** While the poster waits for the hero (P-10) the frame carries a tint of the accent. */
export const WaitingForImage: Story = {
  args: { title: 'Moos und Stein', caption: '2021', accent: accents.moss, deferImage: true },
};

export const WithProgress: Story = {
  args: {
    title: 'Glut',
    caption: 'Noch 1 h 12 min',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    progress: { percent: 36, label: '36 % gesehen' },
  },
};

/**
 * "Weiterschauen" per series (concept 15 P-1): the series poster and title, the episode's code and
 * time left below; the spoken name adds the episode's own title.
 */
export const ContinueEpisode: Story = {
  args: {
    title: 'Moos und Stein',
    caption: 'S4 · F7 · Noch 20 min',
    label: 'Moos und Stein, S4 · F7, Der lange Winter, Noch 20 min',
    posterUrl: art.posterMoss,
    accent: accents.moss,
    progress: { percent: 56, label: '56 % gesehen' },
  },
};

export const WithoutPoster: Story = {
  args: { title: 'Slouchcat Test H264 (2024)', caption: '2024', posterUrl: undefined },
};

/** An extra of a title: no poster, its kind as a label (B-7). */
export const WithBadge: Story = {
  args: { title: 'Making Of', caption: undefined, posterUrl: undefined, badge: 'Featurette' },
};

/**
 * An audiobook or a podcast: the cover is square (M6). It sits in the middle of the 2:3 frame of
 * every card, on a blurred and darkened copy of itself (concept 18 C-1, variant B).
 */
export const Square: Story = {
  args: {
    title: 'Die Stunde der Dämmerung',
    caption: 'Mara Lindqvist',
    shape: 'square',
    accent: accents.dusk,
    progress: { percent: 52, label: '52 % gehört' },
  },
};

/** The library page: a finished title carries a check mark; no age rating (concept 17 F-10). */
export const Watched: Story = {
  args: {
    title: 'Glut',
    caption: '2019',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    watchedLabel: 'Gesehen',
  },
};

/** A finished audiobook: square cover with the check mark. */
export const SquareHeard: Story = {
  args: {
    title: 'Die Stunde der Dämmerung',
    caption: 'Mara Lindqvist',
    shape: 'square',
    accent: accents.dusk,
    watchedLabel: 'Gehört',
  },
};

/**
 * The pen beside the card's link, over the top right of its cover (concept 17 D-7): card and pen
 * share a cell that carries the focus scale, so the card stays large while the pointer is on the
 * pen; the pen itself grows and lightens (tool.hover.scale). A finished title's check moves to the
 * bottom right.
 */
export const WithTool: Story = {
  args: {
    title: 'Glut',
    caption: '2019',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    watchedLabel: 'Gesehen',
    tool: (
      <Button
        variant="secondary"
        size="sm"
        icon="pen"
        iconOnly
        aria-label="Glut: Metadaten bearbeiten"
      />
    ),
  },
};

/** Tab after the card's link reaches the pen: the cell stays scaled once, the pen grows. */
export const WithToolFocused: Story = {
  ...WithTool,
  tags: ['press:Tab+Tab'],
};

/** The watchlist button of a title not marked yet, as `CoverWatchlist` draws it. */
const unmarked = (
  <Button
    variant="secondary"
    size="sm"
    icon="bookmark"
    iconOnly
    overlay
    aria-pressed={false}
    aria-label="Glut merken"
  />
);

/** The same button of a marked title: filled in the profile's accent. */
const marked = (
  <Button
    variant="secondary"
    size="sm"
    icon="bookmark"
    iconOnly
    overlay
    aria-pressed
    aria-label="Glut von der Merkliste nehmen"
  />
);

/**
 * The watchlist button over the bottom right of the cover (concept 17 D-9): a title not marked
 * shows it only while pointed at or focused, so at rest the card looks as without it.
 */
export const WithMark: Story = {
  args: {
    title: 'Glut',
    caption: '2019',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    mark: unmarked,
  },
};

/** Tab after the card's link reaches the button: it shows with its focus and grows. */
export const WithMarkFocused: Story = {
  ...WithMark,
  tags: ['press:Tab+Tab'],
};

/**
 * A marked title shows the filled button always; the check of a finished title moves to the top
 * left.
 */
export const WithMarkPressed: Story = {
  args: {
    title: 'Glut',
    caption: '2019',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    watchedLabel: 'Gesehen',
    mark: marked,
  },
};

/** A started title: the progress bar ends left of the button. */
export const WithMarkProgress: Story = {
  args: {
    title: 'Glut',
    caption: 'Noch 1 h 12 min',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    progress: { percent: 36, label: '36 % gesehen' },
    mark: marked,
  },
};

/** Pen at the top right, mark at the bottom right; Tab goes link, pen, mark. */
export const WithToolAndMark: Story = {
  args: {
    title: 'Glut',
    caption: '2019',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    watchedLabel: 'Gesehen',
    tool: (
      <Button
        variant="secondary"
        size="sm"
        icon="pen"
        iconOnly
        overlay
        aria-label="Glut: Metadaten bearbeiten"
      />
    ),
    mark: unmarked,
  },
};

/** The third Tab stop of the cell is the mark. */
export const WithToolAndMarkFocused: Story = {
  ...WithToolAndMark,
  tags: ['press:Tab+Tab+Tab'],
};

/** A square cover: the button sits on its bottom right corner, not under the title. */
export const SquareWithMark: Story = {
  args: {
    title: 'Die Stunde der Dämmerung',
    caption: 'Mara Lindqvist',
    shape: 'square',
    accent: accents.dusk,
    mark: marked,
  },
};

/**
 * Without `href` the card is a button (the spotlight row under the band, concept 08, F-5): the
 * chosen card carries a ring in the title's accent.
 */
export const ButtonActive: Story = {
  args: { href: undefined, onPress: () => undefined, active: true, accent: accents.dusk },
};

/** A card of that row that is not the chosen one: the poster at 70 %. */
export const ButtonDimmed: Story = {
  args: {
    href: undefined,
    onPress: () => undefined,
    dimmed: true,
    title: 'Glut',
    caption: '2019',
    posterUrl: art.posterEmber,
    accent: accents.ember,
  },
};

/** The chosen card with the band's time bar drawn inside its frame (a child of the card). */
export const ButtonWithTimeBar: Story = {
  args: {
    href: undefined,
    onPress: () => undefined,
    active: true,
    accent: accents.dusk,
    children: (
      <span
        style={{
          position: 'absolute',
          right: 0,
          bottom: 0,
          left: 0,
          width: '40%',
          height: 'var(--sc-space-1)',
          backgroundColor: 'var(--sc-color-text-primary)',
        }}
      />
    ),
  },
};

/**
 * A poster and a square cover side by side, as in a mixed row or grid: both frames are 2:3, so the
 * titles start on one line (concept 18 C-1).
 */
export const TitlesOnOneLine: Story = {
  render: (args) => (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
        gap: 'var(--sc-space-4)',
        alignItems: 'start',
      }}
    >
      <div>
        <PosterCard {...args} accent={accents.dusk} />
      </div>
      <div>
        <PosterCard
          {...args}
          title="Glut"
          caption="Mara Lindqvist"
          posterUrl={art.posterEmber}
          accent={accents.ember}
          shape="square"
        />
      </div>
    </div>
  ),
};
