import { useCallback, useEffect, useRef, useState, type Ref } from 'react';
import { Form as AriaForm, type FormProps } from 'react-aria-components';

/** The messages of one server answer, keyed by input name, as react-aria takes them. */
type ServerErrors = NonNullable<FormProps['validationErrors']>;

const none: ServerErrors = {};
const nothing: readonly string[] = [];

/** What the page says about the fields right now, and how many answers came before it. */
interface Answer {
  key: string;
  round: number;
}

/**
 * The current answer. `round` counts up with every change of its content, so a correction made
 * under the last answer cannot hide this one — not even when the server repeats itself word for
 * word, because the empty answer while the request runs counts as a round of its own.
 */
function useAnswer(given: ServerErrors): Answer {
  const key = JSON.stringify(given);
  const [answer, setAnswer] = useState<Answer>({ key, round: 0 });
  if (answer.key === key) return answer;
  const next = { key, round: answer.round + 1 };
  setAnswer(next);
  return next;
}

/**
 * Hands react-aria the same object for as long as its content is the same.
 *
 * react-aria shows a server error again whenever the identity of `validationErrors` changes, and
 * `formErrors(t, mutation.error)` builds a fresh object on every render: the message came back
 * right after the field had dropped it, so no correction ever made it disappear.
 */
function useHeld(errors: ServerErrors): ServerErrors {
  const key = JSON.stringify(errors);
  const [held, setHeld] = useState({ key, errors });
  if (held.key === key) return held.errors;
  setHeld({ key, errors });
  return errors;
}

/**
 * The form of the app: react-aria's `Form`, but a server's message for one field goes as soon as
 * the reader changes that field's value, and comes back with the next answer.
 *
 * react-aria drops a server error only on the native `change` event, which a text input fires on
 * blur. Waiting for that blur, with a message that came back on every render, is what made valid
 * input look rejected (Marco's finding of 23 October 2026 on the mail settings).
 */
export function Form({
  validationErrors,
  ref,
  ...rest
}: FormProps & { ref?: Ref<HTMLFormElement> }) {
  const form = useRef<HTMLFormElement>(null);
  const given = validationErrors ?? none;
  const { round } = useAnswer(given);
  const [marked, setMarked] = useState({ round, fields: nothing });

  const attach = useCallback(
    (element: HTMLFormElement | null) => {
      form.current = element;
      if (typeof ref === 'function') ref(element);
      else if (ref) ref.current = element;
    },
    [ref],
  );

  useEffect(() => {
    const element = form.current;
    if (!element) return;
    // `input` covers typing, `change` the controls that report only when they are done (a native
    // select, a file input); both bubble to the form. A mark belongs to the round it was made in,
    // so one left over from the previous answer is ignored instead of hiding the new one.
    const changed = (event: Event) => {
      const { name } = (event.target ?? {}) as { name?: string };
      if (!name) return;
      setMarked((previous) => {
        const fields = previous.round === round ? previous.fields : nothing;
        return fields.includes(name) ? previous : { round, fields: [...fields, name] };
      });
    };
    element.addEventListener('input', changed);
    element.addEventListener('change', changed);
    return () => {
      element.removeEventListener('input', changed);
      element.removeEventListener('change', changed);
    };
  }, [round]);

  const corrected = marked.round === round ? marked.fields : nothing;
  const shown: ServerErrors = {};
  for (const [field, message] of Object.entries(given)) {
    if (!corrected.includes(field)) shown[field] = message;
  }
  const held = useHeld(shown);

  return <AriaForm {...rest} ref={attach} validationErrors={held} />;
}
