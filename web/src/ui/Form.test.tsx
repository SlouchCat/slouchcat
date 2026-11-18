import { act, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useRef, useState, type ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { AriaButton } from './AriaButton';
import { Form } from './Form';
import { RadioGroup } from './RadioGroup';
import { TextField } from './TextField';

/**
 * A form as every page builds it: the answer of the last submit is turned into messages on every
 * render, so the object handed to `validationErrors` is a new one each time. That is what kept
 * the message under a corrected field (Marco's finding of 23 October 2026: setting up the mail
 * server made valid input look rejected).
 *
 * `answer` is what the mutation reports: `null` while the request runs, otherwise the field
 * messages of the server's answer.
 */
function SmtpForm({
  answer,
  onSend,
}: {
  answer: Record<string, string> | null;
  onSend?: (host: string) => void;
}) {
  const [host, setHost] = useState('smtp.falsch');
  return (
    <Form
      validationErrors={answer === null ? undefined : { ...answer }}
      onSubmit={(event) => {
        event.preventDefault();
        onSend?.(host);
      }}
    >
      <TextField name="host" label="Server" value={host} onChange={setHost} />
      <TextField name="port" label="Port" defaultValue="587" />
      <RadioGroup
        name="tls"
        label="Verschlüsselung"
        defaultValue="starttls"
        options={[
          { value: 'starttls', label: 'STARTTLS' },
          { value: 'none', label: 'Keine' },
        ]}
      />
      <AriaButton type="submit">Speichern</AriaButton>
    </Form>
  );
}

const message = 'Dieser Wert ist ungültig.';
const required = 'Pflichtfeld.';

/**
 * A form whose messages never change, as a page holds them between two answers; `check` is a
 * button that only asks the form whether it is valid, as the announcement page does before it
 * publishes.
 */
function FixedForm({
  errors,
  sent,
  children,
}: {
  errors: Record<string, string>;
  sent: string[];
  children?: ReactNode;
}) {
  const form = useRef<HTMLFormElement>(null);
  return (
    <Form
      ref={form}
      validationErrors={{ ...errors }}
      onSubmit={(event) => {
        event.preventDefault();
        sent.push('sent');
      }}
    >
      <TextField name="host" label="Server" defaultValue="smtp.falsch" />
      {children}
      <AriaButton onPress={() => form.current?.reportValidity()}>Prüfen</AriaButton>
      <AriaButton type="submit">Speichern</AriaButton>
    </Form>
  );
}

/** Lets the retry's timers run: it needs a few tasks, none of them waits. */
const settle = () => new Promise((resolve) => setTimeout(resolve, 50));

/** Runs the pending timers one by one, each with its render, as the browser would. */
function runTimers() {
  for (let step = 0; step < 10; step++) {
    act(() => {
      vi.advanceTimersToNextTimer();
    });
  }
}

describe('Form', () => {
  it('keeps every character the reader types', async () => {
    // The listener that marks a field must not re-render while the input event is still being
    // handled: react-aria has not written the new value into the page's state yet, so the render
    // puts the old value back and the character is gone. The Playwright suite caught exactly that
    // -- the login page received "2e-owner@…" for "e2e-owner@…" -- and no unit test saw it,
    // because typing a single character still leaves the message assertions right.
    const user = userEvent.setup();
    render(<SmtpForm answer={{ host: message }} />);

    const host = screen.getByLabelText('Server');
    await user.clear(host);
    await user.type(host, 'smtp.beispiel.ch');

    expect(host).toHaveValue('smtp.beispiel.ch');
  });

  it('drops the server message of a field while it is corrected', async () => {
    const user = userEvent.setup();
    render(<SmtpForm answer={{ host: message }} />);
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.type(screen.getByLabelText('Server'), 'x');

    expect(screen.queryByText(message)).not.toBeInTheDocument();
  });

  it('keeps the message of a field nobody touched', async () => {
    const user = userEvent.setup();
    render(<SmtpForm answer={{ host: message, port: required }} />);
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.type(screen.getByLabelText('Server'), 'x');

    expect(screen.queryByText(message)).not.toBeInTheDocument();
    expect(screen.getByText(required)).toBeInTheDocument();
  });

  it('drops the message of a choice the reader changes', async () => {
    // A control that is not a text input. It still goes through the `input` listener here --
    // jsdom fires `input` for a radio too -- so this does not exercise the `change` path; that one
    // is the belt and braces for a control that reports only when it is done.
    const user = userEvent.setup();
    render(<SmtpForm answer={{ tls: message, host: required }} />);
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.click(screen.getByRole('radio', { name: 'Keine' }));

    expect(screen.queryByText(message)).not.toBeInTheDocument();
    expect(screen.getByText(required)).toBeInTheDocument();
  });

  it('shows an answer that arrives after the correction', async () => {
    // The setup wizard checks a single field on its own: no submit, the answer just arrives.
    const user = userEvent.setup();
    const { rerender } = render(<SmtpForm answer={null} />);

    await user.type(screen.getByLabelText('Server'), 'x');
    rerender(<SmtpForm answer={{ host: message }} />);

    expect(await screen.findByText(message)).toBeInTheDocument();
  });

  it('shows the message again when the next answer repeats it', async () => {
    const user = userEvent.setup();
    const { rerender } = render(<SmtpForm answer={{ host: message }} />);
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.type(screen.getByLabelText('Server'), 'x');
    expect(screen.queryByText(message)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    rerender(<SmtpForm answer={null} />);
    rerender(<SmtpForm answer={{ host: message }} />);

    expect(await screen.findByText(message)).toBeInTheDocument();
  });

  it('sends the same value again when the reader retries after a refusal', async () => {
    // The server's message stands in the field's native validity, so the browser stopped the
    // next submit by itself: a name refused because another profile had it could not be sent
    // again after that profile was renamed, and Enter right after typing raced the mark that
    // drops the message (the unstable profile-tag test of 18 November 2026).
    const user = userEvent.setup();
    const sent: string[] = [];
    render(<SmtpForm answer={{ host: message }} onSend={(host) => sent.push(host)} />);
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Speichern' }));

    await vi.waitFor(() => expect(sent).toEqual(['smtp.falsch']));
    expect(screen.queryByText(message)).not.toBeInTheDocument();
  });

  it('still stops a submit the field itself refuses', async () => {
    const user = userEvent.setup();
    const sent: string[] = [];
    render(
      <Form
        validationErrors={{ host: message }}
        onSubmit={(event) => {
          event.preventDefault();
          sent.push('sent');
        }}
      >
        <TextField name="host" label="Server" isRequired defaultValue="" />
        <AriaButton type="submit">Speichern</AriaButton>
      </Form>,
    );
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await user.click(screen.getByRole('button', { name: 'Speichern' }));

    expect(sent).toEqual([]);
    expect(screen.getByLabelText(/Server/)).toHaveAttribute('aria-invalid', 'true');
  });

  it('clears the message when the next answer carries none', async () => {
    const user = userEvent.setup();
    const { rerender } = render(<SmtpForm answer={{ host: message }} />);
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    rerender(<SmtpForm answer={null} />);
    rerender(<SmtpForm answer={{}} />);

    expect(screen.queryByText(message)).not.toBeInTheDocument();
  });

  it('does not send when the page only asks whether the form is valid', async () => {
    // `reportValidity()` fires `invalid` like a refused submit: the announcement page's
    // «Veröffentlichen» saved the draft instead and hid the reason it stayed unpublished.
    const user = userEvent.setup();
    const sent: string[] = [];
    render(<FixedForm errors={{ host: message }} sent={sent} />);
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Prüfen' }));
    await settle();

    expect(sent).toEqual([]);
    expect(screen.getByText(message)).toBeInTheDocument();
  });

  it('shows a message the page made itself again when the retry sent nothing', async () => {
    // The spotlight dialog says «Pflichtfeld» while no place is chosen; its submit handler sends
    // nothing then, so the message must not vanish on the second press.
    const user = userEvent.setup();
    const sent: string[] = [];
    render(<FixedForm errors={{ host: required }} sent={sent} />);
    expect(await screen.findByText(required)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await vi.waitFor(() => expect(sent).toEqual(['sent']));
    await settle();

    expect(screen.getByText(required)).toBeInTheDocument();
  });

  it('keeps the server messages while another field fails its own rule', async () => {
    const user = userEvent.setup();
    const sent: string[] = [];
    render(
      <FixedForm errors={{ host: message }} sent={sent}>
        <TextField name="port" label="Port" isRequired defaultValue="" />
      </FixedForm>,
    );
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await settle();

    expect(sent).toEqual([]);
    expect(screen.getByText(message)).toBeInTheDocument();
  });

  it('sends once for two quick presses', () => {
    vi.useFakeTimers();
    try {
      const sent: string[] = [];
      render(<FixedForm errors={{ host: message }} sent={sent} />);
      const save = screen.getByRole('button', { name: 'Speichern' });

      fireEvent.click(save);
      fireEvent.click(save);
      runTimers();

      expect(sent).toEqual(['sent']);
    } finally {
      vi.useRealTimers();
    }
  });

  it('sends nothing once the form is gone before its retry', () => {
    vi.useFakeTimers();
    try {
      const sent: string[] = [];
      const { unmount } = render(<FixedForm errors={{ host: message }} sent={sent} />);

      fireEvent.click(screen.getByRole('button', { name: 'Speichern' }));
      unmount();
      runTimers();

      expect(sent).toEqual([]);
    } finally {
      vi.useRealTimers();
    }
  });
});
