import type { Decorator, Meta, StoryObj } from '@storybook/react-vite';
import { CastCard } from './CastCard';
import { Row } from './Row';

const narrow: Decorator = (Story) => (
  <div style={{ width: 'var(--sc-size-poster-md)', padding: 'var(--sc-space-6)' }}>
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

export const Focused: Story = { decorators: [narrow], tags: ['press:Tab'] };

/** The cast row of a detail page. */
export const InRow: Story = {
  parameters: { layout: 'fullscreen' },
  render: () => (
    <div style={{ paddingTop: 'var(--sc-space-6)' }}>
      <Row title="Besetzung">
        <CastCard href="#" name="Sigourney Weaver" caption="als Ellen Ripley" />
        <CastCard href="#" name="Tom Skerritt" caption="als Dallas" />
        <CastCard href="#" name="Ridley Scott" caption="Regie" />
      </Row>
    </div>
  ),
};
