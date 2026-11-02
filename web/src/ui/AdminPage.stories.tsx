import type { Meta, StoryObj } from '@storybook/react-vite';
import { AdminPage } from './AdminPage';
import { AriaButton } from './AriaButton';
import { DataTable, type DataColumn } from './DataTable';
import { StatusStrip } from './StatusStrip';

interface Job {
  id: string;
  name: string;
  state: string;
}

const jobs: Job[] = [
  { id: 'scan', name: 'Bibliotheksscan', state: 'Bereit' },
  { id: 'fill', name: 'Metadaten auffüllen', state: '12 wartend' },
];

const columns: DataColumn<Job>[] = [
  { key: 'name', header: 'Jobart', rowHeader: true, cell: (row) => row.name },
  { key: 'state', header: 'Zustand', cell: (row) => row.state },
];

// The frame of an administration page (concept 12, S-2 to S-4): header in one line, status
// strip, content. Page renders its own <main> with the app header's padding.
const meta = {
  title: 'UI/AdminPage',
  component: AdminPage,
  parameters: { layout: 'fullscreen' },
  args: {
    title: 'Hintergrundjobs',
    help: 'Alles, was der Server im Hintergrund tut: laufende Aufgaben, Warteschlange, Zeitpläne und Fehlschläge.',
    liveSeconds: 5,
    actions: <AriaButton icon="refresh">Alle erneut versuchen</AriaButton>,
    status: (
      <StatusStrip
        label="Zusammenfassung"
        tiles={[
          { key: 'running', label: 'Laufend', value: '1', tone: 'success' },
          { key: 'queued', label: 'Wartend', value: '12' },
          { key: 'planned', label: 'Geplant', value: '6' },
          { key: 'paused', label: 'Pausiert', value: '0' },
          { key: 'failed', label: 'Fehlgeschlagen', value: '265', tone: 'danger' },
        ]}
      />
    ),
    children: (
      <DataTable caption="Jobarten" columns={columns} rows={jobs} rowKey={(row) => row.id} />
    ),
  },
} satisfies Meta<typeof AdminPage>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Today's density: the h1 in the display size, as on every page until K12-8. */
export const Default: Story = {};

/** The help button opened with the keyboard: the intro sentence below the header. */
export const HelpOpen: Story = { tags: ['press:Tab+Enter'] };

/** Compact density: 24 px title, 32 px controls, 24 px between the sections. */
export const Compact: Story = { globals: { density: 'compact' } };

/** Compact, help open. */
export const CompactHelpOpen: Story = {
  globals: { density: 'compact' },
  tags: ['press:Tab+Enter'],
};
