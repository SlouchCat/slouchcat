import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
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
function SmtpForm({ answer }: { answer: Record<string, string> | null }) {
  const [host, setHost] = useState('smtp.falsch');
  return (
    <Form
      validationErrors={answer === null ? undefined : { ...answer }}
      onSubmit={(event) => event.preventDefault()}
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

describe('Form', () => {
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

  it('clears the message when the next answer carries none', async () => {
    const user = userEvent.setup();
    const { rerender } = render(<SmtpForm answer={{ host: message }} />);
    expect(await screen.findByText(message)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    rerender(<SmtpForm answer={null} />);
    rerender(<SmtpForm answer={{}} />);

    expect(screen.queryByText(message)).not.toBeInTheDocument();
  });
});
