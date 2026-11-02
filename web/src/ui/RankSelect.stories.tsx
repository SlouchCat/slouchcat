import type { Meta, StoryObj } from '@storybook/react-vite';
import { useId, useState } from 'react';
import { RankHint, RankSelect, rankTo, type RankMode } from './RankSelect';
import styles from './stories/RankSelectStory.module.css';

/** The text languages of the metadata page: each line chooses its position (S-16). */
function Languages({ mode }: { mode: RankMode }) {
  const [order, setOrder] = useState(['Deutsch', 'Englisch', 'Französisch', 'Japanisch']);
  const hint = useId();
  return (
    <div className={styles.list}>
      <ol className={styles.rows}>
        {order.map((name, index) => (
          <li key={name} className={styles.row}>
            <span>{name}</span>
            <RankSelect
              name={name}
              position={index}
              count={order.length}
              mode={mode}
              describedBy={hint}
              onChange={(to) => setOrder(rankTo(order, index, to, mode))}
            />
          </li>
        ))}
      </ol>
      <RankHint id={hint} mode={mode} />
    </div>
  );
}

const meta = {
  title: 'UI/RankSelect',
  component: Languages,
  args: { mode: 'move' },
} satisfies Meta<typeof Languages>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Move: the others move along. One hint for the list, linked from every select. */
export const Move: Story = {};

/** Swap: the entry on the chosen position takes the old one. */
export const Swap: Story = { args: { mode: 'swap' } };

/** A single select with its own hint below it. */
export const Single: Story = {
  render: () => <RankSelect name="TMDB" position={0} count={3} onChange={() => undefined} />,
};

/** Focused with the keyboard. */
export const Focused: Story = { tags: ['press:Tab'] };

export const Compact: Story = { globals: { density: 'compact' } };
