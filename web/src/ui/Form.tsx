import { useCallback, useEffect, useLayoutEffect, useRef, useState, type Ref } from 'react';
import { Form as AriaForm, type FormProps } from 'react-aria-components';

/** The messages of one server answer, keyed by input name, as react-aria takes them. */
type ServerErrors = NonNullable<FormProps['validationErrors']>;

const none: ServerErrors = {};
const nothing: readonly string[] = [];

/** The rules of a control itself; `customError` is where react-aria puts the server's message. */
const ownRules = [
  'valueMissing',
  'typeMismatch',
  'patternMismatch',
  'tooLong',
  'tooShort',
  'rangeUnderflow',
  'rangeOverflow',
  'stepMismatch',
  'badInput',
] as const;

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
  const { key, round } = useAnswer(given);
  const [marked, setMarked] = useState({ round, fields: nothing });
  // What the retry below compares with once its submit is done: did an answer come meanwhile?
  const latest = useRef({ round, marked });
  useLayoutEffect(() => {
    latest.current = { round, marked };
  });

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
    // `input` covers typing and every control the browsers fire it for; `change` is the belt and
    // braces for one that reports only when it is done, and it is what react-aria itself listens
    // to. Both bubble to the form. A mark belongs to the round it was made in, so one left over
    // from the previous answer is ignored instead of hiding the new one, and `seen` keeps it at
    // one mark per field per round.
    //
    // The mark waits for the next tick, and that is not a detail. This listener runs before React
    // dispatches its own handlers, so react-aria has not yet written the new value into the page's
    // state: a render started from here puts the old value back into the input and the character
    // is gone. The Playwright suite found exactly that -- the login page received "2e-owner@…"
    // for "e2e-owner@…" -- and no unit test saw it, because jsdom does not order the events the
    // way a browser does.
    const seen = new Set<string>();
    const timers = new Set<ReturnType<typeof setTimeout>>();
    const changed = (event: Event) => {
      const { name } = (event.target ?? {}) as { name?: string };
      if (!name || seen.has(name)) return;
      seen.add(name);
      const timer = setTimeout(() => {
        timers.delete(timer);
        setMarked((previous) => {
          const fields = previous.round === round ? previous.fields : nothing;
          return fields.includes(name) ? previous : { round, fields: [...fields, name] };
        });
      });
      timers.add(timer);
    };
    // react-aria writes a server message into the field's native validity, so the browser stops
    // the next submit by itself and nothing is sent: a reader who retries the same value (the name
    // another profile had until it was renamed) got no save, and neither did one who pressed Enter
    // before the mark above landed -- Chromium runs timers after queued input, so the mark can
    // wait until after the key. A submit the server's message alone stops is the reader trying
    // again: every message of that answer goes, and the form submits once more in a task of its
    // own (the browser ignores a submit started while it still handles the one it stopped), where
    // only the fields' own rules decide. Once per answer, so a message that stays cannot loop.
    //
    // Only a real press counts: `reportValidity()` and `checkValidity()` fire `invalid` too, and a
    // page that asks whether the form may be published must not save it instead. A press is a
    // click on a submit button (Enter in a field clicks it, too) or Enter in an input; the mark
    // lasts until the browser has finished that press. The decision waits for the end of the
    // press, because the browser fires `invalid` for every refused control in turn: one control
    // that its own rule refuses (or another message) keeps the whole form as it is, messages
    // included. When the retry changed nothing -- no request went out, so the answer is still the
    // same one, which is what a message the page made itself looks like -- the messages come
    // back. A submit that gets through while a retry waits cancels it, so a second quick press
    // sends one request, not two.
    const answer = JSON.parse(key) as ServerErrors;
    let pressed = false;
    let retried = false;
    let blocked = false;
    let waiting: ReturnType<typeof setTimeout> | undefined;
    const later = (run: () => void) => {
      const timer = setTimeout(() => {
        timers.delete(timer);
        if (waiting === timer) waiting = undefined;
        run();
      });
      timers.add(timer);
      return timer;
    };
    let before = latest.current.marked;
    // A request the submit started clears the answer in a task queued while the submit ran
    // (TanStack Query tells its readers with `setTimeout`), so this check, queued after it, sees
    // the new round and leaves the marks alone.
    const unchanged = () =>
      later(() => {
        if (latest.current.round === round) setMarked(before);
      });
    const press = (event: Event) => {
      const target = event.target as Element | null;
      const submits =
        event.type === 'click'
          ? target?.closest('[type="submit"]') != null
          : (event as KeyboardEvent).key === 'Enter' && target instanceof HTMLInputElement;
      if (!submits || pressed) return;
      pressed = true;
      later(() => {
        pressed = false;
      });
    };
    const refused = (event: Event) => {
      if (!pressed || retried) return;
      const input = event.target as Partial<HTMLInputElement> | null;
      const name = input?.name;
      const server = name && name in answer ? answer[name] : undefined;
      const message = Array.isArray(server) ? server.join(' ') : server;
      const own = ownRules.some((rule) => input?.validity?.[rule]);
      if (message === undefined || input?.validationMessage !== message || own) blocked = true;
      if (waiting !== undefined) return;
      waiting = later(() => {
        const stop = blocked;
        blocked = false;
        if (stop) return;
        retried = true;
        before = latest.current.marked;
        setMarked({ round, fields: Object.keys(answer) });
        waiting = later(() => {
          element.requestSubmit();
          unchanged();
        });
      });
    };
    const sent = () => {
      if (waiting === undefined) return;
      clearTimeout(waiting);
      timers.delete(waiting);
      waiting = undefined;
      if (retried) unchanged();
    };
    element.addEventListener('input', changed);
    element.addEventListener('change', changed);
    element.addEventListener('click', press, true);
    element.addEventListener('keydown', press, true);
    // `invalid` and `submit` do not bubble the way a listener here needs; the form sees both in
    // the capture phase, before the page's own handler.
    element.addEventListener('invalid', refused, true);
    element.addEventListener('submit', sent, true);
    return () => {
      element.removeEventListener('input', changed);
      element.removeEventListener('change', changed);
      element.removeEventListener('click', press, true);
      element.removeEventListener('keydown', press, true);
      element.removeEventListener('invalid', refused, true);
      element.removeEventListener('submit', sent, true);
      for (const timer of timers) clearTimeout(timer);
    };
  }, [key, round]);

  const corrected = marked.round === round ? marked.fields : nothing;
  const shown: ServerErrors = {};
  for (const [field, message] of Object.entries(given)) {
    if (!corrected.includes(field)) shown[field] = message;
  }
  const held = useHeld(shown);

  return <AriaForm {...rest} ref={attach} validationErrors={held} />;
}
