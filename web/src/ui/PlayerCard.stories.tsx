import type { Meta, StoryObj } from '@storybook/react-vite';
import { PlayerCard } from './PlayerCard';
import { art } from './stories/fixtures';

const noop = () => undefined;

const Episodes = () => (
  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 14rem)', gap: '1rem' }}>
    <PlayerCard
      eyebrow="S01E01"
      title="Freiheitstag"
      caption="52 Min."
      imageUrl={art.backdropDusk}
      progress={{ percent: 100, label: '100 % gesehen' }}
      onPress={noop}
    />
    <PlayerCard
      eyebrow="S01E02"
      title="Die lange Nacht unter der Erde"
      caption="48 Min."
      progress={{ percent: 40, label: '40 % gesehen' }}
      isCurrent
      onPress={noop}
    />
    <PlayerCard eyebrow="S01E03" title="Maschinen" caption="55 Min." onPress={noop} />
  </div>
);

const Cast = () => (
  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 10rem)', gap: '1rem' }}>
    <PlayerCard shape="person" title="Rebecca Ferguson" caption="als Juliette" onPress={noop} />
    <PlayerCard shape="person" title="Tim Robbins" caption="als Bernard" onPress={noop} />
    <PlayerCard shape="person" title="Morten Tyldum" caption="Regie" onPress={noop} />
  </div>
);

const meta = {
  title: 'UI/PlayerCard',
  component: PlayerCard,
  args: { title: 'Freiheitstag', onPress: noop },
  render: () => <Episodes />,
} satisfies Meta<typeof PlayerCard>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * «Als Nächstes»: episodes with still, code, title and progress; the playing one is outlined, the
 * third has no still yet.
 */
export const Default: Story = {};

/** «Besetzung»: the initial, the name and the role. */
export const Person: Story = { render: () => <Cast /> };

/** Focused by keyboard. */
export const Focused: Story = { tags: ['press:Tab'] };
