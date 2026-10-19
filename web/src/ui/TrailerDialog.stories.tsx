import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { AriaButton } from './AriaButton';
import { TrailerDialog } from './TrailerDialog';

// The screenshots must not load YouTube (no network in the baseline run): the stories show the
// dialog with its status slot instead of a frame.
function Demo({ status }: { status: string }) {
  const [language, setLanguage] = useState('de');
  return (
    <>
      {/* The dialog sits in a portal; this keeps the story root from being empty. */}
      <AriaButton icon="play">Trailer</AriaButton>
      <TrailerDialog
        isOpen
        onOpenChange={() => undefined}
        title="Trailer: Die Stunde der Dämmerung"
        closeLabel="Schliessen"
        languageLabel="Sprache"
        languages={[
          { value: 'de', label: 'Deutsch' },
          { value: 'en', label: 'English' },
        ]}
        language={language}
        onLanguageChange={setLanguage}
        frameTitle="Trailer von Die Stunde der Dämmerung (YouTube)"
        note="Der Trailer wird von YouTube abgespielt (youtube-nocookie.com)."
      >
        {status}
      </TrailerDialog>
    </>
  );
}

const meta = {
  title: 'UI/TrailerDialog',
  component: TrailerDialog,
  args: {
    isOpen: true,
    onOpenChange: () => undefined,
    title: 'Trailer',
    closeLabel: 'Schliessen',
    languageLabel: 'Sprache',
    languages: [],
    language: 'de',
    onLanguageChange: () => undefined,
    frameTitle: 'Trailer',
  },
  render: () => <Demo status="Wird geladen …" />,
} satisfies Meta<typeof TrailerDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {};

/** Tab from the dialog reaches the close button, then the language choice. */
export const Focused: Story = { tags: ['press:Tab+Tab'] };
