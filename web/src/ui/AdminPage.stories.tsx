import type { Meta, StoryObj } from '@storybook/react-vite';
import { AdminFrame } from './adminFrame';
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

/** Today's density: the h1 in the display size, as on /settings/* and on touch devices. */
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

const frame = {
  eyebrow: 'Automatisierung / Hintergrundjobs',
  tabs: {
    label: 'Seiten im Bereich Automatisierung',
    tabs: [
      {
        href: '#jobs',
        label: 'Hintergrundjobs',
        current: true,
        alarm: { value: '265', tone: 'danger' as const },
        accessibleLabel: 'Hintergrundjobs, 265 fehlgeschlagen',
      },
      {
        href: '#integrations',
        label: 'Integrationen',
        count: '3 / 8',
        accessibleLabel: 'Integrationen, 3 von 8 eingerichtet',
      },
      { href: '#notifications', label: 'Benachrichtigungen' },
    ],
  },
};

/**
 * Inside the administration's frame (build plan 2.5, 2.6): the area's tabs, the line over the
 * title, the large title and the intro sentence open below it; no help button.
 */
export const Framed: Story = {
  decorators: [
    (Story) => (
      <AdminFrame value={frame}>
        <Story />
      </AdminFrame>
    ),
  ],
};

/** The frame in the compact density of the administration. */
export const FramedCompact: Story = { ...Framed, globals: { density: 'compact' } };

/**
 * A long intro in the frame, compact: two lines with an ellipsis and «Mehr» beside them, so the
 * page's first working line stays where it is.
 */
export const FramedLongIntro: Story = {
  ...Framed,
  globals: { density: 'compact' },
  args: {
    help: 'Alles, was der Server im Hintergrund tut: laufende Aufgaben, die Warteschlange, die Zeitpläne der wiederkehrenden Jobs und die Fehlschläge der letzten Tage, mit Neustart, Pause und dem Protokoll jedes einzelnen Laufs, damit du siehst, was gerade passiert und was später kommt.',
  },
};
