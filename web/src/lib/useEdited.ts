import { useCallback, useState } from 'react';

/**
 * Whether the user edited a form since the last submit (`submitted()`). The server's field errors
 * are about the values that were sent: a form shows them only until the first edit (typing, a
 * file, an option), so the browser's own validation does not keep blocking the next submit.
 * react-aria's Form takes no `onInput`: `onInput` goes on an element around the fields. It is a
 * React handler on purpose: a native listener would re-render in the middle of the input event,
 * before react-aria's own handler, and lose the typed character.
 */
export function useEdited() {
  const [edited, setEdited] = useState(false);
  const onInput = useCallback(() => setEdited(true), []);
  const submitted = useCallback(() => setEdited(false), []);
  return { edited, submitted, onInput };
}
