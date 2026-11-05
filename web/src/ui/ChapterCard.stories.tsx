import type { Meta, StoryObj } from '@storybook/react-vite';
import { ChapterCard } from './ChapterCard';
import { art } from './stories/fixtures';

const noop = () => undefined;
const durationMs = 9_840_000;

const Row = ({ auto = false }: { auto?: boolean }) => (
  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 12rem)', gap: '1rem' }}>
    <ChapterCard
      index={0}
      title="Anfang"
      startMs={0}
      durationMs={durationMs}
      progress={100}
      auto={auto}
      onPress={noop}
    />
    <ChapterCard
      index={1}
      title="Die Farm des Replikanten am Rand der Stadt"
      startMs={600_000}
      durationMs={durationMs}
      progress={40}
      auto={auto}
      isCurrent
      onPress={noop}
    />
    <ChapterCard index={2} startMs={1_200_000} durationMs={durationMs} auto={auto} onPress={noop} />
    <ChapterCard
      index={3}
      title="Los Angeles"
      startMs={1_800_000}
      durationMs={durationMs}
      imageUrl={art.backdropDusk}
      auto={auto}
      onPress={noop}
    />
  </div>
);

const meta = {
  title: 'UI/ChapterCard',
  component: ChapterCard,
  args: { index: 0, startMs: 0, durationMs, onPress: noop },
  render: () => <Row />,
} satisfies Meta<typeof ChapterCard>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Until stage 3 the number stands in for the image; the playing chapter is outlined, with its
 * progress; the third has no title and is «Kapitel 3»; the fourth shows an image.
 */
export const Default: Story = {};

/** Replacement chapters every 10 minutes (F-6) carry «automatisch». */
export const Automatic: Story = { render: () => <Row auto /> };

/** Focused by keyboard. */
export const Focused: Story = { tags: ['press:Tab+Tab'] };

/**
 * Stage 3: every chapter with its image, the playing one outlined; a replacement chapter keeps
 * «automatisch» over its image, and an image that fails to load leaves the number.
 */
export const Images: Story = {
  render: () => (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 12rem)', gap: '1rem' }}>
      <ChapterCard
        index={0}
        title="Anfang"
        startMs={0}
        durationMs={durationMs}
        imageUrl={art.backdropDusk}
        progress={100}
        onPress={noop}
      />
      <ChapterCard
        index={1}
        title="Die Farm"
        startMs={600_000}
        durationMs={durationMs}
        imageUrl={art.backdropDusk}
        progress={40}
        isCurrent
        onPress={noop}
      />
      <ChapterCard
        index={2}
        startMs={1_200_000}
        durationMs={durationMs}
        imageUrl={art.backdropDusk}
        auto
        onPress={noop}
      />
      <ChapterCard
        index={3}
        title="Los Angeles"
        startMs={1_800_000}
        durationMs={durationMs}
        imageUrl="missing-chapter.webp"
        onPress={noop}
      />
    </div>
  ),
};
