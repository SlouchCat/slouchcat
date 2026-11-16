import type {
  LibraryKind,
  MetadataProvider,
  MetadataProviderInfo,
  RootContent,
} from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useEffect, useRef, useState, type ReactNode, type RefObject } from 'react';
import { Button, Disclosure, DisclosurePanel, Heading } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AriaButton } from './AriaButton';
import { Chip } from './Chip';
import { Icon } from './Icon';
import { ProviderList, providerChain, providerListError } from './ProviderList';
import { RadioGroup } from './RadioGroup';
import { TextField } from './TextField';
import styles from './FolderList.module.css';

/** One folder of the list, as the forms hold it before it is sent. */
export interface FolderEntry {
  /** Stable key of the card (not its position: cards go in and out). */
  key: string;
  path: string;
  content: RootContent;
  providers: MetadataProvider[];
}

/** The contents a library of this kind may hold in its folders (concept 07, F-3). */
export function contentsOf(kind: LibraryKind): RootContent[] {
  if (kind === 'audiobooks') return ['audiobook'];
  if (kind === 'podcasts') return ['podcast'];
  return ['movie', 'series'];
}

/** The list a new folder of this content starts with: the order the server used before. */
export function defaultProviders(content: RootContent): MetadataProvider[] {
  if (content === 'series') return ['tmdb', 'tvdb'];
  if (content === 'audiobook' || content === 'podcast') return ['local'];
  return ['tmdb'];
}

let sequence = 0;

/** Focus targets after a removal (no card key starts with a space): the add button, or the
 * «Rückgängig» and the head of a card, by its key after the prefix. */
const ADD_BUTTON = ' add';
const UNDO = ' undo:';
const HEAD = ' head:';

/**
 * A fresh card. In a video library the second card offers the other content, so «Dokus» with a
 * film and a series folder is two clicks (concept 7).
 */
export function newFolderEntry(kind: LibraryKind, existing: readonly FolderEntry[]): FolderEntry {
  const contents = contentsOf(kind);
  const used = new Set(existing.map((entry) => entry.content));
  const content = contents.find((candidate) => !used.has(candidate)) ?? contents[0] ?? 'movie';
  sequence += 1;
  return { key: `folder-${sequence}`, path: '', content, providers: defaultProviders(content) };
}

/** Whether one path lies in the other (or is the same), on a path boundary. */
function overlaps(a: string, b: string): boolean {
  const x = a.replace(/\/+$/, '') || '/';
  const y = b.replace(/\/+$/, '') || '/';
  if (x === y) return true;
  const [short, long] = x.length < y.length ? [x, y] : [y, x];
  return short === '/' || long.startsWith(`${short}/`);
}

/**
 * What keeps the list from being sent: two cards whose paths overlap, or a provider list that
 * cannot identify (the server would answer 422 to both). A blank path is the field's own
 * business (it is required), so an untouched card does not block the button. The message of the
 * first problem; null when the list is fine.
 */
export function folderListError(
  t: TFunction,
  entries: readonly FolderEntry[],
  infos: readonly MetadataProviderInfo[] | undefined,
): string | null {
  for (const [index, entry] of entries.entries()) {
    const path = entry.path.trim();
    const earlier =
      path && entries.slice(0, index).find((other) => overlaps(other.path.trim(), path));
    if (earlier) return t('library.roots.overlap', { number: index + 1 });
    const providers = providerListError(t, entry.content, entry.providers, infos);
    if (providers) return t('library.roots.providers', { number: index + 1, message: providers });
  }
  return null;
}

/** Removing a stored folder only marks it; the form removes it when it is saved (concept 24 4.3). */
interface FolderMarking {
  /** Whether the card is a stored folder (marked) or a new one (taken out of the list at once). */
  isStored: (entry: FolderEntry) => boolean;
  marked: ReadonlySet<string>;
  onMark: (key: string) => void;
  onUndo: (key: string) => void;
}

interface FolderListProps {
  kind: LibraryKind;
  entries: readonly FolderEntry[];
  onChange: (entries: FolderEntry[]) => void;
  /** What `listMetadataProviders` said; absent in the setup wizard (fixed lists then). */
  infos?: readonly MetadataProviderInfo[];
  /** How many cards the list keeps at least: one for video and audiobooks, none for podcasts. */
  min: number;
  /** Prefix of the input names: `roots` on the admin page, `library.roots` in the wizard. */
  namePrefix: string;
  /**
   * The open cards by key, when the form holds them (it opens a card for a field error or a
   * jump link). Without, the cards there at the first render start open (a new library's one
   * card) and the list keeps the state.
   */
  expandedKeys?: ReadonlySet<string>;
  onExpandedChange?: (key: string, expanded: boolean) => void;
  /** Rendered right of the path field of a card: the browse button. */
  pathButton?: (entry: FolderEntry, inputRef: RefObject<HTMLInputElement | null>) => ReactNode;
  /** Rendered below the path field of a card: the facts of a stored folder. */
  pathAddon?: (entry: FolderEntry, inputRef: RefObject<HTMLInputElement | null>) => ReactNode;
  /**
   * Rendered at the end of a card, below its sources; `name` is the card's input prefix
   * (`roots[1]`), so the part can name its fields and find the server's messages.
   */
  cardAddon?: (entry: FolderEntry, name: string) => ReactNode;
  /** More of the closed line, after content and sources: languages, images, «geändert». */
  summary?: (entry: FolderEntry) => ReactNode;
  /** The path field of a card lost the focus. */
  onPathBlur?: (entry: FolderEntry) => void;
  /** The server's field messages by input name (`roots[1].providers`), shown at the field. */
  fieldErrors?: Readonly<Record<string, string>>;
  /**
   * How each card orders its providers: arrow buttons (the default) or a position per entry
   * (`rank`, concept 12 S-16, the administration's library form).
   */
  ordering?: 'arrows' | 'rank';
  /** Stored folders are marked for removal instead of leaving the list (the form «Bearbeiten»). */
  marking?: FolderMarking;
}

/**
 * «Ordnerliste» (ADR 0015, F-13; concept 24 4.2): the folders of a library as numbered cards that
 * fold. Closed, a card is one line: path, «Inhalt» (films or series in a video library; fixed
 * otherwise), the source chain and what `summary` adds. Open, it holds the path with what
 * `pathButton` and `pathAddon` add, the content, «Metadatenquellen» and `cardAddon`, and at its end
 * the way out. «+ Weiteren Ordner hinzufügen» below adds an open card. One submit of the
 * surrounding form saves them all, the server all of them or none. The fields of a closed card
 * stay mounted, so nothing typed is lost. The new card takes the focus in its path field;
 * removing one hands it to the add button, marking one to its «Rückgängig», undoing to the card's
 * head, so the keyboard never loses its place (rule 8).
 */
export function FolderList({
  kind,
  entries,
  onChange,
  infos,
  min,
  namePrefix,
  expandedKeys,
  onExpandedChange,
  pathButton,
  pathAddon,
  cardAddon,
  summary,
  onPathBlur,
  fieldErrors,
  ordering,
  marking,
}: FolderListProps) {
  const { t } = useTranslation();
  const contents = contentsOf(kind);
  const inputs = useRef(new Map<string, HTMLInputElement>());
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const addButton = useRef<HTMLButtonElement>(null);
  const [ownExpanded, setOwnExpanded] = useState<ReadonlySet<string>>(
    () => new Set(entries.map((entry) => entry.key)),
  );
  const expanded = expandedKeys ?? ownExpanded;
  const setExpanded = (key: string, open: boolean) => {
    setOwnExpanded((current) => {
      const next = new Set(current);
      if (open) next.add(key);
      else next.delete(key);
      return next;
    });
    onExpandedChange?.(key, open);
  };
  // The card key whose path field takes the focus after the next render, or a button.
  const focusNext = useRef<string | null>(null);
  const marked = marking?.marked;
  useEffect(() => {
    const target = focusNext.current;
    if (!target) return;
    focusNext.current = null;
    if (target === ADD_BUTTON) addButton.current?.focus();
    else if (target.startsWith(' ')) buttons.current.get(target)?.focus();
    else inputs.current.get(target)?.focus();
  }, [entries, marked]);
  const buttonRef = (id: string) => (node: HTMLButtonElement | null) => {
    if (node) buttons.current.set(id, node);
    else buttons.current.delete(id);
  };

  const update = (key: string, patch: Partial<FolderEntry>) =>
    onChange(entries.map((entry) => (entry.key === key ? { ...entry, ...patch } : entry)));
  const add = () => {
    const entry = newFolderEntry(kind, entries);
    focusNext.current = entry.key;
    // What was just added is open (concept 24 5, exception 2).
    setExpanded(entry.key, true);
    onChange([...entries, entry]);
  };
  const remove = (entry: FolderEntry) => {
    if (marking?.isStored(entry)) {
      focusNext.current = `${UNDO}${entry.key}`;
      setExpanded(entry.key, false);
      marking.onMark(entry.key);
      return;
    }
    focusNext.current = ADD_BUTTON;
    onChange(entries.filter((other) => other.key !== entry.key));
  };
  const kept = entries.filter((entry) => !marked?.has(entry.key)).length;

  return (
    <div className={styles.list}>
      {entries.length === 0 && (
        <p className={styles.empty}>
          {kind === 'podcasts' ? t('library.roots.emptyPodcasts') : t('library.roots.empty')}
        </p>
      )}
      <ol className={styles.cards}>
        {entries.map((entry, index) => {
          const number = index + 1;
          const name = `${namePrefix}[${index}]`;
          const title = entry.path.trim() || t('library.roots.number', { number });
          const badge = (
            <span className={styles.number} aria-hidden="true">
              {number}
            </span>
          );
          if (marked?.has(entry.key)) {
            return (
              <li
                key={entry.key}
                className={`${styles.card} ${styles.marked}`}
                aria-label={t('library.roots.number', { number })}
                data-folder-key={entry.key}
              >
                <div className={styles.head}>
                  {badge}
                  <span className={styles.summary}>
                    <span className={styles.cardTitle}>{title}</span>
                    <span className={styles.chain}>{t('library.roots.marked')}</span>
                  </span>
                  <AriaButton
                    ref={buttonRef(`${UNDO}${entry.key}`)}
                    variant="ghost"
                    icon="replay"
                    aria-label={t('library.roots.undoLabel', { number })}
                    onPress={() => {
                      focusNext.current = `${HEAD}${entry.key}`;
                      marking?.onUndo(entry.key);
                    }}
                  >
                    {t('library.roots.undo')}
                  </AriaButton>
                </div>
              </li>
            );
          }
          const inputRef: RefObject<HTMLInputElement | null> = {
            get current() {
              return inputs.current.get(entry.key) ?? null;
            },
            set current(node) {
              if (node) inputs.current.set(entry.key, node);
              else inputs.current.delete(entry.key);
            },
          };
          const stored = marking?.isStored(entry) ?? false;
          return (
            <li
              key={entry.key}
              className={styles.card}
              aria-label={t('library.roots.number', { number })}
              data-folder-key={entry.key}
            >
              <Disclosure
                className={styles.disclosure}
                isExpanded={expanded.has(entry.key)}
                onExpandedChange={(open) => setExpanded(entry.key, open)}
              >
                <div className={styles.head}>
                  <Heading level={3} className={styles.heading}>
                    <Button
                      slot="trigger"
                      ref={buttonRef(`${HEAD}${entry.key}`)}
                      className={styles.trigger}
                    >
                      <span className={styles.chevron}>
                        <Icon name="chevronRight" size="sm" />
                      </span>
                      {badge}
                      <span className={`${styles.cardTitle} ${styles.cardPath}`}>{title}</span>
                    </Button>
                  </Heading>
                  <span className={styles.summary}>
                    <Chip tone="info">{t(`library.content.${entry.content}`)}</Chip>
                    <span className={styles.chain}>{providerChain(t, entry.providers)}</span>
                    {summary?.(entry)}
                  </span>
                </div>
                <DisclosurePanel className={styles.fields}>
                  <div className={styles.pathRow}>
                    <TextField
                      name={`${name}.path`}
                      label={t('setup.library.path')}
                      placeholder={t('setup.library.pathPlaceholder')}
                      autoComplete="off"
                      spellCheck="false"
                      mono
                      isRequired
                      inputRef={inputRef}
                      value={entry.path}
                      onChange={(path) => update(entry.key, { path })}
                      onBlur={onPathBlur ? () => onPathBlur(entry) : undefined}
                      validate={(value) => {
                        if (!value.trim()) return t('errors.field.required');
                        return value.trim().startsWith('/') ? null : t('errors.field.not_absolute');
                      }}
                    />
                    {pathButton && (
                      <div className={styles.pathButton}>{pathButton(entry, inputRef)}</div>
                    )}
                  </div>
                  {pathAddon?.(entry, inputRef)}
                  {contents.length > 1 && (
                    <RadioGroup
                      name={`${name}.content`}
                      label={t('library.content.title')}
                      description={t('library.content.hint')}
                      value={entry.content}
                      onChange={(value) => {
                        const content = contents.find((candidate) => candidate === value);
                        if (content && content !== entry.content) {
                          update(entry.key, { content, providers: defaultProviders(content) });
                        }
                      }}
                      options={contents.map((value) => ({
                        value,
                        label: t(`library.content.${value}`),
                      }))}
                    />
                  )}
                  <ProviderList
                    name={`${name}.providers`}
                    content={entry.content}
                    value={entry.providers}
                    onChange={(providers) => update(entry.key, { providers })}
                    infos={infos}
                    ordering={ordering}
                    error={fieldErrors?.[`${name}.providers`] ?? fieldErrors?.[`${name}.content`]}
                  />
                  {cardAddon?.(entry, name)}
                  {(stored || kept > min) && (
                    <div className={styles.remove}>
                      <AriaButton
                        variant="dangerQuiet"
                        icon="trash"
                        aria-label={t('library.roots.removeLabel', { number })}
                        onPress={() => remove(entry)}
                      >
                        {stored ? t('library.roots.removeStored') : t('library.roots.remove')}
                      </AriaButton>
                    </div>
                  )}
                </DisclosurePanel>
              </Disclosure>
            </li>
          );
        })}
      </ol>
      <div className={styles.add}>
        <AriaButton ref={addButton} icon="plus" onPress={add}>
          {entries.length === 0 ? t('library.roots.addFirst') : t('library.roots.addMore')}
        </AriaButton>
      </div>
    </div>
  );
}
