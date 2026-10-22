import type { Meta, StoryObj } from '@storybook/react-vite';
import { AdminNav } from './AdminNav';
import { Button } from './Button';
import { Page, PageNav } from './Page';
import { Panel } from './Panel';
import { Text } from './Text';

const meta = {
  title: 'UI/Page',
  component: Page,
  parameters: { layout: 'fullscreen' },
  args: { children: null },
} satisfies Meta<typeof Page>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Centred column for setup and login. */
export const Form: Story = {
  args: {
    layout: 'form',
    children: (
      <Panel>
        <Text as="h1" variant="display">
          Anmelden
        </Text>
        <Text tone="secondary">Mit dem Konto des Servers.</Text>
        <Button variant="primary" block>
          Weiter
        </Button>
      </Panel>
    ),
  },
};

/** Page under the app header (admin pages). */
export const Content: Story = {
  args: {
    layout: 'content',
    children: (
      <Text as="h1" variant="display">
        Bibliotheken
      </Text>
    ),
  },
};

/** An admin page: the section navigation (PageNav) above the heading. */
export const WithSectionNav: Story = {
  args: {
    layout: 'content',
    children: (
      <Text as="h1" variant="display">
        Bibliotheken
      </Text>
    ),
  },
  decorators: [
    (Story) => (
      <PageNav
        nav={
          <AdminNav
            label="Verwaltung"
            groups={[
              { links: [{ href: '#admin', label: 'Übersicht' }] },
              {
                label: 'Bibliothek',
                links: [
                  { href: '#libraries', label: 'Bibliotheken', current: true },
                  { href: '#podcasts', label: 'Podcasts' },
                ],
              },
            ]}
          />
        }
      >
        <Story />
      </PageNav>
    ),
  ],
};
