import type { Meta, StoryObj } from '@storybook/react-vite';
import { PosterCard } from './PosterCard';
import { RequestMark } from './RequestMark';
import { accents, art } from './stories/fixtures';

const meta = {
  title: 'UI/RequestMark',
  component: RequestMark,
  args: { label: 'Die Stunde der Dämmerung anfordern', onPress: () => undefined },
  decorators: [
    (Story) => (
      <div style={{ width: 'var(--sc-size-poster-lg)', padding: 'var(--sc-space-6)' }}>
        <PosterCard
          href="#"
          title="Die Stunde der Dämmerung"
          caption="2026 · Noch nicht da"
          posterUrl={art.posterDusk}
          accent={accents.dusk}
          eager
          mark={<Story />}
        />
      </div>
    ),
  ],
} satisfies Meta<typeof RequestMark>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The plus on the poster of a missing title (shown on hover, on focus and without hover). */
export const Request: Story = {};

/** Tab: first the card's link, then the plus. */
export const Focused: Story = { tags: ['press:Tab+Tab'] };

/** Another profile's open wish: the bell joins it. */
export const Join: Story = { args: { label: 'Benachrichtige mich: Glut', icon: 'bell' } };

/** Offline the mark stays, locked. */
export const Offline: Story = { args: { isDisabled: true } };
