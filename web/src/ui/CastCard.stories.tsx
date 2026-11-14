import type { Decorator, Meta, StoryObj } from '@storybook/react-vite';
import { CastCard } from './CastCard';
import { Row } from './Row';
import { art } from './stories/fixtures';

const narrow: Decorator = (Story) => (
  <div style={{ width: 'var(--sc-size-portrait-md)', padding: 'var(--sc-space-6)' }}>
    <Story />
  </div>
);

const meta = {
  title: 'UI/CastCard',
  component: CastCard,
  args: { href: '#', name: 'Sigourney Weaver', caption: 'als Ellen Ripley' },
} satisfies Meta<typeof CastCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { decorators: [narrow] };

/** With a portrait; the focused card keeps the ring and scale. */
export const WithPortrait: Story = { decorators: [narrow], args: { imageUrl: art.posterDusk } };

export const Focused: Story = { decorators: [narrow], tags: ['press:Tab'] };

/** A long role stops after two lines. */
export const LongRole: Story = {
  decorators: [narrow],
  args: { imageUrl: art.posterDusk, caption: 'als Ellen Ripley, Warrant Officer der Nostromo' },
};

/** The cast row of a detail page: 2:3 portraits, one without a picture (concept 21 P-7). */
export const InRow: Story = {
  parameters: { layout: 'fullscreen' },
  render: () => (
    <div style={{ paddingTop: 'var(--sc-space-6)' }}>
      <Row title="Besetzung" variant="people">
        <CastCard
          href="#"
          name="Sigourney Weaver"
          caption="als Ellen Ripley"
          imageUrl={art.posterMoss}
        />
        <CastCard href="#" name="Tom Skerritt" caption="als Dallas" />
        <CastCard href="#" name="Ridley Scott" caption="Regie" />
      </Row>
    </div>
  ),
};
