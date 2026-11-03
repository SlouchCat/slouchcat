import type { AdminItemHit } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Notice } from '@/ui';
import { AriaButton, TextField } from '@/ui/aria';
import { adminItemSearchQuery, MIN_SEARCH } from './queries';

type ItemKind = AdminItemHit['kind'];
import styles from './CurationParts.module.css';

// Parts shared by the curation pages (M7): the title search of the pick and collection dialogs
// and the datetime-local conversion of a pick's window.

/** Wait after the last key before the search asks the server. */
const SEARCH_DEBOUNCE_MS = 200;

/** "Film · 2024 · Serie X": what tells two hits of the same name apart. */
export function titleMeta(
  t: TFunction,
  title: { kind: ItemKind; year?: number; parentTitle?: string },
): string {
  return [t(`kind.${title.kind}`), title.year, title.parentTitle].filter(Boolean).join(' · ');
}

interface TitleSearchProps {
  label: string;
  hint: string;
  /** Shown when the search found nothing to offer. */
  emptyText: string;
  /** Name of the hit list. */
  resultsLabel: string;
  /** Text of the button on every hit and its spoken name ("<title> wählen"). */
  actionText: string;
  actionLabel: (hit: AdminItemHit) => string;
  onPick: (hit: AdminItemHit) => void;
  /** Ids that are not offered (titles already in the list). */
  exclude?: ReadonlySet<string>;
  /** Kinds that may be picked; all when absent. */
  kinds?: readonly ItemKind[];
  /** Focus the field when it appears (after the list it adds to has loaded). */
  autoFocus?: boolean;
}

/**
 * Search field with a list of hits from `GET /admin/items/search`, each with a button. Enter in
 * the field does not submit the surrounding form; Escape still closes the dialog.
 */
export function TitleSearch({
  label,
  hint,
  emptyText,
  resultsLabel,
  actionText,
  actionLabel,
  onPick,
  exclude,
  kinds,
  autoFocus,
}: TitleSearchProps) {
  const { t } = useTranslation();
  const [text, setText] = useState('');
  const [term, setTerm] = useState('');
  useEffect(() => {
    const timer = window.setTimeout(() => setTerm(text.trim()), SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [text]);
  const search = useQuery(adminItemSearchQuery(term));
  const input = useRef<HTMLInputElement>(null);
  const hits = (search.data ?? []).filter(
    (hit) => (!kinds || kinds.includes(hit.kind)) && !exclude?.has(hit.id),
  );
  return (
    <div className={styles.search}>
      <TextField
        type="search"
        label={label}
        description={hint}
        value={text}
        onChange={setText}
        inputRef={input}
        autoFocus={autoFocus}
        autoComplete="off"
        spellCheck="false"
        onKeyDown={(event) => {
          if (event.key === 'Enter') event.preventDefault();
          event.continuePropagation();
        }}
      />
      {search.isError && <Notice tone="danger">{problemMessage(t, search.error)}</Notice>}
      {term.length >= MIN_SEARCH && search.isSuccess && hits.length === 0 && (
        <p className={styles.muted}>{emptyText}</p>
      )}
      {hits.length > 0 && (
        <ul className={styles.list} aria-label={resultsLabel}>
          {hits.map((hit) => (
            <li key={hit.id} className={styles.entry}>
              <span className={styles.stack}>
                <span>{hit.title}</span>
                <span className={styles.muted}>{titleMeta(t, hit)}</span>
              </span>
              <AriaButton
                variant="secondary"
                aria-label={actionLabel(hit)}
                onPress={() => {
                  onPick(hit);
                  // The hit leaves the list with its button: back to the field for the next one.
                  input.current?.focus();
                }}
              >
                {actionText}
              </AriaButton>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

const pad = (value: number) => String(value).padStart(2, '0');

/** An ISO time as the value of a `datetime-local` input (local time, minutes), '' for none. */
export function toLocalInput(iso: string | undefined): string {
  if (!iso) return '';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`;
}

/** The value of a `datetime-local` input as an ISO time, undefined when empty or invalid. */
export function fromLocalInput(value: string): string | undefined {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}
