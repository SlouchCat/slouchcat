import type { Meta, StoryObj } from '@storybook/react-vite';
import { AriaButton } from './AriaButton';
import { EditorDialog } from './EditorDialog';
import { Tabs } from './Tabs';
import { TextLink } from './TextLink';

const meta = {
  title: 'UI/EditorDialog',
  component: EditorDialog,
  args: {
    title: 'Nebelküste',
    subtitle: 'Film · Filme',
    closeLabel: 'Schliessen',
    onClose: () => undefined,
    children: null,
  },
} satisfies Meta<typeof EditorDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The metadata editor's frame with its four tabs (concept 15 7.4); no picture yet. */
export const WithTabs: Story = {
  render: (args) => (
    <>
      {/* The dialog sits in a portal; this keeps the story root from being empty. */}
      <AriaButton icon="pen" iconOnly aria-label="Metadaten bearbeiten" />
      <EditorDialog
        {...args}
        link={<TextLink href="/admin/items/1">In der Verwaltung öffnen</TextLink>}
      >
        <Tabs
          label="Bereiche"
          tabs={[
            { id: 'images', label: 'Bilder', content: <p>Bildarten und Vorschläge.</p> },
            { id: 'metadata', label: 'Metadaten', content: <p>Felder mit Schloss.</p> },
            { id: 'match', label: 'Zuordnen', content: <p>Suche bei TMDB oder TVDB.</p> },
            { id: 'refresh', label: 'Aktualisieren', content: <p>Jetzt aktualisieren.</p> },
          ]}
        />
      </EditorDialog>
    </>
  ),
};

/** An episode: the still lies. */
export const Landscape: Story = {
  args: { title: 'Pilot', subtitle: 'Folge · Serien', imageShape: 'landscape' },
  render: (args) => (
    <>
      <AriaButton icon="pen" iconOnly aria-label="Metadaten bearbeiten" />
      <EditorDialog {...args}>
        <p>Inhalt</p>
      </EditorDialog>
    </>
  ),
};
